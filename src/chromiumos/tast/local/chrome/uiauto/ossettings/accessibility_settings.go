// Copyright 2022 The ChromiumOS Authors.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package ossettings

import (
	"context"

	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/chrome/uiauto/nodewith"
	"chromiumos/tast/local/chrome/uiauto/role"
)

const (
	manageA11yFeaturePageURL = "manageAccessibility"
	onScreenKeyboardOption   = "On-screen keyboard"
)

// LaunchAtManageA11yFeaturePage launches Settings app on language page.
func LaunchAtManageA11yFeaturePage(ctx context.Context, tconn *chrome.TestConn, cr *chrome.Chrome) (*OSSettings, error) {
	ui := uiauto.New(tconn)
	languagesPageHeading := nodewith.NameStartingWith("Manage accessibility features").Role(role.Heading).Ancestor(WindowFinder)
	return LaunchAtPageURL(ctx, tconn, cr, manageA11yFeaturePageURL, ui.Exists(languagesPageHeading))
}

// SetOnScreenKeyboard sets the option of On-screen keyboard.
func (s *OSSettings) SetOnScreenKeyboard(cr *chrome.Chrome, enabled bool) uiauto.Action {
	return s.SetToggleOption(cr, onScreenKeyboardOption, enabled)
}
