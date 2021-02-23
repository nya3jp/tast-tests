// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package typec

import (
	"context"
	"regexp"
	"strings"

	"chromiumos/tast/dut"
	"chromiumos/tast/errors"
	"chromiumos/tast/remote/servo"
	"chromiumos/tast/testing"
)

// This is the regexp used to check whether the EC UART command  to check for PD disabled succeeded.
const pdDisabledRegexp = "[\"State\\: Attached\\.SRC\"]"

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

	pxy, err := servo.NewProxy(ctx, s.RequiredVar("servo"), d.KeyFile(), d.KeyDir())
	if err != nil {
		s.Fatal("Failed to connect to servo: ", err)
	}
	defer pxy.Close(ctx)

	if err := setServoPDState(ctx, pxy, servo.Off); err != nil {
		s.Fatal("Failed to disable PD on servo: ", err)
	}

	if err := checkPDDisconnected(ctx, pxy); err != nil {
		s.Fatal("Failed to verify PD disconnected: ", err)
	}

	// Configure Servod to be DP Pin C.
	if err := setServoDPMode(ctx, pxy); err != nil {
		s.Fatal("Failed to configure servo for DP: ", err)
	}

	// Re-enable PD on the device.
	if err := setServoPDState(ctx, pxy, servo.On); err != nil {
		s.Fatal("Failed to disable PD on servo: ", err)
	}

	s.Log("Connecting to DUT")
	if err := d.Connect(ctx); err != nil {
		s.Fatal("Failed to connect to DUT: ", err)
	} else if !d.Connected(ctx) {
		s.Fatal("Not connected after connecting")
	}

	// Check that the partner DP alternate mode is found.
	if err := checkForDPAltMode(ctx, d, s); err != nil {
		s.Fatal("Failed to find the expected partner: ", err)
	}

}

// checkPDDisconnected uses the EC to approximate whether the servo was disabled/disconnected from
// a PD perspective. This is determined to be the case of the "pd <port> state" EC console command
// returns a result with the DUT state as "Attached.SRC".
func checkPDDisconnected(ctx context.Context, pxy *servo.Proxy) error {
	if err := pxy.Servo().SetECCommandRegexp(ctx, pdDisabledRegexp); err != nil {
		return errors.Wrap(err, "failed to set EC command regexp")
	}
	defer pxy.Servo().SetECCommandRegexp(ctx, "None")

	if err := pxy.Servo().RunECCommand(ctx, "pd 0 state"); err != nil {
		return errors.Wrap(err, "failed to run pd state command on EC")
	}

	res, err := pxy.Servo().GetECCommand(ctx)
	if err != nil {
		return errors.Wrap(err, "failed to get the response of the EC command")
	}

	if matched, err := regexp.MatchString(`State\: Attached\.SRC`, res); err != nil {
		return errors.Wrap(err, "couldn't check EC Command response")
	} else if !matched {
		return errors.Errorf("EC Command response was unexpected: %s", res)
	}

	return nil
}

func setServoPDState(ctx context.Context, pxy *servo.Proxy, val servo.OnOffValue) error {
	var role servo.V4RoleValue = "snk"
	if val == servo.On {
		role = "src"
	}

	if err := pxy.Servo().SetV4Role(ctx, role); err != nil {
		return errors.Wrap(err, "failed to turn set Power role to snk")
	}

	if err := pxy.Servo().PDComm(ctx, val); err != nil {
		return errors.Wrap(err, "failed to turn off PD on servo")
	}

	return nil
}

func setServoDPMode(ctx context.Context, pxy *servo.Proxy) error {
	if err := pxy.Servo().RunServoCommand(ctx, "usbc_action dp disable"); err != nil {
		return errors.Wrap(err, "failed to disable DP support")
	}

	if err := pxy.Servo().RunServoCommand(ctx, "usbc_action dp pin c"); err != nil {
		return errors.Wrap(err, "failed to set DP pin assignment")
	}

	if err := pxy.Servo().RunServoCommand(ctx, "usbc_action dp mf 0"); err != nil {
		return errors.Wrap(err, "failed to set DP multi-function")
	}

	if err := pxy.Servo().RunServoCommand(ctx, "usbc_action dp enable"); err != nil {
		return errors.Wrap(err, "failed to enable DP support")
	}

	return nil
}

// checkForDPAltMode verifies that a partner was enumerated with the expected DP altmode.
func checkForDPAltMode(ctx context.Context, d *dut.DUT, s *testing.State) error {
	// Servo is always on port 0.
	out, err := d.Command("ls", "/sys/class/typec/port0-partner").Output(ctx)
	if err != nil {
		return errors.Wrap(err, "could not run ls command on DUT")
	}

	for _, device := range strings.Split(string(out), "\n") {
		s.Log("Device is: ", device)
	}

	return nil
}
