// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package gamepad

import (
	"context"
	"time"

	"chromiumos/tast/local/bundles/cros/gamepad/jstest"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         SwitchPro,
		Desc:         "Checks that the switch pro mappings are what we expect",
		Contacts:     []string{"jtguitar@google.com", "chromeos-tango@google.com", "hcutts@chromium.org", "ricardoq@chromium.org"},
		SoftwareDeps: []string{"chrome"},
		Data:         []string{"switch_pro.hid", "replay.html"},
		Timeout:      2 * time.Minute,
	})
}

func SwitchPro(ctx context.Context, s *testing.State) {
	const switchProHidRecording = "switch_pro.hid"
	d, err := jstest.CreateDevice(ctx, s.DataPath(switchProHidRecording))
	if err != nil {
		s.Fatal("Failed to create DS3: ", err)
	}
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
	jstest.KernelCommunicationDone = true
	jstest.Gamepad(ctx, s, d, s.DataPath(switchProHidRecording), buttonMappings, expectedButtons)
}
