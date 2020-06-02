// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package arc

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"google.golang.org/grpc"

	"chromiumos/tast/ctxutil"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/arc"
	"chromiumos/tast/local/arc/optin"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/testexec"
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
	)

	// Create arguments for running ureadahead.
	args := []string{
		"--quiet",
		"--force-trace",
	}

	var packPath string
	// Part of arguments differ in container and arcvm.
	if request.VmEnabled {
		packPath = filepath.Join(ureadaheadDataDir, arcvmPackName)
		args = append(args, fmt.Sprintf("--path-prefix-filter=%s", arcvmRoot))
		args = append(args, fmt.Sprintf("--pack-file=%s", packPath))
		args = append(args, arcvmRoot)
	} else {
		packPath = filepath.Join(ureadaheadDataDir, containerPackName)
		args = append(args, fmt.Sprintf("--path-prefix=%s", containerRoot))
		args = append(args, containerRoot)
	}

	if _, err := os.Stat(packPath); err == nil {
		if err := os.Remove(packPath); err != nil {
			return nil, errors.Wrap(err, "failed to clean up existing pack")
		}
	} else if !os.IsNotExist(err) {
		return nil, errors.Wrap(err, "failed to check if pack exists")
	}

	testing.ContextLog(ctx, "Start ureadahead tracing")

	cmd := testexec.CommandContext(ctx, "ureadahead", args...)

	if err := cmd.Start(); err != nil {
		return nil, errors.Wrap(err, "failed to start ureadahead tracing")
	}

	defer func() {
		if err := stopUreadaheadTracing(ctx, cmd); err != nil {
			testing.ContextLog(ctx, "Failed to stop ureadahead tracing")
		}
	}()

	chromeArgs := []string{
		"--arc-force-show-optin-ui",
		"--arc-disable-app-sync",
		"--arc-disable-play-auto-install",
		"--arc-disable-locale-sync",
		"--arc-play-store-auto-update=off",
		"--arc-disable-ureadahead"}

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

	if _, err := os.Stat(packPath); err != nil {
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
