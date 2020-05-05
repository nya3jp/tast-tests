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

const (
	ds4HidRecording = "ds4.hid"

	ds4MacAddressRequest       = 0x81
	ds4MotionSensorCalibration = 0x02
)

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
	d.SetUniq(dualshock.Uniq)
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
	testing.ContextLog(ctx, "calling get report ds4")
	return dualshock.GetReport(ctx, processRNumDS4, d, buf)
}

// processRNumDS4 returns the data that will be written in the get report
// reply depending on the rnum that was sent.
func processRNumDS4(d *uhid.Device, rnum uhid.RNumType) []byte {
	if rnum == ds4MacAddressRequest {
		// The only get report request that requires answering is the mac
		// address request.
		return []byte{0x81, 0x01, 0x23, 0x45, 0x67, 0x89, 0xAB}
	} else if rnum == ds4MotionSensorCalibration {
		// While this request doesn't require a *specific* answer, it does
		// require *an* answer. So, we return an empty array and shut down
		// communication.
		jstest.KernelCommunicationDone = true
	}
	return []byte{}
}
