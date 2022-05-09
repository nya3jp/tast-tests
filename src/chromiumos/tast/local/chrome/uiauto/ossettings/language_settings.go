// Copyright 2022 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package ossettings

import (
	"context"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/chrome/uiauto/nodewith"
	"chromiumos/tast/local/chrome/uiauto/role"
	"chromiumos/tast/local/input"
)

const languageSubPageURL = "osLanguages/languages"

// LaunchAtLanguageSettingsPage launches Settings app on language page.
func LaunchAtLanguageSettingsPage(ctx context.Context, tconn *chrome.TestConn, cr *chrome.Chrome) (*OSSettings, error) {
	ui := uiauto.New(tconn)
	languagesPageHeading := nodewith.NameStartingWith("Languages").Role(role.Heading).Ancestor(WindowFinder)
	return LaunchAtPageURL(ctx, tconn, cr, languageSubPageURL, ui.Exists(languagesPageHeading))
}

// ChangeDeviceLanguageAndReboot changes to certain device language and reboot device to take effect.
func (s *OSSettings) ChangeDeviceLanguageAndReboot(ctx context.Context, tconn *chrome.TestConn, searchKeyword, languageUniqueIdentifier string) error {
	kb, err := input.Keyboard(ctx)
	if err != nil {
		return errors.Wrap(err, "failed to create virtual keyboard device")
	}
	defer kb.Close()

	// Button to trigger changing device language.
	changeButton := nodewith.NameContaining("Change device language").Role(role.Button)

	newLanguageOption := nodewith.NameContaining(languageUniqueIdentifier).HasClass("list-item")
	confirmAndRetartButton := nodewith.Name("Confirm and restart").Role(role.Button)

	return uiauto.Combine("change device language",
		s.ui.DoDefault(changeButton),
		kb.TypeAction(searchKeyword),
		s.ui.LeftClick(newLanguageOption),
		s.ui.DoDefault(confirmAndRetartButton),
	)(ctx)
}
