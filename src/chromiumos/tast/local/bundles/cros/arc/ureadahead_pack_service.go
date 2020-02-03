// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package arc

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"time"

	"golang.org/x/net/context"
	"google.golang.org/grpc"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/arc"
	"chromiumos/tast/local/arc/optin"
	"chromiumos/tast/local/chrome"
	arcpb "chromiumos/tast/services/cros/arc"
	"chromiumos/tast/testing"
)

const ureadaheadDataDir = "/var/lib/ureadahead"

const containerPackName = "opt.google.containers.android.rootfs.root.pack"
const containerRoot = "/opt/google/containers/android/rootfs/root"

const arcvmPackName = "opt.google.vms.android.pack"
const arcvmRoot = "/opt/google/vms/android"

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

// waitForPack waits for the pack is created.
func waitForPack(ctx context.Context, packPath string) error {
	testing.ContextLogf(ctx, "Waiting for the pack %s", packPath)
	if err := testing.Poll(ctx, func(ctx context.Context) error {
		if _, err := os.Stat(packPath); err != nil {
			if !os.IsNotExist(err) {
				return testing.PollBreak(err)
			}
			return errors.New("Pack is not yet ready")
		}
		return nil
	}, &testing.PollOptions{Timeout: 15 * time.Second}); err != nil {
		return errors.Wrap(err, "failed to wait for pack")
	}
	return nil
}

func (c *UreadaheadPackService) Generate(ctx context.Context, request *arcpb.UreadaheadPackRequest) (*arcpb.UreadaheadPackResponse, error) {
	// Create arguments for running ureadahead
	args := []string{
		"--quiet",
		"--force-trace",
	}

	vmEnabled, err := arc.VMEnabled()
	if err != nil {
		return nil, errors.Wrap(err, "failed to check ARCVM status")
	}

	var packPath string
	// Part of arguments differ in container and arcvm.
	if vmEnabled {
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

	cmd := exec.Command("ureadahead", args...)

	if err := cmd.Start(); err != nil {
		return nil, errors.Wrap(err, "failed to start ureadahead tracing")
	}

	opts := []chrome.Option{
		chrome.ARCSupported(), chrome.RestrictARCCPU(), chrome.GAIALogin(),
		chrome.Auth("crosureadahead@gmail.com", "tkHe*ZgDTc=vw56r", ""),
		chrome.ExtraArgs("--arc-force-show-optin-ui",
			"--arc-disable-app-sync",
			"--arc-disable-play-auto-install",
			"--arc-disable-locale-sync",
			"--arc-play-store-auto-update=off",
			"--arc-disable-ureadahead")}

	if !request.InitialBoot {
		opts = append(opts, chrome.KeepState())
	}

	cr, err := chrome.New(ctx, opts...)
	if err != nil {
		return nil, errors.Wrap(err, "failed to connect to Chrome")
	}
	defer cr.Close(ctx)

	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		return nil, errors.Wrap(err, "failed to create test API connection")
	}
	defer tconn.Close()

	var mode string

	startTime := time.Now()

	if request.InitialBoot {
		// Opt in.
		mode = "initial boot"
		testing.ContextLog(ctx, "Waiting for ARC opt-in flow to complete")
		if err := optin.Perform(ctx, cr, tconn); err != nil {
			return nil, errors.Wrap(err, "failed to perform opt-in")
		}
	} else {
		// Wait Play Store app is in ready state that indicates boot is fully completed.
		mode = "provisioned boot"
		if err := optin.WaitForPlayStoreReady(ctx, tconn); err != nil {
			return nil, err
		}
	}

	duration := time.Now().Sub(startTime)

	testing.ContextLogf(ctx, "Done %s in %s", mode, duration.String())

	testing.ContextLog(ctx, "Stop ureadahead tracing")
	if err := cmd.Process.Signal(os.Interrupt); err != nil {
		return nil, errors.Wrap(err, "failed to stop ureadahead tracing")
	}

	if err := waitForPack(ctx, packPath); err != nil {
		return nil, err
	}

	var response arcpb.UreadaheadPackResponse
	response.PackPath = packPath
	return &response, nil
}
