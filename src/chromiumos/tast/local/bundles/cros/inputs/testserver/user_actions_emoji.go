// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package testserver

import (
	"fmt"

	"chromiumos/tast/local/bundles/cros/inputs/inputactions"
	"chromiumos/tast/local/bundles/cros/inputs/util"
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/chrome/uiauto/nodewith"
	"chromiumos/tast/local/chrome/uiauto/role"
	"chromiumos/tast/local/chrome/useractions"
	"chromiumos/tast/local/input"
)

// InputEmojiWithEmojiPicker returns an user action to input Emoji with PK emoji picker on E14s test server.
func (its *InputsTestServer) InputEmojiWithEmojiPicker(uc *useractions.UserContext, kb *input.KeyboardEventWriter, inputField InputField) *useractions.UserAction {
	emojiChar := "😂"
	emojiMenuFinder := nodewith.NameStartingWith("Emoji")
	emojiPickerFinder := nodewith.Name("Emoji Picker").Role(role.RootWebArea)
	emojiCharFinder := nodewith.Name(emojiChar).First().Ancestor(emojiPickerFinder)

	ui := uiauto.New(uc.TestAPIConn())
	return useractions.NewUserAction(
		"Emoji Picker",
		uiauto.Combine(fmt.Sprintf("input emoji with emoji picker on field %v", inputField),
			its.Clear(inputField),
			// Right click input to trigger context menu and select Emoji.
			its.RightClickFieldAndWaitForActive(inputField),
			ui.LeftClick(emojiMenuFinder),
			// Select item from emoji picker.
			ui.LeftClick(emojiCharFinder),
			// Wait for input value to test emoji.
			util.WaitForFieldTextToBe(uc.TestAPIConn(), inputField.Finder(), emojiChar),
		),
		uc,
		useractions.UserActionCfg{
			ActionAttributes: map[string]string{useractions.AttributeInputField: string(inputField)},
			ActionTags:       []string{inputactions.ActionTagEmoji, inputactions.ActionTagEmojiPicker},
		})
}

// InputEmojiFromSuggestion returns an user action to input Emoji from suggestion on E14s test server.
func (its *InputsTestServer) InputEmojiFromSuggestion(uc *useractions.UserContext, kb *input.KeyboardEventWriter, inputField InputField) *useractions.UserAction {
	const (
		word  = "yum"
		emoji = "🤤"
	)

	ui := uiauto.New(uc.TestAPIConn())

	emojiCandidateWindowFinder := nodewith.HasClass("SuggestionWindowView").Role(role.Window)
	emojiCharFinder := nodewith.Name(emoji).Ancestor(emojiCandidateWindowFinder).HasClass("StyledLabel")

	return useractions.NewUserAction(
		"Emoji suggestion",
		uiauto.Combine(fmt.Sprintf("input emoji from suggestion on field %v", inputField),
			its.Clear(inputField),
			its.ClickFieldAndWaitForActive(inputField),
			kb.TypeAction(word),
			kb.AccelAction("SPACE"),
			ui.LeftClick(emojiCharFinder),
			ui.WaitUntilGone(emojiCandidateWindowFinder),
			util.WaitForFieldTextToBe(uc.TestAPIConn(), inputField.Finder(), word+" "+emoji),
		),
		uc,
		useractions.UserActionCfg{
			ActionAttributes: map[string]string{useractions.AttributeInputField: string(inputField)},
			ActionTags:       []string{inputactions.ActionTagEmoji, inputactions.ActionTagEmojiSuggestion},
		})
}
