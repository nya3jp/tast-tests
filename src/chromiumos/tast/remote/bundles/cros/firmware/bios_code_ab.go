// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package firmware

import (
	"context"
	"strings"
	"time"

	"chromiumos/tast/common/servo"
	"chromiumos/tast/errors"
	"chromiumos/tast/remote/firmware"
	"chromiumos/tast/testing"
	"chromiumos/tast/testing/hwdep"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         BIOSCodeAB,
		Desc:         "Verifies the AP can reach Port 80 code 0xab",
		Contacts:     []string{"jbettis@chromium.org", "cros-fw-engprod@google.com"},
		Attr:         []string{"group:firmware", "firmware_unstable", "firmware_bringup"},
		Vars:         []string{"servo"},
		HardwareDeps: hwdep.D(hwdep.X86()),
	})
}

// BIOSCodeAB reboots the EC, and looks for Port 80 code 0xab.
// On x86 platforms, depthcharge sends Port 80 code 0xab just before starting the kernel.
func BIOSCodeAB(ctx context.Context, s *testing.State) {
	servoSpec, _ := s.Var("servo")
	h := firmware.NewHelperWithoutDUT("", servoSpec, s.DUT().KeyFile(), s.DUT().KeyDir())
	defer h.Close(ctx)

	if err := h.RequireServo(ctx); err != nil {
		s.Fatal("Failed to require servo: ", err)
	}

	s.Log("Capturing EC log")
	if err := h.Servo.SetOnOff(ctx, servo.ECUARTCapture, servo.On); err != nil {
		s.Fatal("Failed to capture EC UART: ", err)
	}
	defer func() {
		if err := h.Servo.SetOnOff(ctx, servo.ECUARTCapture, servo.Off); err != nil {
			s.Fatal("Failed to disable capture EC UART: ", err)
		}
	}()
	// Read the uart stream just to make sure there isn't buffered data.
	if _, err := h.Servo.GetQuotedString(ctx, servo.ECUARTStream); err != nil {
		s.Fatal("Failed to read UART: ", err)
	}
	s.Log("Rebooting EC")
	if err := h.Servo.RunECCommand(ctx, "reboot"); err != nil {
		s.Fatal("Failed to send reboot command: ", err)
	}
	// Wait a little at the end of the test to make sure the EC finishes booting before the next test runs.
	defer func() {
		s.Log("Waiting for boot to finish")
		if err := testing.Sleep(ctx, 20*time.Second); err != nil {
			s.Fatal("Failed to sleep: ", err)
		}
	}()
	var leftoverLines string
	sawPort80 := false
	if err := testing.Poll(ctx, func(ctx context.Context) error {
		if lines, err := h.Servo.GetQuotedString(ctx, servo.ECUARTStream); err != nil {
			s.Fatal("Failed to read UART: ", err)
		} else if lines != "" {
			// It is possible to read partial lines, so save the part after newline for later
			lines = leftoverLines + lines
			if crlfIdx := strings.LastIndex(lines, "\r\n"); crlfIdx < 0 {
				leftoverLines = lines
				lines = ""
			} else {
				leftoverLines = lines[crlfIdx+2:]
				lines = lines[:crlfIdx+2]
			}

			for _, l := range strings.Split(lines, "\r\n") {
				if strings.Contains(l, "Port 80 writes:") {
					sawPort80 = true
				}
				if sawPort80 {
					s.Log("Output: ", l)
				}
				if sawPort80 && (strings.Contains(l, " ab ") || strings.HasSuffix(l, " ab")) {
					return nil
				}
			}
		}
		return errors.New("failed to find code 0xab")
	}, &testing.PollOptions{Interval: time.Millisecond * 200, Timeout: 60 * time.Second}); err != nil {
		s.Error("EC output parsing failed: ", err)
	}
}
