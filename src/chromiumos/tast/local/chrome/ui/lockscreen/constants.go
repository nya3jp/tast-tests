// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package lockscreen

import "chromiumos/tast/local/chrome/ui"

// HidePasswordBtnParams are the UI params for the "Hide password" button on Lock/Start screen.
var HidePasswordBtnParams ui.FindParams = ui.FindParams{
	Name: "Hide password",
	Role: ui.RoleTypeToggleButton,
}

// ShowPasswordBtnParams are the UI params for the "Show password" button on Lock/Start screen.
var ShowPasswordBtnParams ui.FindParams = ui.FindParams{
	Name: "Show password",
	Role: ui.RoleTypeButton,
}

// SubmitBtnParams are the UI params for the "Submit" button on Lock/Start screen.
var SubmitBtnParams ui.FindParams = ui.FindParams{
	Name: "Submit",
	Role: ui.RoleTypeButton,
}

// SwitchToPwdBtnParams are the UI params for the "Switch to password" button on Lock/Start screen.
var SwitchToPwdBtnParams ui.FindParams = ui.FindParams{
	Name: "Switch to password",
	Role: ui.RoleTypeButton,
}
