// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package gamepad

import (
	"context"
	"time"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/bundles/cros/gamepad/dualshock"
	"chromiumos/tast/local/bundles/cros/gamepad/jstest"
	"chromiumos/tast/local/uhid"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         DS3,
		Desc:         "Checks that the DS3 mappings are what we expect",
		Contacts:     []string{"jtguitar@google.com", "chromeos-tango@google.com", "hcutts@chromium.org", "ricardoq@chromium.org"},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"chrome"},
		Data:         []string{"ds3.hid", "replay.html"},
		Timeout:      5 * time.Minute,
	})
}

func DS3(ctx context.Context, s *testing.State) {
	// ds3.hid was recorded using
	// https://gitlab.freedesktop.org/libevdev/hid-tools, a tool for
	// recording hid traffic.
	const ds3HidRecording = "ds3.hid"
	d, err := jstest.CreateDevice(ctx, s.DataPath(ds3HidRecording))
	if err != nil {
		s.Fatal("Failed to create DS3: ", err)
	}
	s.Log("Created controller")
	d.SetUniq(dualshock.Uniq)
	d.EventHandlers[uhid.GetReport] = handleGetReportDS3
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

// handleGetReportDS3 handles a get report request by the kernel for
// the dualshock 3 controller.
func handleGetReportDS3(ctx context.Context, d *uhid.Device, buf []byte) error {
	processRNum := func(rnum uhid.RNumType) ([]byte, error) {
		const (
			// macAddressRequest documentation can be found here:
			// https://source.chromium.org/chromiumos/chromiumos/codesearch/+/master:src/third_party/kernel/v4.4/drivers/hid/hid-sony.c;l=2360
			macAddressRequest uhid.RNumType = 0xf2
			// operationalModeRequest documentation can be found here:
			// https://source.chromium.org/chromiumos/chromiumos/codesearch/+/master:src/third_party/kernel/v4.4/drivers/hid/hid-sony.c;l=1413
			// this request is made to switch the controller to operational
			// mode. Only in this mode does the controller report events.
			operationalModeRequest = 0xf5
		)
		switch rnum {
		case macAddressRequest:
			// This is a hardcoded array based on the uniq constant defined
			// above.
			// undocumented report in the HID report descriptor:
			// the MAC address of the device is stored in the bytes 4-9
			// rest has been dumped on a Sixaxis controller
			return []byte{0xf2, 0xff, 0xff, 0x00, 0x01, 0x23, 0x45, 0x67, 0x89, 0xAB, 0x00, 0x03, 0x40, 0x80, 0x18, 0x01, 0x8a}, nil
		case operationalModeRequest:
			// getReportRequests by the kernel are done after an operational
			// mode request. We end the communication here.
			jstest.KernelCommunicationDone = true
			return []byte{0x01, 0x00, 0x18, 0x5e, 0x0f, 0x71, 0xa4, 0xbb}, nil
		default:
			return []byte{}, errors.Errorf("unsupported request type: 0x%02x", rnum)
		}
	}
	return dualshock.HandleGetReport(ctx, processRNum, d, buf)
}

// processRNumDS3 returns the data that will be written in the get report
// reply depending on the rnum that was sent.
func processRNumDS3(rnum uhid.RNumType) ([]byte, error) {
	const (
		// macAddressRequest documentation can be found here:
		// https://source.chromium.org/chromiumos/chromiumos/codesearch/+/master:src/third_party/kernel/v4.4/drivers/hid/hid-sony.c;l=2360
		macAddressRequest uhid.RNumType = 0xf2
		// operationalModeRequest documentation can be found here:
		// https://source.chromium.org/chromiumos/chromiumos/codesearch/+/master:src/third_party/kernel/v4.4/drivers/hid/hid-sony.c;l=1413
		// this request is made to switch the controller to operational
		// mode. Only in this mode does the controller report events.
		operationalModeRequest = 0xf5
	)
	switch rnum {
	case macAddressRequest:
		// This is a hardcoded array based on the uniq constant defined
		// in dualshock.Uniq.
		// Undocumented report in the HID report descriptor:
		// the MAC address of the device is stored in the bytes 4-9
		// rest has been dumped on a Sixaxis controller
		return []byte{0xf2, 0xff, 0xff, 0x00, 0x01, 0x23, 0x45, 0x67, 0x89, 0xAB, 0x00, 0x03, 0x40, 0x80, 0x18, 0x01, 0x8a}, nil
	case operationalModeRequest:
		// getReportRequests by the kernel are done after an operational
		// mode request. We end the communication here.
		jstest.KernelCommunicationDone = true
		return []byte{0x01, 0x00, 0x18, 0x5e, 0x0f, 0x71, 0xa4, 0xbb}, nil
	default:
		return []byte{}, errors.Errorf("unsupported request type: 0x%02x", rnum)
	}
}
