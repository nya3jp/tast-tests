// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package gamepad contains test to check the correct functioning of
// some controller mappings.
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

type rnum uint8

func init() {
	testing.AddTest(&testing.Test{
		Func:         DS3,
		Desc:         "Checks that the DS3 mappings are what we expect",
		Contacts:     []string{"jtguitar@google.com", "chromeos-tango@google.com", "hcutts@chromium.org", "ricardoq@chromium.org"},
		SoftwareDeps: []string{"chrome"},
		Data:         []string{"ds3.hid", "ds3_replay.html"},
		Timeout:      5 * time.Minute,
	})
}

func DS3(ctx context.Context, s *testing.State) {
	// ds3.hid was recorded using
	// https://gitlab.freedesktop.org/libevdev/hid-tools, a tool for
	// recording hid traffic. It's licensed under GPLv2+ license.
	const ds3HidRecording = "ds3.hid"
	d, err := jstest.CreateDevice(ctx, s.DataPath(ds3HidRecording))
	if err != nil {
		s.Fatal("Failed to create DS3: ", err)
	}
	s.Log("Created controller")
	const uniq = "01:23:45:67:89:AB"
	d.SetUniq(uniq)
	d.EventHandlers[uhid.GetReport] = getReport
	expectedButtons := []string{
		"triangle",
		"circle",
		"x",
		"square",
		"top dpad",
		"right dpad",
		"bottom dpad",
		"left dpad",
		"R1",
		"L1",
		"R3",
		"L3",
		"start",
		"select",
	}
	buttonMappings := `{
		0: "x",
		1: "circle",
		2: "square",
		3: "triangle",
		4: "L1",
		5: "R1",
		6: "L2",
		7: "R2",
		8: "select",
		9: "start",
		10: "L3",
		11: "R3",
		12: "top dpad",
		13: "bottom dpad",
		14: "left dpad",
		15: "right dpad",
		16: "PS",
	}`
	jstest.Gamepad(ctx, s, d, s.DataPath(ds3HidRecording), buttonMappings, expectedButtons)
}

// getReport handles getReport requests by the kernel.
func getReport(ctx context.Context, d *uhid.Device, buf []byte) error {
	reader := bytes.NewReader(buf)
	event := uhid.GetReportRequest{}
	if err := binary.Read(reader, binary.LittleEndian, &event); err != nil {
		return err
	}
	data := processRNum(d, event.RNum)
	reply := uhid.GetReportReplyRequest{
		RequestType: uhid.GetReportReply,
		ID:          event.ID,
		Err:         0,
		DataSize:    uint16(len(data)),
	}
	copy(reply.Data[:], data[:])
	return d.WriteEvent(reply)
}

// processRNum returns the data that will be written in the get report
// reply depending on the rnum that was sent.
func processRNum(d *uhid.Device, rnum uhid.RNumType) []byte {
	const macAddressRequest uhid.RNumType = 0xf2
	const operationalModeRequest uhid.RNumType = 0xf5
	if rnum == macAddressRequest {
		// This is a hardcoded array based on the uniq constant defined
		// above.
		return []byte{0xf2, 0xff, 0xff, 0x00, 0x01, 0x23, 0x45, 0x67, 0x89, 0xAB, 0x00, 0x03, 0x40, 0x80, 0x18, 0x01, 0x8a}
	} else if rnum == operationalModeRequest {

		// getReportRequests by the kernel are done after an operational
		// mode request. We end the communication here.
		jstest.DeviceInfoSet = true
		return []byte{0x01, 0x00, 0x18, 0x5e, 0x0f, 0x71, 0xa4, 0xbb}
	}
	return []byte{}
}
