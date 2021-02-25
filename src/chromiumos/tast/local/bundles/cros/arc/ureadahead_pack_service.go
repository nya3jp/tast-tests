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

// Generate generates ureadahead pack for requested Chrome login mode for VM or container.
func (c *UreadaheadPackService) Generate(ctx context.Context, request *arcpb.UreadaheadPackRequest) (*arcpb.UreadaheadPackResponse, error) {
	vmEnabled, err := arc.VMEnabled()
	if err != nil {
		return nil, errors.Wrap(err, "failed to check whether ARCVM is enabled")
	}

	// Stop UI to make sure we don't have any pending holds and race condition restarting Chrome.
	testing.ContextLog(ctx, "Stopping UI to release all possible locks")
	if err := upstart.StopJob(ctx, "ui"); err != nil {
		return nil, errors.Wrap(err, "failed to stop ui")
	}
	defer upstart.EnsureJobRunning(ctx, "ui")

	var packPath string
	if vmEnabled {
		packPath, err = generateVMPack(ctx, request)
		if err != nil {
			return nil, errors.Wrap(err, "failed to generate ureadahead pack for VM")
		}
	} else {
		packPath, err = generateContainerPack(ctx, request)
		if err != nil {
			return nil, errors.Wrap(err, "failed to generate ureadahead pack for container")
		}
	}

	response := arcpb.UreadaheadPackResponse{
		PackPath: packPath,
	}
	return &response, nil
}

// generateVMPack generates ureadahead initial pack for requested Chrome login mode on guest OS.
func generateVMPack(ctx context.Context, request *arcpb.UreadaheadPackRequest) (string, error) {
	const (
		ureadaheadDataDir = "/var/lib/ureadahead"

		arcvmPackName = "opt.google.vms.android.pack"

		ureadaheadStopTimeout     = 30 * time.Second
		ureadaheadStopInterval    = 1 * time.Second
		ureadaheadFileStatTimeout = 10 * time.Second
	)

	packPath := filepath.Join(ureadaheadDataDir, arcvmPackName)

	if err := os.Remove(packPath); err != nil && !os.IsNotExist(err) {
		return "", errors.Wrap(err, "failed to clean up existing pack")
	}

	testing.ContextLog(ctx, "Start VM ureadahead tracing")

	chromeArgs := append(arc.DisableSyncFlags(), "--arc-force-show-optin-ui", "--arcvm-ureadahead-mode=generate")

	opts := []chrome.Option{
		chrome.ARCSupported(),
		chrome.RestrictARCCPU(),
		chrome.GAIALoginPool(request.Creds),
		chrome.ExtraArgs(chromeArgs...)}

	cr, err := chrome.New(ctx, opts...)
	if err != nil {
		return "", errors.Wrap(err, "failed to connect to Chrome")
	}
	defer cr.Close(ctx)

	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		return "", errors.Wrap(err, "failed to create test API connection")
	}
	defer tconn.Close()

	// Opt in.
	testing.ContextLog(ctx, "Waiting for ARC opt-in flow to complete")
	if err := optin.Perform(ctx, cr, tconn); err != nil {
		return "", errors.Wrap(err, "failed to perform opt-in")
	}

	outdir, ok := testing.ContextOutDir(ctx)
	if !ok || outdir == "" {
		return "", errors.New("failed to get name of the output directory")
	}

	// Connect to ARCVM instance.
	a, err := arc.New(ctx, outdir)
	if err != nil {
		return "", errors.Wrap(err, "failed to connect ARCVM")
	}
	defer a.Close(ctx)

	// Confirm ureadahead_generate service has stopped.
	if err := testing.Poll(ctx, func(ctx context.Context) error {
		if value, err := a.GetProp(ctx, "init.svc.ureadahead_generate"); err != nil {
			return testing.PollBreak(err)
		} else if value != "stopped" {
			return errors.New("ureadahead is not yet stopped")
		}
		return nil
	}, &testing.PollOptions{
		Timeout:  ureadaheadStopTimeout,
		Interval: ureadaheadStopInterval,
	}); err != nil {
		return "", errors.Wrap(err, "failed to wait for ureadahead to stop")
	}

	// Verify ureadahead exited which is triggered by opt-in completion.
	if value, err := a.GetProp(ctx, "dev.arc.ureadahead.exit"); err != nil || value != "1" {
		return "", errors.Wrap(err, "failed to verify ureadahead to exited")
	}

	srcPath := filepath.Join(ureadaheadDataDir, "pack")
	if err := testing.Poll(ctx, func(ctx context.Context) error {
		_, err := a.FileSize(ctx, srcPath)
		return err
	}, &testing.PollOptions{Timeout: ureadaheadFileStatTimeout}); err != nil {
		return "", errors.Wrap(err, "failed to ensure pack file exists")
	}

	if err := a.PullFile(ctx, srcPath, packPath); err != nil {
		return "", errors.Wrapf(err, "failed to pull %s from ARCVM: ", srcPath)
	}

	return packPath, nil
}

// generateContainerPack generates ureadahead pack for requested Chrome login mode, initial or provisioned on host OS.
func generateContainerPack(ctx context.Context, request *arcpb.UreadaheadPackRequest) (string, error) {
	const (
		ureadaheadDataDir = "/var/lib/ureadahead"

		containerPackName = "opt.google.containers.android.rootfs.root.pack"
		containerRoot     = "/opt/google/containers/android/rootfs/root"

		sysOpenTrace = "/sys/kernel/debug/tracing/events/fs/do_sys_open"

		ureadaheadTimeout = 10 * time.Second
	)

	// Create arguments for running ureadahead.
	args := []string{
		"--quiet",
		"--force-trace",
		fmt.Sprintf("--path-prefix=%s", containerRoot),
		containerRoot,
	}
	packPath := filepath.Join(ureadaheadDataDir, containerPackName)

	out, err := testexec.CommandContext(ctx, "lsof", "+D", containerRoot).CombinedOutput()
	if err != nil {
		// In case nobody holds file, lsof returns 1.
		if exitError, ok := err.(*exec.ExitError); !ok || exitError.ExitCode() != 1 {
			return "", errors.Wrap(err, "failed to verify android root is not locked")
		}
	}
	outStr := string(out)
	if outStr != "" {
		return "", errors.Errorf("found locks for %q: %q", containerRoot, outStr)
	}

	if err := os.Remove(packPath); err != nil && !os.IsNotExist(err) {
		return "", errors.Wrap(err, "failed to clean up existing pack")
	}

	if err := ioutil.WriteFile("/proc/sys/vm/drop_caches", []byte("3"), 0200); err != nil {
		return "", errors.Wrap(err, "failed to clear caches")
	}

	testing.ContextLog(ctx, "Start ureadahead tracing")

	// Make sure ureadahead flips these flags to confirm it is started.
	flags := []string{"/sys/kernel/debug/tracing/tracing_on",
		filepath.Join(sysOpenTrace, "enable")}
	for _, flag := range flags {
		if err := ioutil.WriteFile(flag, []byte("0"), 0644); err != nil {
			return "", errors.Wrap(err, "failed to reset ureadahead flag")
		}
	}

	cmd := testexec.CommandContext(ctx, "ureadahead", args...)
	if err := cmd.Start(); err != nil {
		return "", errors.Wrap(err, "failed to start ureadahead tracing")
	}

	// Shorten the total context by 5 seconds to allow for cleanup.
	cleanCtx := ctx
	ctx, cancel := ctxutil.Shorten(ctx, 5*time.Second)
	defer cancel()

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
		return "", errors.Wrap(err, "failed to ensure ureadahead started")
	}

	defer func() {
		if err := stopUreadaheadTracing(cleanCtx, cmd); err != nil {
			testing.ContextLog(cleanCtx, "Failed to stop ureadahead tracing")
		}
	}()

	// Explicitly set filter for sys_open in order to significantly reduce the tracing traffic.
	sysOpenFilterPath := filepath.Join(sysOpenTrace, "filter")
	sysOpenFilterContent := fmt.Sprintf("filename ~ \"%s/*\"", containerRoot)
	if err := ioutil.WriteFile(sysOpenFilterPath, []byte(sysOpenFilterContent), 0644); err != nil {
		return "", errors.Wrap(err, "failed to set sys open filter")
	}
	// Try to reset filter on exit.
	defer func() {
		if err := ioutil.WriteFile(sysOpenFilterPath, []byte("0"), 0644); err != nil {
			testing.ContextLog(cleanCtx, "WARNING: Failed to reset sys open filter")
		}
	}()

	chromeArgs := append(arc.DisableSyncFlags(), "--arc-force-show-optin-ui")

	opts := []chrome.Option{
		chrome.ARCSupported(),
		chrome.RestrictARCCPU(),
		chrome.GAIALoginPool(request.Creds),
		chrome.ExtraArgs(chromeArgs...)}

	cr, err := chrome.New(ctx, opts...)
	if err != nil {
		return "", errors.Wrap(err, "failed to connect to Chrome")
	}
	defer cr.Close(cleanCtx)

	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		return "", errors.Wrap(err, "failed to create test API connection")
	}
	defer tconn.Close()

	// Opt in.
	testing.ContextLog(ctx, "Waiting for ARC opt-in flow to complete")
	if err := optin.Perform(ctx, cr, tconn); err != nil {
		return "", errors.Wrap(err, "failed to perform opt-in")
	}

	if err := stopUreadaheadTracing(ctx, cmd); err != nil {
		return "", err
	}

	if err := testing.Poll(ctx, func(ctx context.Context) error {
		_, err := os.Stat(packPath)
		return err
	}, &testing.PollOptions{Timeout: ureadaheadTimeout}); err != nil {
		return "", errors.Wrap(err, "failed to ensure pack file exists")
	}

	return packPath, nil
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
