// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package typec

import (
	"context"
	//	"regexp"
	//	"strings"

	//	"chromiumos/tast/dut"
	//	"chromiumos/tast/errors"
	"chromiumos/tast/remote/servo"
	"chromiumos/tast/rpc"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:     Basic,
		Desc:     "Checks basic typec kernel driver functionality",
		Contacts: []string{"pmalani@chromium.org", "chromeos-power@google.com"},
		Attr:     []string{"group:mainline", "informational"},
		Vars:     []string{"servo"},
	})
}

// Basic does the following:
// - Disconnect the servo as Power Delivery (PD) device.
// - Reconfigure the servo as Pin C DP device.
// - Reconnect the servo
// - Verify that the kernel recognizes the connection and can parse PD identity data from the EC.
func Basic(ctx context.Context, s *testing.State) {
	d := s.DUT()
	if !d.Connected(ctx) {
		s.Fatal("Failed DUT connection check at the beginning")
	}

	// Connect to gRPC server.
	_, err := rpc.Dial(ctx, d, s.RPCHint(), "cros")
	if err != nil {
		s.Fatal("Failed to connect to the RPC service on the DUT: ", err)
	}

	// Configure servo to disconnect from DUT on close.
	pxy, err := servo.NewProxy(ctx, s.RequiredVar("servo"), d.KeyFile(), d.KeyDir())
	if err != nil {
		s.Fatal("Failed to connect to servo: ", err)
	}
	defer pxy.Close(ctx)

	s.Log("Prash 1")
	// Simulate a disconnect. The only way do to this with the current servo
	// methods API is to switch the servo off.
	/*
		if err := pxy.Servo().SetPowerState(ctx, servo.PowerStateOff); err != nil {
			s.Fatal("Failed to disconnect servo: ", err)
		}
	*/

	// TODO(pmalani); Confirm that the DUT has disconnected.

	// Re-enable the servo.
	/*
		if err := pxy.Servo().SetPowerState(ctx, servo.PowerStateOn); err != nil {
			s.Fatal("Failed to disconnect servo: ", err)
		}
	*/

	// Re-connect to gRPC server.
	_, err = rpc.Dial(ctx, d, s.RPCHint(), "cros")
	if err != nil {
		s.Fatal("Failed to connect to the RPC service on the DUT: ", err)
	}

	if !d.Connected(ctx) {
		s.Fatal("Failed to connect to DUT")
	}

	// Check that the partner device is found.

	// Verify that the PD identity info is non-zero.
}

/*
// checkForPartner looks through /sys/class/typec and returns nil if it finds any port partner.
func checkForPartner(ctx context.Context, d *dut.DUT, expected bool) error {
	// Check that the servo device is listed as a partner. We don't have anything else
	// connected so we basically check if there is a partner.
	out, err := d.Command("ls", "/sys/class/typec").Output(ctx)
	if err != nil {
		return errors.Wrap(err, "could not run ls command on DUT")
	}

	found := false
	for _, device := range strings.Split(string(out), "\n") {
		if matched, err := regexp.MatchString(`port\d-partner`, device); err != nil {
			return errors.Wrap(err, "rrror running regex")
		} else if matched {
			found = true
			break
		}
	}

	if !found && expected {
		return errors.New("no Type C partner found")
	} else if found && !expected {
		return errors.New("type C partner found when none expected")
	}

	return nil
}
*/
