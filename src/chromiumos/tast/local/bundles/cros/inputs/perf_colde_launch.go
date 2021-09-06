// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package inputs

import (
	"context"
	"strings"
	"time"

	"chromiumos/tast/ctxutil"
	"chromiumos/tast/local/bundles/cros/inputs/pre"
	"chromiumos/tast/local/bundles/cros/inputs/testserver"
	"chromiumos/tast/local/bundles/cros/inputs/util"
	"chromiumos/tast/local/chrome/ime"
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/chrome/uiauto/faillog"
	"chromiumos/tast/local/input"
	"chromiumos/tast/testing"
	"chromiumos/tast/testing/hwdep"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         ColdLaunchAndWaitUtilReady,
		Desc:         "Measure CrOS IME cold launch performance",
		Contacts:     []string{"googleo@chromium.org", "essential-inputs-team@google.com"},
		Attr:         []string{"group:input-tools", "group:input-tools-upstream"},
		SoftwareDeps: []string{"chrome"},
		Pre:          pre.NonVKClamshell,
		HardwareDeps: hwdep.D(pre.InputsStableModels),
		Timeout:      3 * time.Minute,
		Params: []testing.Param{
			{
				Name: "jp",
				Val:  ime.Japanese,
			},
		},
	})
}

func ColdLaunchAndWaitUtilReady(ctx context.Context, s *testing.State) {
	cr := s.PreValue().(pre.PreData).Chrome

	cleanupCtx := ctx
	ctx, cancel := ctxutil.Shorten(ctx, 5*time.Second)
	defer cancel()

	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to connect Test API: ", err)
	}
	defer faillog.DumpUITreeOnError(cleanupCtx, s.OutDir(), s.HasError, tconn)

	// Add IME for testing.
	imeCode := ime.ChromeIMEPrefix + s.Param().(ime.InputMethod).ID

	s.Logf("Add new input method: %s", imeCode)
	if err := ime.AddInputMethod(ctx, tconn, imeCode); err != nil {
		s.Fatalf("Failed to set input method to %s: %v: ", imeCode, err)
	}

	kb, err := input.Keyboard(ctx)
	if err != nil {
		s.Fatal("Failed to get keyboard: ", err)
	}
	defer kb.Close()

	its, err := testserver.Launch(ctx, cr, tconn)
	if err != nil {
		s.Fatal("Fail to launch inputs test server: ", err)
	}
	defer its.Close()

	inputField := testserver.TextAreaInputField

	// Focus on the input field and wait for it's focused.
	if err := its.ClickFieldAndWaitForActive(inputField)(ctx); err != nil {
		s.Fatal("Failed to wait for input field to activate: ", err)
	}

	// ui := uiauto.New(tconn)

	// Always clear the text field.
	// clearAndFocus := uiauto.Combine("clear input field and focus",
	// 	its.Clear(inputField),
	// 	its.ClickFieldAndWaitForActive(inputField),
	// )

	s.Logf("Enable  input method: %s", imeCode)

	if err := tconn.Call(ctx, nil, `chrome.inputMethodPrivate.setCurrentInputMethod`, imeCode); err != nil {
		s.Fatalf("Failed to activate %s: %v: ", imeCode, err)
	}

	subtests := []struct {
		Name   string
		Action uiauto.Action
	}{
		// Type 'a'.
		// The text field should show the selected candidate.
		{
			Name: "FirstDecodedChar",
			Action: uiauto.Combine("Repeat Type 'a' for 20",
				kb.TypeAction(strings.Repeat("a", 20)),
				kb.AccelAction("Enter"),
			),
		},
		// Type and Tab several times to select a candidate.
		// Press Enter, which should submit the selected candidate and hide the candidates window.
		{
			Name: "Cal a number",
			Action: uiauto.Combine("calculate number of 'a'",
				util.WaitForFieldTextToBe(tconn, inputField.Finder(), "aaaaaa"),
			),
		},
	}

	for _, subtest := range subtests {
		s.Run(ctx, subtest.Name, func(ctx context.Context, s *testing.State) {
			defer faillog.DumpUITreeWithScreenshotOnError(ctx, s.OutDir(), s.HasError, cr, "ui_tree_"+string(subtest.Name))

			if err := subtest.Action(ctx); err != nil {
				s.Fatalf("Failed to validate keys input in %s: %v", inputField, err)
			}
		})
	}
}
