// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package gamepad

import (
	"bytes"
	"context"
	"encoding/binary"
	"time"

	"chromiumos/tast/local/bundles/cros/gamepad/jstest"
	"chromiumos/tast/local/uhid"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         SwitchPro,
		Desc:         "Checks that the switch pro mappings are what we expect",
		Contacts:     []string{"jtguitar@google.com", "chromeos-tango@google.com", "hcutts@chromium.org", "ricardoq@chromium.org"},
		SoftwareDeps: []string{"chrome"},
		Data:         []string{"switch_pro.hid", "replay.html"},
		Timeout:      1 * time.Minute,
	})
}

func SwitchPro(ctx context.Context, s *testing.State) {
	const switchProHidRecording = "switch_pro.hid"
	d, err := jstest.CreateDevice(ctx, s.DataPath(switchProHidRecording))
	if err != nil {
		s.Fatal("Failed to create Switch Pro: ", err)
	}
	d.EventHandlers[uhid.Output] = handleOutput
	s.Log("Created controller")
	expectedButtons := []string{
		"x",
		"a",
		"b",
		"y",
		"top dpad",
		"right dpad",
		"bottom dpad",
		"left dpad",
		"r",
		"l",
		"zr",
		"zl",
		"right analog stick",
		"left analog stick",
		"home",
		"capture",
		"plus",
		"minus",
	}
	buttonMappings := `{
		0: "b",
		1: "a",
		2: "y",
		3: "x",
		4: "l",
		5: "r",
		6: "zl",
		7: "zr",
		8: "minus",
		9: "plus",
		10: "left analog stick",
		11: "right analog stick",
		12: "top dpad",
		13: "bottom dpad",
		14: "left dpad",
		15: "right dpad",
		16: "home",
		17: "capture"
	}`
	jstest.Gamepad(ctx, s, d, s.DataPath(switchProHidRecording), buttonMappings, expectedButtons)
}

func handleOutput(ctx context.Context, d *uhid.Device, buf []byte) error {

	type chromeOutputRequest int
	const (
		requestNumberIndex = 10

		setPlayerLights            chromeOutputRequest = 0x30
		enableIMU                                      = 0x40
		setIMUSensitivity                              = 0x41
		memoryRead                                     = 0x10
		readIMUCalibration                             = 0x20
		readHorizontalOffsets                          = 0x80
		readAnalogStickCalibration                     = 0x3D
		readAnalogStickParameters                      = 0x86
		enableVibration                                = 0x48
		setHomeLight                                   = 0x38
		setInputReportMode                             = 0x03
	)

	// The initialization sequence for the switch controller entails
	// many back and forths through output report requests that entail
	// replies in the form of input reports.
	replies := map[chromeOutputRequest][64]byte{
		setPlayerLights:            {0x21, 0x67, 0x91, 0x00, 0x80, 0x00, 0x6F, 0x27, 0x7D, 0xF7, 0xB7, 0x83, 0x00, 0x80, 0x30},
		enableIMU:                  {0x21, 0x6C, 0x91, 0x00, 0x80, 0x00, 0x71, 0x87, 0x7D, 0xF9, 0xB7, 0x83, 0x00, 0x80, 0x40},
		setIMUSensitivity:          {0x21, 0x75, 0x91, 0x00, 0x80, 0x00, 0x6F, 0x77, 0x7D, 0xF9, 0xB7, 0x83, 0x00, 0x80, 0x41},
		readIMUCalibration:         {0x21, 0x7B, 0x91, 0x00, 0x80, 0x00, 0x70, 0x87, 0x7D, 0xF8, 0x97, 0x83, 0x00, 0x90, 0x10, 0x20, 0x60, 0x00, 0x00, 0x18, 0xC8, 0xFF, 0x70, 0x00, 0xEA, 0x02, 0x00, 0x40, 0x00, 0x40, 0x00, 0x40, 0xF4, 0xFF, 0x0A, 0x00, 0x02, 0x00, 0xE7, 0x3B, 0xE7, 0x3B, 0xE7, 0x3B},
		readHorizontalOffsets:      {0x21, 0x81, 0x91, 0x00, 0x80, 0x00, 0x6E, 0xC7, 0x7D, 0xF8, 0xB7, 0x83, 0x00, 0x90, 0x10, 0x80, 0x60, 0x00, 0x00, 0x06, 0x50, 0xFD, 0x00, 0x00, 0xC6, 0x0F},
		readAnalogStickCalibration: {0x21, 0x87, 0x91, 0x00, 0x80, 0x00, 0x70, 0xA7, 0x7D, 0xF8, 0x97, 0x83, 0x00, 0x90, 0x10, 0x3D, 0x60, 0x00, 0x00, 0x12, 0x05, 0x36, 0x5E, 0x72, 0x67, 0x81, 0xE0, 0xA5, 0x58, 0xF6, 0x77, 0x83, 0x17, 0xC6, 0x5E, 0xCD, 0x25, 0x66},
		readAnalogStickParameters:  {0x21, 0x8F, 0x91, 0x00, 0x80, 0x00, 0x6E, 0xB7, 0x7D, 0xF9, 0xA7, 0x83, 0x00, 0x90, 0x10, 0x86, 0x60, 0x00, 0x00, 0x12, 0x0F, 0x30, 0x61, 0x96, 0x30, 0xF3, 0xD4, 0x14, 0x54, 0x41, 0x15, 0x54, 0xC7, 0x79, 0x9C, 0x33, 0x36, 0x63},
		enableVibration:            {0x21, 0x95, 0x91, 0x00, 0x80, 0x00, 0x6F, 0xD7, 0x7D, 0xF7, 0xC7, 0x83, 0x00, 0x80, 0x48},
		setHomeLight:               {0x21, 0x9E, 0x91, 0x00, 0x80, 0x00, 0x70, 0xB7, 0x7D, 0xF7, 0xA7, 0x83, 0x0C, 0x80, 0x38},
		setInputReportMode:         {0x21, 0xA7, 0x91, 0x00, 0x80, 0x00, 0x70, 0x47, 0x7D, 0xF8, 0xB7, 0x83, 0x09, 0x80, 0x03},
	}

	reader := bytes.NewReader(buf)
	event := uhid.OutputRequest{}
	if err := binary.Read(reader, binary.LittleEndian, &event); err != nil {
		return err
	}

	requestNumber := chromeOutputRequest(event.Data[requestNumberIndex])
	// If it's a request for a memory read then the next byte will
	// determine the reply.
	if requestNumber == memoryRead {
		requestNumber = chromeOutputRequest(event.Data[requestNumberIndex+1])
	} else if requestNumber == setInputReportMode {
		jstest.KernelCommunicationDone = true
	}
	reply := replies[chromeOutputRequest(requestNumber)]
	return d.InjectEvent([]uint8(reply[:]))
}
