// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package inputs

import (
	"context"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"chromiumos/tast/ctxutil"
	"chromiumos/tast/local/bundles/cros/inputs/data"
	"chromiumos/tast/local/bundles/cros/inputs/pre"
	"chromiumos/tast/local/bundles/cros/inputs/testserver"
	"chromiumos/tast/local/bundles/cros/inputs/util"
	"chromiumos/tast/local/chrome/ime"
	"chromiumos/tast/local/chrome/uiauto/faillog"
	"chromiumos/tast/local/chrome/useractions"
	"chromiumos/tast/testing"
	"chromiumos/tast/testing/hwdep"
)

var typingModeTestIMEs = []ime.InputMethod{
	ime.EnglishUS,
	ime.JapaneseWithUSKeyboard,
}
var typingModeTestMessages = []data.Message{data.TypingMessageHello}

func init() {
	testing.AddTest(&testing.Test{
		Func:         VirtualKeyboardTypingUserMode,
		Desc:         "Checks that virtual keyboard works in different user modes",
		Contacts:     []string{"shengjun@chromium.org", "essential-inputs-team@google.com"},
		Attr:         []string{"group:input-tools-upstream", "group:input-tools"},
		SoftwareDeps: []string{"chrome", "google_virtual_keyboard"},
		HardwareDeps: hwdep.D(pre.InputsStableModels),
		Timeout:      time.Duration(len(typingModeTestIMEs)) * time.Duration(len(typingModeTestMessages)) * time.Minute,
		Params: []testing.Param{
			{
				Name: "guest",
				Pre:  pre.VKEnabledInGuest,
			},
			{
				Name: "incognito",
				Pre:  pre.VKEnabledReset,
			},
		},
	})
}

func VirtualKeyboardTypingUserMode(ctx context.Context, s *testing.State) {
	cr := s.PreValue().(pre.PreData).Chrome
	tconn := s.PreValue().(pre.PreData).TestAPIConn
	uc := s.PreValue().(pre.PreData).UserContext

	its, err := testserver.LaunchInMode(ctx, cr, tconn, strings.HasSuffix(s.TestName(), "incognito"))
	if err != nil {
		s.Fatal("Failed to launch inputs test server: ", err)
	}
	defer its.Close()

	if strings.HasSuffix(s.TestName(), "incognito") {
		uc.SetAttribute(useractions.AttributeIncognitoMode, strconv.FormatBool(true))
		// Remove the incognito attribute after test.
		defer uc.RemoveAttribute(useractions.AttributeIncognitoMode)
	}

	inputField := testserver.TextAreaInputField

	for _, im := range typingModeTestIMEs {
		if err := im.InstallAndActivate(tconn)(ctx); err != nil {
			s.Fatalf("Failed to install and activate input method %q: %v", im, err)
		}
		uc.SetAttribute(useractions.AttributeInputMethod, im.Name)

		inputData, ok := data.TypingMessageHello.GetInputData(im)
		if !ok {
			s.Fatal("Failed to get input data: ", err)
		}

		validationAction := its.ValidateInputFieldForMode(uc, inputField, util.InputWithVK, inputData, nil)

		testName := string(im.Name) + "-" + string(inputData.ExpectedText)
		s.Run(ctx, testName, func(ctx context.Context, s *testing.State) {
			cleanupCtx := ctx
			ctx, cancel := ctxutil.Shorten(ctx, 5*time.Second)
			defer cancel()
			defer faillog.DumpUITreeWithScreenshotOnError(cleanupCtx, filepath.Join(s.OutDir(), testName), s.HasError, cr, "ui_tree_"+testName)

			if err := validationAction.Run(ctx); err != nil {
				s.Fatalf("Subtest %q failed: %v", testName, err)
			}
		})
	}
}
