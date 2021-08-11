// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package inputs

import (
	"context"
	"path/filepath"
	"time"

	"chromiumos/tast/ctxutil"
	"chromiumos/tast/local/bundles/cros/inputs/pre"
	"chromiumos/tast/local/bundles/cros/inputs/testserver"
	"chromiumos/tast/local/bundles/cros/inputs/util"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/ime"
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/chrome/uiauto/faillog"
	"chromiumos/tast/local/chrome/uiauto/nodewith"
	"chromiumos/tast/local/input"
	"chromiumos/tast/testing"
	"chromiumos/tast/testing/hwdep"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         PhysicalKeyboardEmojiSuggestion,
		Desc:         "Checks emoji suggestions with physical keyboard typing",
		Contacts:     []string{"shengjun@chromium.org", "essential-inputs-team@google.com"},
		Attr:         []string{"group:mainline", "group:input-tools", "informational"},
		SoftwareDeps: []string{"chrome"},
		Params: []testing.Param{{
			ExtraAttr:         []string{"group:input-tools-upstream"},
			ExtraHardwareDeps: hwdep.D(hwdep.Model(pre.StableModels...)),
		}, {
			Name:              "informational",
			ExtraHardwareDeps: hwdep.D(pre.InputsUnstableModels),
		}}})
}

func PhysicalKeyboardEmojiSuggestion(ctx context.Context, s *testing.State) {
	cr, err := chrome.New(ctx, chrome.ExtraArgs("--force-tablet-mode=clamshell"), chrome.EnableFeatures("EmojiSuggestAddition"), chrome.EnableFeatures("AssistEmojiEnhanced"))
	if err != nil {
		s.Fatal("Failed to start Chrome in clamshell mode: ", err)
	}
	defer cr.Close(ctx)

	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to connect Test API: ", err)
	}

	cleanupCtx := ctx
	ctx, cancel := ctxutil.Shorten(ctx, 5*time.Second)
	defer cancel()

	defer faillog.DumpUITreeOnError(cleanupCtx, s.OutDir(), s.HasError, tconn)

	// PK emoji suggestion only works in English(US).
	inputMethod := ime.EnglishUS

	// Activate function checks the current IME. It does nothing if the given input method is already in-use.
	// It is called here just in case IME has been changed in last test.
	if err := inputMethod.Activate(tconn)(ctx); err != nil {
		s.Fatal("Failed to set IME: ", err)
	}

	its, err := testserver.Launch(ctx, cr, tconn)
	if err != nil {
		s.Fatal("Failed to launch inputs test server: ", err)
	}
	defer its.Close()

	kb, err := input.Keyboard(ctx)
	if err != nil {
		s.Fatal("Failed to get keyboard: ", err)
	}
	defer kb.Close()

	const inputField = testserver.TextInputField

	testing.Sleep(ctx, 3*time.Minute)

	type suggestion struct {
		word      string
		emojiChar string
	}

	testData := []suggestion{
		{
			word:      "achoo",
			emojiChar: "😂",
		},
		{
			word:      "ZZZ",
			emojiChar: "😂",
		},
	}

	for _, data := range testData {
		// Using 'shortcut_{index} as test name.
		testName := data.word
		s.Run(ctx, testName, func(ctx context.Context, s *testing.State) {
			if err := its.Clear(inputField)(ctx); err != nil {
				s.Fatalf("Failed to clear input field %s: %v", string(inputField), err)
			}
			defer func() {
				outDir := filepath.Join(s.OutDir(), testName)
				faillog.DumpUITreeWithScreenshotOnError(ctx, outDir, s.HasError, cr, "ui_tree_"+testName)
			}()

			emojiCandidateWindowFinder := nodewith.NameStartingWith("Emoji")
			emojiCharFinder := nodewith.Name(data.emojiChar).Ancestor(emojiCandidateWindowFinder)

			ui := uiauto.New(tconn)

			if err := uiauto.Combine("validate emoji suggestion",
				its.ClickFieldAndWaitForActive(inputField),
				kb.TypeAction(data.word),
				kb.AccelAction("SPACE"),
				ui.LeftClick(emojiCharFinder),
				ui.WaitUntilGone(emojiCandidateWindowFinder),
				util.WaitForFieldTextToBe(tconn, inputField.Finder(), data.emojiChar),
			)(ctx); err != nil {
				s.Fatal("Failed to validate emoji suggestion: ", err)
			}
		})
	}
}
