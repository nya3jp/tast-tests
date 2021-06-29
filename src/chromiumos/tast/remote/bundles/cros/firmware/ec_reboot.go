// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package firmware

import (
	"context"
	"time"

	"chromiumos/tast/remote/firmware/pre"
	"chromiumos/tast/testing"
	"chromiumos/tast/testing/hwdep"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         ECReboot,
		Desc:         "Checks that device will reboot when EC gets the remote requests via UART",
		Contacts:     []string{"js@semihalf.com", "chromeos-firmware@google.com"},
		Attr:         []string{"group:firmware", "firmware_experimental"},
		Vars:         []string{"servo"},
		Pre:          pre.NormalMode(),
		HardwareDeps: hwdep.D(hwdep.ChromeEC()),
	})
}

func syncOnDUT(h *pre.Value, s *testing.State) {
	s.Log("Calling sync() before rebooting")
	h.DUT.Conn.Command("sync")
}

func ECReboot(ctx context.Context, s *testing.State) {
	var (
		oldBootID  string
		newBootID  string
		powerState string
		err        error
	)
	const (
		reconnectTimeout = 3 * time.Minute
	)

	h := s.PreValue().(*pre.Value).Helper

	// Reboot via EC normally
	if oldBootID, err = h.Reporter.BootID(ctx); err != nil {
		s.Fatal("Failed to fetch current boot ID: ", err)
	}

	syncOnDUT(h.Helper, s)
	s.Log("Rebooting using EC")
	if err := h.Servo.RunECCommand(ctx, "reboot"); err != nil {
		s.Fatal("Failed to reboot: ", err)
	}
	s.Log("Reestablishing connection to DUT")
	if err := testing.Poll(ctx, func(ctx context.Context) error {
		return h.DUT.WaitConnect(ctx)
	}, &testing.PollOptions{Timeout: reconnectTimeout}); err != nil {
		s.Fatal("Failed to reconnect to DUT after rebooting using EC", err)
	}

	if newBootID, err = h.Reporter.BootID(ctx); err != nil {
		s.Fatal("Failed to fetch current boot ID: ", err)
	}
	if newBootID == oldBootID {
		s.Fatal("Failed to reboot normally, old boot ID is the same as new boot ID")
	}

	// Hard reboot via EC
	if oldBootID, err = h.Reporter.BootID(ctx); err != nil {
		s.Fatal("Failed to fetch current boot ID: ", err)
	}

	syncOnDUT(h.Helper, s)
	s.Log("Hard rebooting using EC")
	if err := h.Servo.RunECCommand(ctx, "reboot hard"); err != nil {
		s.Fatal("Failed to hard reboot: ", err)
	}
	s.Log("Reestablishing connection to DUT")
	if err := testing.Poll(ctx, func(ctx context.Context) error {
		return h.DUT.WaitConnect(ctx)
	}, &testing.PollOptions{Timeout: reconnectTimeout}); err != nil {
		s.Fatal("Failed to reconnect to DUT after hard rebooting using EC", err)
	}

	if newBootID, err = h.Reporter.BootID(ctx); err != nil {
		s.Fatal("Failed to fetch current boot ID: ", err)
	}
	if newBootID == oldBootID {
		s.Fatal("Failed to reboot normally, old boot ID is the same as new boot ID")
	}

	// Reboot with ap-off via EC
	if oldBootID, err = h.Reporter.BootID(ctx); err != nil {
		s.Fatal("Failed to fetch current boot ID: ", err)
	}

	syncOnDUT(h.Helper, s)
	s.Log("Turning AP off using EC")
	if err := h.Servo.RunECCommand(ctx, "reboot ap-off"); err != nil {
		s.Fatal("Failed to put AP off: ", err)
	}

	if powerState, err = h.Servo.GetECSystemPowerState(ctx); err != nil {
		s.Fatal("Failed to get EC system power state: ", err)
	}
	if powerState != "G3" {
		s.Fatal("Failed to put AP off, power state is: ", powerState)
	}

	// "Rebooting" normally to make sure AP is on again
	s.Log("Rebooting again using EC to make AP on")
	if err := h.Servo.RunECCommand(ctx, "reboot"); err != nil {
		s.Fatal("Failed to reboot after making AP off: ", err)
	}
	s.Log("Reestablishing connection to DUT")
	if err := testing.Poll(ctx, func(ctx context.Context) error {
		return h.DUT.WaitConnect(ctx)
	}, &testing.PollOptions{Timeout: reconnectTimeout}); err != nil {
		s.Fatal("Failed to reconnect to DUT after rebooting using EC", err)
	}

}
