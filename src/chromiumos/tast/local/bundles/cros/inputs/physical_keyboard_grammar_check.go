// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package inputs

import (
	"context"

	"chromiumos/tast/local/bundles/cros/inputs/pre"
	"chromiumos/tast/local/bundles/cros/inputs/testserver"
	"chromiumos/tast/local/bundles/cros/inputs/util"
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/chrome/uiauto/faillog"
	"chromiumos/tast/local/input"
	"chromiumos/tast/testing"
	"chromiumos/tast/testing/hwdep"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         PhysicalKeyboardGrammarCheck,
		Desc:         "Checks on device grammar check with physical keyboard typing",
		Contacts:     []string{"shengjun@chromium.org", "essential-inputs-team@google.com"},
		Attr:         []string{"group:mainline", "group:input-tools", "informational"},
		HardwareDeps: hwdep.D(hwdep.Model(pre.GrammarEnabledModels...)),
		SoftwareDeps: []string{"chrome"},
		Pre:          pre.NonVKClamshellWithGrammarCheck,
	})
}

func PhysicalKeyboardGrammarCheck(ctx context.Context, s *testing.State) {
	const (
		inputText    = "They is student."
		expectedText = "They are students."
	)

	cr := s.PreValue().(pre.PreData).Chrome
	tconn := s.PreValue().(pre.PreData).TestAPIConn

	defer faillog.DumpUITreeWithScreenshotOnError(ctx, s.OutDir(), s.HasError, cr, "ui_tree")

	its, err := testserver.Launch(ctx, cr, tconn)
	if err != nil {
		s.Fatal("Failed to launch inputs test server: ", err)
	}
	defer its.Close()

	keyboard, err := input.Keyboard(ctx)
	if err != nil {
		s.Fatal("Failed to get keyboard: ", err)
	}
	defer keyboard.Close()

	inputField := testserver.TextAreaInputField

	if err := uiauto.Combine("accept grammar check suggestion",
		its.Clear(inputField),
		its.ClickFieldAndWaitForActive(inputField),
		keyboard.TypeAction(inputText),
		util.WaitForFieldTextToBeIgnoringCase(tconn, inputField.Finder(), inputText),
		keyboard.AccelAction("Left"),
		keyboard.AccelAction("Left"),
		keyboard.AccelAction("Tab"),
		keyboard.AccelAction("Enter"),
		util.WaitForFieldTextToBeIgnoringCase(tconn, inputField.Finder(), expectedText),
	)(ctx); err != nil {
		s.Fatal("Fail to accept grammar check suggestion: ", err)
	}
}
