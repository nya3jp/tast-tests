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

	"chromiumos/tast/common/testexec"
	"chromiumos/tast/ctxutil"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/arc"
	"chromiumos/tast/local/arc/optin"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/disk"
	"chromiumos/tast/local/upstart"
	arcpb "chromiumos/tast/services/cros/arc"
	"chromiumos/tast/testing"
)

// Defines custom error that indicate required flag file is not set to "1".
type flagIsNotSetError struct {
	reason string
}

func (e *flagIsNotSetError) Error() string {
	return e.reason
}

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
	const (
		ureadaheadDataDir = "/var/lib/ureadahead"

		containerPackName = "opt.google.containers.android.rootfs.root.pack"
		containerRoot     = "/opt/google/containers/android/rootfs/root"

		arcvmPackName = "opt.google.vms.android.pack"
		arcvmRoot     = "/opt/google/vms/android"

		sysOpenTrace = "/sys/kernel/debug/tracing/events/fs/do_sys_open"

		logName = "ureadahead.log"

		ureadaheadTimeout = 30 * time.Second
	)

	// Create arguments for running ureadahead.
	args := []string{
		"--verbose",
		"--force-trace",
	}

	// Stop UI to make sure we don't have any pending holds and race condition restarting Chrome.
	testing.ContextLog(ctx, "Stopping UI to release all possible locks")
	if err := upstart.StopJob(ctx, "ui"); err != nil {
		return nil, errors.Wrap(err, "failed to stop ui")
	}
	defer upstart.EnsureJobRunning(ctx, "ui")

	// Check for whether ARCVM is present.
	vmEnabled, err := arc.VMEnabled()
	if err != nil {
		return nil, errors.Wrap(err, "failed to check whether ARCVM is enabled")
	}

	var packPath string
	var arcRoot string
	// Part of arguments differ in container and arcvm.
	if vmEnabled {
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

	if err := os.Remove(packPath); err != nil && !os.IsNotExist(err) {
		return nil, errors.Wrap(err, "failed to clean up existing pack")
	}

	testing.ContextLog(ctx, "Login Chrome")

	chromeArgs := append(arc.DisableSyncFlags(), "--arc-force-show-optin-ui")
	if vmEnabled {
		chromeArgs = append(chromeArgs, "--arcvm-mount-debugfs", "--arcvm-ureadahead-mode=generate")
	}

	opts := []chrome.Option{
		chrome.ARCSupported(), // This does not start ARC automatically
		chrome.RestrictARCCPU(),
		chrome.GAIALoginPool(request.Creds),
		chrome.ExtraArgs(chromeArgs...)}

	cr, err := chrome.New(ctx, opts...)
	if err != nil {
		return nil, errors.Wrap(err, "failed to connect to Chrome")
	}

	// Shorten the total context by 5 seconds to allow for cleanup.
	cleanCtx := ctx
	ctx, cancel := ctxutil.Shorten(ctx, 5*time.Second)
	defer cancel()
	defer cr.Close(cleanCtx)

	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		return nil, errors.Wrap(err, "failed to create test API connection")
	}

	// Drop caches before starting ureadahead tracing.
	if err := disk.DropCaches(ctx); err != nil {
		return nil, errors.Wrap(err, "failed to drop caches")
	}

	testing.ContextLog(ctx, "Start ureadahead tracing")

	flags := []string{"/sys/kernel/debug/tracing/tracing_on",
		filepath.Join(sysOpenTrace, "enable")}

	// Define callback to handle flag.
	type flagHandler func(string) error

	// Helper that processes all tracked tracing flags.
	processFlags := func(fn flagHandler) error {
		for _, flag := range flags {
			if err := fn(flag); err != nil {
				return err
			}
		}
		return nil
	}

	// Make sure ureadahead flips these flags to confirm it is started.
	resetFlag := func(flag string) error {
		return ioutil.WriteFile(flag, []byte("0"), 0644)
	}

	if err := processFlags(resetFlag); err != nil {
		return nil, errors.Wrap(err, "failed to reset ureadahead flag")
	}

	if err := ioutil.WriteFile("/sys/kernel/debug/tracing/trace", []byte(""), 0644); err != nil {
		return nil, errors.Wrap(err, "failed to reset tracing buffer")
	}

	logPath := filepath.Join(ureadaheadDataDir, logName)
	log, err := os.Create(logPath)
	if err != nil {
		return nil, errors.Wrap(err, "failed to create log file")
	}
	defer log.Close()

	cmd := testexec.CommandContext(ctx, "ureadahead", args...)
	cmd.Stdout = log
	cmd.Stderr = log

	if err := cmd.Start(); err != nil {
		return nil, errors.Wrap(err, "failed to start ureadahead tracing")
	}

	// Make sure that content of the flag is set to "1".
	enusureFlagSet := func(flag string) error {
		content, err := ioutil.ReadFile(flag)
		if err != nil {
			return err
		}
		contentStr := strings.TrimSpace(string(content))
		// 1 means flag is enabled.
		if contentStr != "1" {
			return &flagIsNotSetError{
				reason: fmt.Sprintf("flag %q=%q is not set to 1", flag, contentStr),
			}
		}
		return nil
	}

	// Wait ureadahead actually started. All tracked flags must be flipped to "1".
	if err := testing.Poll(ctx, func(ctx context.Context) error {
		if err := processFlags(enusureFlagSet); err != nil {
			if _, ok := err.(*flagIsNotSetError); ok {
				return err
			}
			return testing.PollBreak(errors.Wrap(err, "failed to read flag"))
		}
		return nil
	}, &testing.PollOptions{Timeout: ureadaheadTimeout}); err != nil {
		return nil, errors.Wrap(err, "failed to ensure ureadahead started")
	}

	defer func() {
		if err := stopUreadaheadTracing(cleanCtx, cmd); err != nil {
			testing.ContextLog(cleanCtx, "Failed to stop ureadahead tracing")
		}
	}()

	if vmEnabled {
		// In ARCVM we trace system and vendor images. They are mounted as block devices
		// and normally they would not appear in tracing open requests.
		// Open images explicitly here in order to ensure tracing buffer has it.
		images := []string{"system.raw.img", "vendor.raw.img"}
		for _, image := range images {
			imagePath := filepath.Join(arcvmRoot, image)
			file, err := os.Open(imagePath)
			if err != nil {
				return nil, errors.Wrapf(err, "failed to touch image %q", imagePath)
			}
			file.Close()
		}
	}

	// Opt in.
	testing.ContextLog(ctx, "Waiting for ARC opt-in flow to complete")
	if err := optin.Perform(ctx, cr, tconn); err != nil {
		return nil, errors.Wrap(err, "failed to perform opt-in")
	}

	// Make sure tracing was not stopped in between. This verifies that all tracked flags
	// are still set to 1. If it not, that indicates that other component altered it while
	// ureadahead tracing session was running.
	if err := processFlags(enusureFlagSet); err != nil {
		return nil, errors.Wrap(err, "failed to ensure flag is set")
	}

	if err := stopUreadaheadTracing(ctx, cmd); err != nil {
		return nil, err
	}

	if err := testing.Poll(ctx, func(ctx context.Context) error {
		_, err := os.Stat(packPath)
		return err
	}, &testing.PollOptions{Timeout: ureadaheadTimeout}); err != nil {
		return nil, errors.Wrap(err, "failed to ensure pack file exists")
	}

	testing.ContextLog(ctx, "Ureadahead pack was generated")

	var vmPackPath string
	if vmEnabled {
		// Pull and obtain ARCVM pack from guest OS.
		vmPackPath, err = getGuestPack(ctx)
		if err != nil {
			return nil, errors.Wrap(err, "failed to obtain ureadahead pack from ARCVM guest OS")
		}
	}

	response := arcpb.UreadaheadPackResponse{
		PackPath:   packPath,
		VmPackPath: vmPackPath,
		LogPath:    logPath,
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

// getGuestPack pulls ureadahead initial pack for requested Chrome login mode from guest OS.
func getGuestPack(ctx context.Context) (string, error) {
	const (
		ureadaheadDataDir = "/var/lib/ureadahead"

		arcvmPackName = "arcvm.var.lib.ureadahead.pack"

		ureadaheadStopTimeout      = 50 * time.Second
		ureadaheadStopInterval     = 5 * time.Second
		ureadaheadFileStatTimeout  = 90 * time.Second
		ureadaheadFileStatInterval = 15 * time.Second
	)

	packPath := filepath.Join(ureadaheadDataDir, arcvmPackName)

	if err := os.Remove(packPath); err != nil && !os.IsNotExist(err) {
		return "", errors.Wrapf(err, "failed to clean up %s on the host", packPath)
	}

	outdir, ok := testing.ContextOutDir(ctx)
	if !ok {
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

	// Check for existence of newly generated pack file on guest side.
	srcPath := filepath.Join(ureadaheadDataDir, "pack")
	if err := testing.Poll(ctx, func(ctx context.Context) error {
		_, err := a.FileSize(ctx, srcPath)
		return err
	}, &testing.PollOptions{
		Timeout:  ureadaheadFileStatTimeout,
		Interval: ureadaheadFileStatInterval,
	}); err != nil {
		return "", errors.Wrap(err, "failed to ensure pack file exists")
	}

	if err := a.PullFile(ctx, srcPath, packPath); err != nil {
		return "", errors.Wrapf(err, "failed to pull %s from ARCVM", srcPath)
	}

	return packPath, nil
}
