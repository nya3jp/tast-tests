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
	"fmt"
	"math/rand"
	"strconv"
	"strings"
	"time"

	"chromiumos/tast/local/bundles/cros/gamepad/jstest"
	"chromiumos/tast/local/uhid"
	"chromiumos/tast/testing"
)

const ds3HidRecording = "ds3.hid"

func init() {
	testing.AddTest(&testing.Test{
		Func:         DS3,
		Desc:         "Checks that the DS3 mappings are what we expect",
		Contacts:     []string{"jtguitar@google.com", "chromeos-tango@google.com", "hcutts@chromium.org", "ricardoq@chromium.org"},
		SoftwareDeps: []string{"chrome"},
		Data:         []string{"ds3.hid", "ds3Replay.html"},
		Timeout:      5 * time.Minute,
	})
}

func DS3(ctx context.Context, s *testing.State) {
	d, err := jstest.CreateDevice(ctx, s.DataPath(ds3HidRecording))
	if err != nil {
		s.Fatal("Failed to create DS3: ", err)
	}
	s.Log("Created controller")
	d.SetUniq(uniq())
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

// uniq returns a randomly generated uniq string for a dualshock3. The
// uniq must be composed of a string of 6 colon-separated unsigned 8
// bit hexadecimal integers for the dualshock 3 to properly function.
func uniq() string {
	var rands [6]string
	for i := 0; i < len(rands); i++ {
		rands[i] = fmt.Sprintf("%02x", rand.Intn(256))
	}
	return strings.Join(rands[:], ":")
}

// getReport handles getReport requests by the kernel.
func getReport(ctx context.Context, d *uhid.Device, buf []byte) error {
	reader := bytes.NewReader(buf)
	event := uhid.GetReportRequest{}
	if err := binary.Read(reader, binary.LittleEndian, &event); err != nil {
		return err
	}
	data, err := processRNum(d, event.RNum)
	if err != nil {
		return err
	}
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
func processRNum(d *uhid.Device, rnum uint8) ([]byte, error) {
	if rnum == 0xf2 {
		return f2RNumData([]byte(d.Uniq()))
	} else if rnum == 0xf5 {

		// the creation of a dualshock 3 will entail 2 rnum=0xf2 get
		// report requests and one rnum=0xf5 after that. Therefore, the
		// dispatching ends once we receive the 0xf5 request.
		jstest.DeviceInfoSet = true
		return []byte{0x01, 0x00, 0x18, 0x5e, 0x0f, 0x71, 0xa4, 0xbb}, nil
	}
	return []byte{}, nil
}

// f2RNumData returns the data that will be written in the get report
// reply for rnum=0xf2.
func f2RNumData(uniq []byte) ([]byte, error) {
	// undocumented report in the HID report descriptor: the MAC address
	// of the device is stored in the bytes 4 to 9, the rest has been
	// dumped on a Sixaxis controller.
	data := []byte{0xf2, 0xff, 0xff, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x03, 0x40, 0x80, 0x18, 0x01, 0x8a}
	for i, v := range strings.Split(string(uniq), ":") {
		n, err := strconv.ParseUint(v[:2], 16, 8)
		if err != nil {
			return nil, err
		}
		data[i+4] = uint8(n)
	}
	return data, nil
}
