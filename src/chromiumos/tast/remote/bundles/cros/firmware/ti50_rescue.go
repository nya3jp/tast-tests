// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package firmware

import (
	"bytes"
	"context"
	"os"
	"strings"
	"time"

	"chromiumos/tast/common/servo"
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
	if err := h.Servo.RequireDebugHeader(ctx); err != nil {
		s.Fatal("RequireDebugHeader: ", err)
	}
	servoType, err := h.Servo.GetServoType(ctx)
	if err != nil {
		s.Fatal("GetServoType: ", err)
	}
	ti50ResetControl := "pch_disable"
	if strings.Contains(servoType, "c2d2") {
		ti50ResetControl = "cold_reset"
	}

	image := s.RequiredVar("image")

	if _, err := os.Stat(image); err != nil {
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

	s.Log("Starting rescue")
	cmd := testexec.CommandContext(ctx, "cr50-rescue", "--dauntless", "-v", "-i", image, "-d", uartdev)

	var buf bytes.Buffer
	cmd.Stdout = &buf
	cmd.Stderr = &buf

	err = cmd.Start()
	if err != nil {
		s.Fatal("Failed to start rescue: ", err)
	}
	testing.Sleep(ctx, 2*time.Second)

	err = h.Servo.SetString(ctx, servo.StringControl(ti50ResetControl), "on")
	if err != nil {
		s.Fatal("Servo error: ", err)
	}
	err = h.Servo.SetString(ctx, servo.StringControl(ti50ResetControl), "off")
	if err != nil {
		s.Fatal("Servo error: ", err)
	}

	go func() {
		cmd.Wait()
	}()

	len := 0
	for {
		testing.Sleep(ctx, 5*time.Second)
		if cmd.ProcessState != nil {
			break
		}
		if buf.Len() == len {
			s.Log("No progress from rescue")
			cmd.Kill()
		}
		len = buf.Len()
	}

	err = h.Servo.SetString(ctx, "cr50_ec3po_interp_connect", "on")
	if err != nil {
		s.Fatal("Servo error: ", err)
	}

	cmd.DumpLog(ctx)
	s.Log(strings.ToValidUTF8(buf.String(), ""))
	if !cmd.ProcessState.Success() {
		s.Fatal("Failed to complete rescue")
	}

}
