// Copyright 2021 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package firmware

import (
	"context"
	"time"

	"chromiumos/tast/common/firmware/ti50"
	"chromiumos/tast/common/servo"
	"chromiumos/tast/remote/firmware/ti50/fixture"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:    Ti50Demo,
		Desc:    "Demo ti50 in remote environment(Andreiboard connected to devboardsvc host)",
		Timeout: 30 * time.Second,
		Contacts: []string{
			"aluo@chromium.org",            // Test Author
			"chromeos-firmware@google.com", // CrOS Firmware Developers
		},
		Attr:    []string{"group:firmware"},
		Fixture: fixture.Ti50,
		Vars:    []string{"servo"},
	})
}

func Ti50Demo(ctx context.Context, s *testing.State) {
	f := s.FixtValue().(*fixture.Value)

	board, err := f.DevBoard(ctx, 4096, time.Second)
	if err != nil {
		s.Fatal("Could not get board: ", err)
	}

	err = board.Open(ctx)
	if err != nil {
		s.Fatal("Open console port: ", err)
	}
	// Wait a little for opentitantool to take over the console, this will test
	// that flashing still works after the console command.
	testing.Sleep(ctx, 5*time.Second)

	var svo *servo.Servo

	// Capture CCD output using the gsc_uart_capture servo control
	if servoHost, ok := s.Var("servo"); ok {
		svop, err := servo.NewProxy(ctx, servoHost, "", "")
		if err != nil {
			s.Fatal("Open servo: ", err)
		}
		defer svop.Close(ctx)

		svo = svop.Servo()

		if err = svo.SetString(ctx, "gsc_uart_capture", "on"); err != nil {
			s.Fatal("gsc_uart_capture=on Failed: ", err)
		}
		defer svo.SetString(ctx, "gsc_uart_capture", "off")
	} else {
		s.Log("Skipping CCD output collection via servo")
	}

	if err = ti50.Demo(ctx, board, ""); err != nil {
		s.Fatal("Ti50Demo Failed: ", err)
	}

	if svo != nil {
		ccdOutput, err := svo.GetString(ctx, "gsc_uart_stream")
		if err != nil {
			s.Fatal("Get gsc_uart_stream: ", err)
		}
		s.Log("CCD output via servo: ", ccdOutput)
	}
}
