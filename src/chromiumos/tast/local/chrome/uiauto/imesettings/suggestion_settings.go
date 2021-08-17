// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package imesettings

import (
	"context"
	"fmt"
	"time"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/chrome/uiauto/nodewith"
	"chromiumos/tast/local/chrome/uiauto/ossettings"
	"chromiumos/tast/local/chrome/uiauto/role"
	"chromiumos/tast/testing"
)

const suggestionSubPageURL = "osLanguages/smartInputs"

var emojiSuggestionToggleButton = nodewith.Name("Emoji suggestions").Role(role.ToggleButton)

// LaunchAtSuggestionSettingsPage launches Settings app on suggestions page.
func LaunchAtSuggestionSettingsPage(ctx context.Context, tconn *chrome.TestConn, cr *chrome.Chrome) (*IMESettings, error) {
	ui := uiauto.New(tconn)
	suggestionPageHeading := nodewith.NameStartingWith("Suggestions").Role(role.Heading).Ancestor(ossettings.WindowFinder)
	settings, err := ossettings.LaunchAtPageURL(ctx, tconn, cr, suggestionSubPageURL, ui.Exists(suggestionPageHeading))
	if err != nil {
		return nil, err
	}
	return &IMESettings{settings: settings.WithPollOpts(defaultPollOpts)}, nil
}

// ToggleEmojiSuggestions returns an action to click the 'Emoji suggestions' toggle button.
func (i *IMESettings) ToggleEmojiSuggestions(tconn *chrome.TestConn) uiauto.Action {
	return i.settings.LeftClick(emojiSuggestionToggleButton)
}

// WaitUntilEmojiSuggestion returns an action waits until emoji suggestion in expected state.
func (i *IMESettings) WaitUntilEmojiSuggestion(cr *chrome.Chrome, tconn *chrome.TestConn, expected bool) uiauto.Action {
	const toggleButtonCSSSelector = `cr-toggle[aria-label="Emoji suggestions"]`
	expr := fmt.Sprintf(`
		var optionNode = shadowPiercingQuery(%q);
		if(optionNode == undefined){
			throw new Error("Emoji suggestions setting item is not found.");
		}
		optionNode.getAttribute("aria-pressed")==="true";
		`, toggleButtonCSSSelector)

	return uiauto.New(tconn).WithInterval(time.Second).Retry(5, func(ctx context.Context) error {
		var actual bool
		if err := i.settings.EvalJSWithShadowPiercer(ctx, cr, expr, &actual); err != nil {
			return testing.PollBreak(err)
		}
		if actual != expected {
			return errors.Errorf(`'Emoji suggestions' option value is incorrect. got %v; want %v`, actual, expected)
		}
		return nil
	})
}
