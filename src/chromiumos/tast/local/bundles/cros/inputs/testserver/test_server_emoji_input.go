// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package testserver

import (
	"fmt"

	"chromiumos/tast/local/bundles/cros/inputs/emojipicker"
	"chromiumos/tast/local/bundles/cros/inputs/util"
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/chrome/uiauto/nodewith"
	"chromiumos/tast/local/chrome/useractions"
	"chromiumos/tast/local/input"
)

var emojiMenuFinder = nodewith.NameStartingWith("Emoji")

// TriggerEmojiPickerFromContextMenu launches emoji picker from context menu.
func (its *InputsTestServer) TriggerEmojiPickerFromContextMenu(inputField InputField) uiauto.Action {
	return uiauto.Combine("launches emoji picker from context menu",
		its.RightClickFieldAndWaitForActive(inputField),
		its.ui.LeftClick(emojiMenuFinder),
		emojipicker.WaitUntilExists(its.tconn),
	)
}

// InputEmojiWithEmojiPicker returns a user action to input Emoji with PK emoji picker on E14s test server.
func (its *InputsTestServer) InputEmojiWithEmojiPicker(uc *useractions.UserContext, inputField InputField, emojiChar string) uiauto.Action {
	emojiCharFinder := emojipicker.NodeFinder.Name(emojiChar).First()
	ui := emojipicker.NewUICtx(its.tconn)

	action := uiauto.Combine(fmt.Sprintf("input emoji with emoji picker on field %v", inputField),
		its.Clear(inputField),
		its.TriggerEmojiPickerFromContextMenu(inputField),
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
			Attributes: map[string]string{useractions.AttributeInputField: string(inputField)},
			Tags:       []useractions.ActionTag{useractions.ActionTagEmoji, useractions.ActionTagEmojiPicker},
		})
}

// InputEmojiWithEmojiPickerSearch returns a user action to input Emoji with PK emoji picker on E14s test server using search.
func (its *InputsTestServer) InputEmojiWithEmojiPickerSearch(uc *useractions.UserContext, inputField InputField, keyboard *input.KeyboardEventWriter, searchString, emojiChar string) uiauto.Action {
	emojiResultFinder := nodewith.Name(fmt.Sprintf("%s %s", emojiChar, searchString))
	ui := emojipicker.NewUICtx(its.tconn)

	action := uiauto.Combine(fmt.Sprintf("input emoji with emoji picker on field %v", inputField),
		its.Clear(inputField),
		its.TriggerEmojiPickerFromContextMenu(inputField),
		ui.LeftClick(emojipicker.SearchFieldFinder),
		keyboard.TypeAction(searchString),
		ui.LeftClick(emojiResultFinder),
		// Wait for input value to be test Emoji.
		util.WaitForFieldTextToBe(uc.TestAPIConn(), inputField.Finder(), emojiChar),
	)

	return uiauto.UserAction(
		"Input Emoji with Emoji Picker",
		action,
		uc,
		&useractions.UserActionCfg{
			Attributes: map[string]string{useractions.AttributeInputField: string(inputField), useractions.AttributeTestScenario: "Input emoji by searching in emoji picker"},
			Tags:       []useractions.ActionTag{useractions.ActionTagEmoji, useractions.ActionTagEmojiPicker},
		})
}
