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
		SoftwareDeps: []string{"tpm2", "chrome"},
		ServiceDeps:  []string{"tast.cros.security.BootLockboxService"},
	})
}

// ModeReboot does the following:
// - Log in on a DUT
// - Validate TBT alt mode is working correctly.
// - Reboot the system.
// - Validate that we are *not* in TBT alt mode and that USB+DP mode is working correctly.
//
// This test requires the following H/W topology to run.
//
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
	cl, err := rpc.Dial(ctx, s.DUT(), s.RPCHint(), "cros")
	if err != nil {
		s.Fatal("Failed to connect to the RPC service on the DUT: ", err)
	}
	defer cl.Close(ctx)

	client := security.NewBootLockboxServiceClient(cl.Conn)

	// Login to Chrome.
	_, err = client.NewChromeLogin(ctx, &empty.Empty{})
	if err != nil {
		s.Fatal("Failed to start Chrome: ", err)
	}

	s.Log("Verify that a TBT device is enumerated")

	err = testing.Poll(ctx, func(ctx context.Context) error {
		return isTBTDevicePresent(ctx, d)
	}, &testing.PollOptions{Interval: 100 * time.Millisecond, Timeout: 10 * time.Second})
	if err != nil {
		s.Fatal("TBT enumeration after logged in failed: ", err)
	}

	// TODO(b/179338646): Reboot the system.

	// Wait for system to come back online.

	// TODO(b/179338646): Check that there is no TBT device enumerated.

	// TODO(b/179338646): Check that there is a connected DP monitor.

}

// isTBTDevicePresent is a helper function which checks if a TBT device is connected to the DUT.
func isTBTDevicePresent(ctx context.Context, d *dut.DUT) error {
	out, err := d.Command("ls", "/sys/bus/thunderbolt/devices").Output(ctx)
	if err != nil {
		return errors.Wrap(err, "could not run ls command on dut")
	}

	for _, device := range strings.Split(string(out), "\n") {
		if device == "" {
			continue
		}

		if device != "domain0" && device != "0-0" {
			return nil
		}
	}

	return errors.New("no TBT device found")
}
