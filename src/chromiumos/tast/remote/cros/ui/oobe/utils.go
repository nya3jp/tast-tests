// Copyright 2022 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package oobeutil

import (
	"chromiumos/tast/services/cros/ui"
)

// SearchingForKeyboardNodeName is the node name in OOBE HID detection page,
// for an element indicating DUT is looking for a keyboard device to connect to.
const SearchingForKeyboardNodeName = "Searching for keyboard"

// FoundUSBKeyboardNodeName is the node name in OOBE HID detection page,
// for an element indicating a USB keyboard device is connected to DUT.
const FoundUSBKeyboardNodeName = "USB keyboard connected"

// ContinueButtonEnabledNodeName is the node name in OOBE HID detection page,
// for an element indicating contunue button is enabled.
const ContinueButtonEnabledNodeName = "Continue button enabled"

// ContinueButtonFinder is the continue button in OOBE HID detection screen.
var ContinueButtonFinder = &ui.Finder{
	NodeWiths: []*ui.NodeWith{
		{Value: &ui.NodeWith_Name{Name: "Continue"}},
		{Value: &ui.NodeWith_Role{Role: ui.Role_ROLE_BUTTON}},
	},
}
