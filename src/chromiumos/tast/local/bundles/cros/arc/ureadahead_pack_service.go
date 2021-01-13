// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package arc

import (
	"context"
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"google.golang.org/grpc"

	"chromiumos/tast/ctxutil"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/arc"
	"chromiumos/tast/local/arc/optin"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/testexec"
	"chromiumos/tast/local/upstart"
	arcpb "chromiumos/tast/services/cros/arc"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddService(&testing.Service{
		Register: func(srv *grpc.Server, s *testing.ServiceState) {
			arcpb.RegisterUreadaheadPackServiceServer(srv, &UreadaheadPackService{s: s})
		},
	})
}

// UreadaheadPackService implements tast.cros.arc.UreadaheadPackService.
type UreadaheadPackService struct {
	s *testing.ServiceState
}

// Generate generates ureadahead pack for requested Chrome login mode, initial or provisioned.
func (c *UreadaheadPackService) Generate(ctx context.Context, request *arcpb.UreadaheadPackRequest) (*arcpb.UreadaheadPackResponse, error) {
	const (
		ureadaheadDataDir = "/var/lib/ureadahead"

		containerPackName = "opt.google.containers.android.rootfs.root.pack"
		containerRoot     = "/opt/google/containers/android/rootfs/root"

		arcvmPackName = "opt.google.vms.android.pack"
		arcvmRoot     = "/opt/google/vms/android"

		sysOpenTrace = "/sys/kernel/debug/tracing/events/fs/do_sys_open"

		ureadaheadTimeout = 10 * time.Second
	)

	// Create arguments for running ureadahead.
	args := []string{
		"--quiet",
		"--force-trace",
	}

	// Stop UI to make sure we don't have any pending holds and race condition restarting Chrome.
	testing.ContextLog(ctx, "Stopping UI to release all possible locks")
	if err := upstart.StopJob(ctx, "ui"); err != nil {
		return nil, errors.Wrap(err, "failed to stop ui")
	}
	defer upstart.EnsureJobRunning(ctx, "ui")

	var packPath string
	var arcRoot string
	// Part of arguments differ in container and arcvm.
	if request.VmEnabled {
		packPath = filepath.Join(ureadaheadDataDir, arcvmPackName)
		args = append(args, fmt.Sprintf("--path-prefix-filter=%s", arcvmRoot))
		args = append(args, fmt.Sprintf("--pack-file=%s", packPath))
		arcRoot = arcvmRoot
	} else {
		packPath = filepath.Join(ureadaheadDataDir, containerPackName)
		args = append(args, fmt.Sprintf("--path-prefix=%s", containerRoot))
		arcRoot = containerRoot
	}
	args = append(args, arcRoot)

	out, err := testexec.CommandContext(ctx, "lsof", "+D", arcRoot).CombinedOutput()
	if err != nil {
		// In case nobody holds file, lsof returns 1.
		if exitError, ok := err.(*exec.ExitError); !ok || exitError.ExitCode() != 1 {
			return nil, errors.Wrap(err, "failed to verify android root is not locked")
		}
	}
	outStr := string(out)
	if outStr != "" {
		return nil, errors.Errorf("found locks for %q: %q", arcRoot, outStr)
	}

	if _, err := os.Stat(packPath); err == nil {
		if err := os.Remove(packPath); err != nil {
			return nil, errors.Wrap(err, "failed to clean up existing pack")
		}
	} else if !os.IsNotExist(err) {
		return nil, errors.Wrap(err, "failed to check if pack exists")
	}

	if err := ioutil.WriteFile("/proc/sys/vm/drop_caches", []byte("3"), 0200); err != nil {
		return nil, errors.Wrap(err, "failed to clear caches")
	}

	testing.ContextLog(ctx, "Start ureadahead tracing")

	// Make sure ureadahead flips these flags to confirm it is started.
	flags := []string{"/sys/kernel/debug/tracing/tracing_on",
		filepath.Join(sysOpenTrace, "enable")}
	for _, flag := range flags {
		if err := ioutil.WriteFile(flag, []byte("0"), 0644); err != nil {
			return nil, errors.Wrap(err, "failed to reset ureadahead flag")
		}
	}

	cmd := testexec.CommandContext(ctx, "ureadahead", args...)
	if err := cmd.Start(); err != nil {
		return nil, errors.Wrap(err, "failed to start ureadahead tracing")
	}

	// Wait ureadahead actually started.
	if err := testing.Poll(ctx, func(ctx context.Context) error {
		for _, flag := range flags {
			content, err := ioutil.ReadFile(flag)
			if err != nil {
				return testing.PollBreak(errors.Wrap(err, "failed to read flag"))
			}
			contentStr := strings.TrimSpace(string(content))
			if contentStr != "1" {
				return errors.Errorf("flag %q=%q is not yet flipped to 1", flag, contentStr)
			}

		}
		return nil
	}, &testing.PollOptions{Timeout: ureadaheadTimeout}); err != nil {
		return nil, errors.Wrap(err, "failed to ensure ureadahead started")
	}

	defer func() {
		if err := stopUreadaheadTracing(ctx, cmd); err != nil {
			testing.ContextLog(ctx, "Failed to stop ureadahead tracing")
		}
	}()

	// Explicitly set filter for sys_open in order to significantly reduce the tracing traffic.
	sysOpenFilterPath := filepath.Join(sysOpenTrace, "filter")
	sysOpenFilterContent := fmt.Sprintf("filename ~ \"%s/*\"", arcRoot)
	if err := ioutil.WriteFile(sysOpenFilterPath, []byte(sysOpenFilterContent), 0644); err != nil {
		return nil, errors.Wrap(err, "failed to set sys open filter")
	}
	// Try to reset filter on exit.
	defer func() {
		if err := ioutil.WriteFile(sysOpenFilterPath, []byte("0"), 0644); err != nil {
			testing.ContextLog(ctx, "WARNING: Failed to reset sys open filter")
		}
	}()

	chromeArgs := append(arc.DisableSyncFlags(), "--arc-force-show-optin-ui", "--arc-disable-ureadahead")

	opts := []chrome.Option{
		chrome.ARCSupported(), chrome.RestrictARCCPU(), chrome.GAIALogin(),
		chrome.Auth(request.Username, request.Password, ""),
		chrome.ExtraArgs(chromeArgs...)}

	if !request.InitialBoot {
		opts = append(opts, chrome.KeepState())
	}

	cr, err := chrome.New(ctx, opts...)
	if err != nil {
		return nil, errors.Wrap(err, "failed to connect to Chrome")
	}
	defer cr.Close(ctx)

	actualVMEnabled, err := arc.VMEnabled()
	if err != nil {
		return nil, errors.Wrap(err, "failed to check ARCVM status")
	}

	if actualVMEnabled != request.VmEnabled {
		return nil, errors.New("ARCVM enabled mismatch with actual ARC type")
	}

	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		return nil, errors.Wrap(err, "failed to create test API connection")
	}
	defer tconn.Close()

	// Shorten the total context by 5 seconds to allow for cleanup.
	shortCtx, cancel := ctxutil.Shorten(ctx, 5*time.Second)
	defer cancel()

	if request.InitialBoot {
		// Opt in.
		testing.ContextLog(shortCtx, "Waiting for ARC opt-in flow to complete")
		if err := optin.Perform(shortCtx, cr, tconn); err != nil {
			return nil, errors.Wrap(err, "failed to perform opt-in")
		}
	} else {
		testing.ContextLog(shortCtx, "Waiting for Play Store app to be ready")
		// Wait Play Store app is in ready state that indicates boot is fully completed.
		if err := optin.WaitForPlayStoreReady(shortCtx, tconn); err != nil {
			return nil, err
		}
	}

	if err := stopUreadaheadTracing(shortCtx, cmd); err != nil {
		return nil, err
	}

	if err := testing.Poll(ctx, func(ctx context.Context) error {
		_, err := os.Stat(packPath)
		return err
	}, &testing.PollOptions{Timeout: ureadaheadTimeout}); err != nil {
		return nil, errors.Wrap(err, "failed to ensure pack file exists")
	}

	response := arcpb.UreadaheadPackResponse{
		PackPath: packPath,
	}
	return &response, nil
}

// stopUreadaheadTracing stops ureadahead tracing by sending interrupt request and waits until it
// stops. If ureadahead tracing is already stopped this returns no error.
func stopUreadaheadTracing(ctx context.Context, cmd *testexec.Cmd) error {
	if cmd.ProcessState != nil {
		// Already stopped. Do nothing.
		return nil
	}

	testing.ContextLog(ctx, "Sending interrupt signal to ureadahead tracing process")
	if err := cmd.Process.Signal(os.Interrupt); err != nil {
		return errors.Wrap(err, "failed to send interrupt signal to ureadahead tracing")
	}

	if err := cmd.Wait(); err != nil {
		return errors.Wrap(err, "failed to wait ureadahead tracing done")
	}

	return nil
}
