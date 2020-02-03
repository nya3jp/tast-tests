// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package arc

import (
	"fmt"
	"os"
	"os/exec"
	"time"

	"github.com/golang/protobuf/ptypes/empty"
	"golang.org/x/net/context"
	"google.golang.org/grpc"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/bundles/cros/arc/optin"
	"chromiumos/tast/local/chrome"
	arcpb "chromiumos/tast/services/cros/arc"
	"chromiumos/tast/testing"
)

const packName = "/var/lib/ureadahead/opt.google.containers.android.rootfs.root.pack"
const root = "/opt/google/containers/android/rootfs/root"

func init() {
	testing.AddService(&testing.Service{
		Register: func(srv *grpc.Server, s *testing.ServiceState) {
			arcpb.RegisterUreadaheadPackServiceServer(srv, &UreadaheadPackService{s: s})
		},
	})
}

// waitForPack waits for pack is created
func waitForPack(ctx context.Context) error {
	testing.ContextLogf(ctx, "Waiting for the pack %s", packName)
	if err := testing.Poll(ctx, func(ctx context.Context) error {
		if _, err := os.Stat(packName); err != nil {
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

// UreadaheadPackService implements tast.cros.arc.UreadaheadPackService.
type UreadaheadPackService struct {
	s *testing.ServiceState
}

func (c *UreadaheadPackService) Generate(ctx context.Context, request *arcpb.UreadaheadPackRequest) (*empty.Empty, error) {
	if _, err := os.Stat(packName); err == nil {
		if err := os.Remove(packName); err != nil {
			return nil, errors.Wrap(err, "failed to clean up existing pack")
		}
	} else if !os.IsNotExist(err) {
		return nil, errors.Wrap(err, "failed to check if pack exists")
	}

	testing.ContextLog(ctx, "Start ureadahead tracing")
	cmd := exec.Command("ureadahead", "--quiet", "--force-trace", fmt.Sprintf("--path-prefix=%s", root), root)
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
		mode = "Initial boot"
		testing.ContextLog(ctx, "Waiting for ARC opt-in flow to complete")
		if err := optin.Perform(ctx, cr, tconn); err != nil {
			return nil, errors.Wrap(err, "failed to perform opt-in")
		}
	} else {
		// Wait Play Store app is in ready state that indicates boot is fully completed.
		mode = "Provisioned boot"
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

	if err := waitForPack(ctx); err != nil {
		return nil, err
	}

	return &empty.Empty{}, nil
}
