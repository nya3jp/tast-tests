// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package typec

import (
	"context"
	"strings"
	"time"

	"github.com/golang/protobuf/ptypes/empty"

	"chromiumos/tast/dut"
	"chromiumos/tast/errors"
	"chromiumos/tast/rpc"
	"chromiumos/tast/services/cros/security"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         ModeReboot,
		Desc:         "Demonstrates USB Type C mode selection after reboot",
		Contacts:     []string{"pmalani@chromium.org"},
		SoftwareDeps: []string{"tpm2", "reboot", "chrome"},
		ServiceDeps:  []string{"tast.cros.security.BootLockboxService"},
	})
}

// ModeReboot does the following:
// - Log in on a DUT.
// - Validate TBT alt mode is working correctly.
// - Reboot the system.
// - Validate that we are *not* in TBT alt mode and that USB+DP mode is working correctly.
//
// This test requires the following H/W topology to run.
//
//        DUT ------> Thunderbolt3 (>= Titan Ridge) dock -----> DP monitor.
//      (USB4)
//
func ModeReboot(ctx context.Context, s *testing.State) {
	d := s.DUT()
	if !d.Connected(ctx) {
		s.Fatal("Not initially connected to DUT")
	}

	// Connect to gRPC server
	cl, err := rpc.Dial(ctx, d, s.RPCHint(), "cros")
	if err != nil {
		s.Fatal("Failed to connect to the RPC service on the DUT: ", err)
	}

	// Login to Chrome.
	client := security.NewBootLockboxServiceClient(cl.Conn)
	_, err = client.NewChromeLogin(ctx, &empty.Empty{})
	if err != nil {
		s.Fatal("Failed to start Chrome: ", err)
	}

	s.Log("Verifying that a TBT device is enumerated")
	err = testing.Poll(ctx, func(ctx context.Context) error {
		return checkTBTDevice(ctx, d, true)
	}, &testing.PollOptions{Interval: 100 * time.Millisecond, Timeout: 10 * time.Second})
	if err != nil {
		s.Fatal("TBT enumeration after logged in failed: ", err)
	}

	s.Log("Rebooting the DUT")
	if err := d.Reboot(ctx); err != nil {
		s.Fatal("Failed to reboot DUT: ", err)
	}

	s.Log("Reconnecting to the gRPC server")
	cl, err = rpc.Dial(ctx, d, s.RPCHint(), "cros")
	if err != nil {
		s.Fatal("Failed to connect to the RPC service on the DUT: ", err)
	}

	s.Log("Making sure we are still connected to the DUT")
	if !d.Connected(ctx) {
		s.Fatal("Couldn't remain connected to DUT post reboot")
	}

	s.Log("Verifying that no TBT device is enumerated")
	err = testing.Poll(ctx, func(ctx context.Context) error {
		return checkTBTDevice(ctx, d, false)
	}, &testing.PollOptions{Interval: 100 * time.Millisecond, Timeout: 10 * time.Second})
	if err != nil {
		s.Fatal("TBT non-enumeration after reboot failed: ", err)
	}

	// TODO(b/179338646): Check that there is a connected DP monitor.
}

// checkTBTDevice is a helper function which checks for TBT device connection to a DUT.
// |presence| specifies whether we want to check for the presence of a TBT device (true) or the
// absence of one (false).
func checkTBTDevice(ctx context.Context, d *dut.DUT, presence bool) error {
	out, err := d.Command("ls", "/sys/bus/thunderbolt/devices").Output(ctx)
	if err != nil {
		return errors.Wrap(err, "could not run ls command on DUT")
	}

	found := false
	for _, device := range strings.Split(string(out), "\n") {
		if device == "" {
			continue
		}

		if device != "domain0" && device != "0-0" {
			found = true
			break
		}
	}

	if presence {
		if found {
			return nil
		}

		return errors.New("no TBT device found")
	}

	// Handle the cases where we are checking for absence.
	if !found {
		return nil
	}

	return errors.New("TBT device found")
}
