// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package firmware

import (
	"bytes"
	"context"
	"time"

	"chromiumos/tast/common/testexec"
	"chromiumos/tast/remote/firmware"
	"chromiumos/tast/testing"
	"chromiumos/tast/testing/hwdep"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         Ti50Rescue,
		Desc:         "Use UART rescue to flash Ti50 image",
		Contacts:     []string{"ecgh@chromium.org", "ti50-core@google.com"},
		Attr:         []string{"group:firmware"},
		Vars:         []string{"servo", "image"},
		HardwareDeps: hwdep.D(hwdep.ChromeEC()),
		Data:         []string{firmware.ConfigFile},
	})
}

func Ti50Rescue(ctx context.Context, s *testing.State) {
	servoSpec := s.RequiredVar("servo")
	h := firmware.NewHelper(s.DUT(), s.RPCHint(), s.DataPath(firmware.ConfigFile), servoSpec, "", "", "", "")
	defer h.Close(ctx)

	if err := h.RequireServo(ctx); err != nil {
		s.Fatal("Failed to connect to servo: ", err)
	}

	image := s.RequiredVar("image")

	cmd := testexec.CommandContext(ctx, "ls", image)
	if err := cmd.Run(); err != nil {
		s.Fatal("Image file not found: ", err)
	}

	uartdev, err := h.Servo.GetString(ctx, "raw_cr50_uart_pty")
	if err != nil {
		s.Fatal("Servo error: ", err)
	}

	err = h.Servo.SetString(ctx, "cr50_ec3po_interp_connect", "off")
	if err != nil {
		s.Fatal("Servo error: ", err)
	}

	cmd = testexec.CommandContext(ctx, "cr50-rescue", "--dauntless", "-v", "-i", image, "-d", uartdev)

	var buf bytes.Buffer
	cmd.Stdout = &buf
	cmd.Stderr = &buf

	err = cmd.Start()
	if err != nil {
		s.Fatal("Failed to start rescue: ", err)
	}
	testing.Sleep(ctx, 2*time.Second)
	s.Log(buf.String())

	err = h.Servo.SetString(ctx, "pch_disable", "on")
	if err != nil {
		s.Fatal("Servo error: ", err)
	}
	err = h.Servo.SetString(ctx, "pch_disable", "off")
	if err != nil {
		s.Fatal("Servo error: ", err)
	}

	testing.Sleep(ctx, 5*time.Second)
	s.Log(buf.String())

	err = cmd.Wait()
	if err != nil {
		s.Fatal("Failed to complete rescue: ", err)
	}
	cmd.DumpLog(ctx)

	err = h.Servo.SetString(ctx, "cr50_ec3po_interp_connect", "on")
	if err != nil {
		s.Fatal("Servo error: ", err)
	}
}
