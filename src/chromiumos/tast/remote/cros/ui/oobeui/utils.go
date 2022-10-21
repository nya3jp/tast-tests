// Copyright 2022 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package oobeutil provides common functions used for OOBE UI tests.
package oobeutil

import (
	"chromiumos/tast/services/cros/ui"
)

// SearchingForKeyboardNodeName is the node name in OOBE HID detection page,
// for an element indicating DUT is looking for a keyboard device to connect to.
const SearchingForKeyboardNodeName = "Searching for keyboard"

// FoundKeyboardNodeName part of a node name in OOBE HID detection page,
// for an element indicating a keyboard device |KEYBD_REF| is paired or pairing to DUT.
const FoundKeyboardNodeName = "KEYBD_REF"

// ContinueButtonFinder is the continue button in OOBE HID detection screen.
var ContinueButtonFinder = &ui.Finder{
	NodeWiths: []*ui.NodeWith{
		{Value: &ui.NodeWith_Name{Name: "Continue"}},
		{Value: &ui.NodeWith_Role{Role: ui.Role_ROLE_BUTTON}},
	},
}
