// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package imesettings

import (
	"context"

	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/chrome/uiauto/nodewith"
	"chromiumos/tast/local/chrome/uiauto/ossettings"
	"chromiumos/tast/local/chrome/uiauto/role"
)

const suggestionSubPageURL = "osLanguages/smartInputs"

// LaunchAtSuggestionSettingsPage launches Settings app on suggestions page.
func LaunchAtSuggestionSettingsPage(ctx context.Context, tconn *chrome.TestConn, cr *chrome.Chrome) (*IMESettings, error) {
	ui := uiauto.New(tconn)
	suggestionPageHeading := nodewith.NameStartingWith("Suggestions").Role(role.Heading).Ancestor(ossettings.WindowFinder)
	settings, err := ossettings.LaunchAtPageURL(ctx, tconn, cr, suggestionSubPageURL, ui.Exists(suggestionPageHeading))
	if err != nil {
		return nil, err
	}
	return &IMESettings{settings.WithPollOpts(defaultPollOpts)}, nil
}
