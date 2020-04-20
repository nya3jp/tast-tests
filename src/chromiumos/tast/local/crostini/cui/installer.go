// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package cui contains functions to interact with the ChromeOS parts of the crostini UI.
// This is primarily the settings and the installer.
package cui

import (
	"time"

	"chromiumos/tast/local/apps"
	"chromiumos/tast/local/chrome/ui"
	"chromiumos/tast/local/crostini/uic"
)

// InstallCrostini returns a UI automation graph that installs Crostini
// with the given username and disk size.
func InstallCrostini() *uic.Node {
	// TODO(mwarton): username and diskSize handling.
	return uic.Steps(
		uic.Launch("Settings", apps.Settings.ID),
		uic.Find(ui.FindParams{Role: ui.RoleTypeButton, Name: "Linux (Beta)"}).Focus().LeftClick(),
		uic.Find(ui.FindParams{Role: ui.RoleTypeButton, Name: "Next"}).LeftClick(),
		uic.Find(ui.FindParams{Role: ui.RoleTypeButton, Name: "Install"}).LeftClick(),
		uic.WaitUntilGone(ui.FindParams{Role: ui.RoleTypeButton, Name: "Cancel"}, 2*time.Minute))
}
