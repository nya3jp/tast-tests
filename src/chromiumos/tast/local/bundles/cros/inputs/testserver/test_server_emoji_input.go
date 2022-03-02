// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package testserver

import (
	"fmt"
	"time"

	"chromiumos/tast/local/bundles/cros/inputs/util"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/chrome/uiauto/nodewith"
	"chromiumos/tast/local/chrome/uiauto/role"
	"chromiumos/tast/local/chrome/useractions"
	"chromiumos/tast/local/input"
)

var emojiMenuFinder = nodewith.NameStartingWith("Emoji")
var emojiPickerFinder = nodewith.Name("Emoji Picker").Role(role.RootWebArea)

func newEmojiUICtx(tconn *chrome.TestConn) *uiauto.Context {
	return uiauto.New(tconn).WithTimeout(30 * time.Second)
}

// InputEmojiWithEmojiPicker returns a user action to input Emoji with PK emoji picker on E14s test server.
func (its *InputsTestServer) InputEmojiWithEmojiPicker(uc *useractions.UserContext, inputField InputField, emojiChar string) uiauto.Action {
	emojiCharFinder := nodewith.Name(emojiChar).First().Ancestor(emojiPickerFinder)
	ui := newEmojiUICtx(uc.TestAPIConn())

	action := uiauto.Combine(fmt.Sprintf("input emoji with emoji picker on field %v", inputField),
		its.Clear(inputField),
		// Right click input to trigger context menu and select Emoji.
		its.RightClickFieldAndWaitForActive(inputField),
		ui.LeftClick(emojiMenuFinder),
		// Select item from emoji picker.
		ui.LeftClick(emojiCharFinder),
		// Wait for input value to test emoji.
		util.WaitForFieldTextToBe(uc.TestAPIConn(), inputField.Finder(), emojiChar),
	)

	return uiauto.UserAction(
		"Input Emoji with Emoji Picker",
		action,
		uc,
		&useractions.UserActionCfg{
			Attributes: map[string]string{
				useractions.AttributeFeature:      useractions.FeatureEmojiPicker,
				useractions.AttributeInputField:   string(inputField),
				useractions.AttributeTestScenario: fmt.Sprintf("Input %q into %q field via emoji picker", emojiChar, string(inputField)),
			},
			Tags: []useractions.ActionTag{
				useractions.ActionTagEssentialInputs,
			},
		})
}

// InputEmojiWithEmojiPickerSearch returns a user action to input Emoji with PK emoji picker on E14s test server using search.
func (its *InputsTestServer) InputEmojiWithEmojiPickerSearch(uc *useractions.UserContext, inputField InputField, keyboard *input.KeyboardEventWriter, searchString, emojiChar string) uiauto.Action {
	emojiSearchFinder := nodewith.Name("Search").Role(role.SearchBox).Ancestor(emojiPickerFinder)
	emojiResultFinder := nodewith.Name(fmt.Sprintf("%s %s", emojiChar, searchString))
	ui := newEmojiUICtx(uc.TestAPIConn())

	action := uiauto.Combine(fmt.Sprintf("input emoji with emoji picker on field %v", inputField),
		its.Clear(inputField),
		// Right click input to trigger context menu and select Emoji.
		its.RightClickFieldAndWaitForActive(inputField),
		ui.LeftClick(emojiMenuFinder),
		ui.LeftClick(emojiSearchFinder),
		keyboard.TypeAction(searchString),
		ui.LeftClick(emojiResultFinder),
		// Wait for input value to be test Emoji.
		util.WaitForFieldTextToBe(uc.TestAPIConn(), inputField.Finder(), emojiChar),
	)

	return uiauto.UserAction(
		"Input Emoji by search in Emoji Picker",
		action,
		uc,
		&useractions.UserActionCfg{
			Attributes: map[string]string{
				useractions.AttributeFeature:      useractions.FeatureEmojiPicker,
				useractions.AttributeInputField:   string(inputField),
				useractions.AttributeTestScenario: fmt.Sprintf("Search %q for %q then submit", searchString, emojiChar),
			},
			Tags: []useractions.ActionTag{
				useractions.ActionTagEssentialInputs,
			},
		})
}
