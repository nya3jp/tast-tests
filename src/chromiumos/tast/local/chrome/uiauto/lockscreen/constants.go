// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package lockscreen

import (
	"chromiumos/tast/local/chrome/uiauto/nodewith"
	"chromiumos/tast/local/chrome/uiauto/role"
)

// HidePasswordButton is the finder for the "Hide password" button on Lock/Start screen.
var HidePasswordButton = nodewith.Role(role.ToggleButton).ClassName("ToggleImageButton").Name("Hide password")

// ShowPasswordButton is the finder for the "Show password" button on Lock/Start screen.
var ShowPasswordButton = nodewith.Role(role.Button).ClassName("ToggleImageButton").Name("Show password")

// SubmitButton is the finder for the "Submit" button on Lock/Start screen.
var SubmitButton = nodewith.Name("Submit").Role(role.Button)

// SwitchToPasswordButton is the finder for the "Switch to password" button on Lock/Start screen.
var SwitchToPasswordButton = nodewith.Role(role.Button).ClassName("LabelButton").Name("Switch to password")
