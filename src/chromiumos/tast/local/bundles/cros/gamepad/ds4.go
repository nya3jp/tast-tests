// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package gamepad contains test to check the correct functioning of
// some controller mappings.
package gamepad

import (
	"context"
	"time"

	"chromiumos/tast/local/bundles/cros/gamepad/dualshock"
	"chromiumos/tast/local/bundles/cros/gamepad/jstest"
	"chromiumos/tast/local/uhid"
	"chromiumos/tast/testing"
)

const ds4HidRecording = "ds4.hid"

func init() {
	testing.AddTest(&testing.Test{
		Func:         DS4,
		Desc:         "Checks that the DS4 mappings are what we expect",
		Contacts:     []string{"jtguitar@google.com", "chromeos-tango@google.com", "hcutts@chromium.org", "ricardoq@chromium.org"},
		SoftwareDeps: []string{"chrome"},
		Data:         []string{"ds4.hid", "replay.html"},
		Timeout:      5 * time.Minute,
	})
}

func DS4(ctx context.Context, s *testing.State) {
	d, err := jstest.CreateDevice(ctx, s.DataPath(ds4HidRecording))
	if err != nil {
		s.Fatal("Failed to create DS4: ", err)
	}
	s.Log("Created controller")
	d.SetUniq(dualshock.Uniq())
	d.EventHandlers[uhid.GetReport] = getReportDS4
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
		"options",
		"share",
		"PS",
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
		8: "share",
		9: "options",
		10: "L3",
		11: "R3",
		12: "top dpad",
		13: "bottom dpad",
		14: "left dpad",
		15: "right dpad",
		16: "PS",
	}`
	jstest.Gamepad(ctx, s, d, s.DataPath(ds4HidRecording), buttonMappings, expectedButtons)
}

// getReportDS4 handles getReport requests by the kernel.
func getReportDS4(ctx context.Context, d *uhid.Device, buf []byte) error {
	return dualshock.GetReport(processRNumDS4, d, buf)
}

// processRNumDS4 returns the data that will be written in the get report
// reply depending on the rnum that was sent.
func processRNumDS4(d *uhid.Device, rnum uint8) ([]byte, error) {
	if rnum == 0x05 {
		return []byte{0x05, 0x1e, 0x00, 0x05, 0x00, 0xe2, 0xff, 0xf2, 0x22, 0xbe, 0x22, 0x8d, 0x22, 0x4f, 0xdd, 0x4d, 0xdd, 0x39, 0xdd, 0x1c, 0x02, 0x1c, 0x02, 0xe3, 0x1f, 0x8b, 0xdf, 0x8c, 0x1e, 0xb4, 0xde, 0x30, 0x20, 0x71, 0xe0, 0x10, 0x00, 0xca, 0xfc, 0x64, 0x4d}, nil
	} else if rnum == 0xa3 {
		return []byte{0xa3, 0x41, 0x70, 0x72, 0x20, 0x20, 0x38, 0x20, 0x32, 0x30, 0x31, 0x34, 0x00, 0x00, 0x00, 0x00, 0x00, 0x30, 0x39, 0x3a, 0x34, 0x36, 0x3a, 0x30, 0x36, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x01, 0x00, 0x43, 0x03, 0x00, 0x00, 0x00, 0x51, 0x00, 0x05, 0x00, 0x00, 0x80, 0x03, 0x00}, nil
	} else if rnum == 0x81 {
		// The only get report request that requires answering is 0x81.
		return rnumData81(d.Uniq())
	}
	return []byte{}, nil
}

// rnumData81 returns a byte array used to reply to a get report
// request with rnum = 0x81. The array consists of the colon-separated
// elements of uniq with other necessary information.
func rnumData81(uniq string) ([]byte, error) {
	report := []byte{0x81, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00}
	return dualshock.StoreUniqInReport(report, uniq, 1)
}
