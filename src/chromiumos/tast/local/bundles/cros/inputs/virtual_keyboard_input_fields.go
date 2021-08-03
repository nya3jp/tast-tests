// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package inputs

import (
	"context"
	"path/filepath"
	"time"

	"chromiumos/tast/ctxutil"
	"chromiumos/tast/local/bundles/cros/inputs/data"
	"chromiumos/tast/local/bundles/cros/inputs/pre"
	"chromiumos/tast/local/bundles/cros/inputs/testserver"
	"chromiumos/tast/local/chrome/ime"
	"chromiumos/tast/local/chrome/uiauto/faillog"
	"chromiumos/tast/local/chrome/uiauto/vkb"
	"chromiumos/tast/testing"
	"chromiumos/tast/testing/hwdep"
)

var inputFieldTestIMEs = []ime.InputMethod{
	ime.JapaneseWithUSKeyboard,
	ime.ChinesePinyin,
	ime.EnglishUS,
}

var inputFieldToMessage = map[testserver.InputField]data.Message{
	testserver.TextAreaInputField:    data.TypingMessageHello,
	testserver.TextInputField:        data.TypingMessageHello,
	testserver.SearchInputField:      data.TypingMessageHello,
	testserver.PasswordInputField:    data.TypingMessagePassword,
	testserver.NumberInputField:      data.TypingMessageNumber,
	testserver.EmailInputField:       data.TypingMessageEmail,
	testserver.URLInputField:         data.TypingMessageURL,
	testserver.TelInputField:         data.TypingMessageTel,
	testserver.TextInputNumericField: data.TypingMessageNumber,
}

func init() {
	testing.AddTest(&testing.Test{
		Func:         VirtualKeyboardInputFields,
		Desc:         "Checks that virtual keyboard works on different input fields",
		Contacts:     []string{"shengjun@chromium.org", "essential-inputs-team@google.com"},
		Attr:         []string{"group:mainline", "informational", "group:input-tools-upstream", "group:input-tools"},
		SoftwareDeps: []string{"chrome", "google_virtual_keyboard"},
		Pre:          pre.VKEnabledTabletReset,
		Timeout:      time.Duration(len(inputFieldTestIMEs)) * time.Duration(len(inputFieldToMessage)) * time.Minute,
		Params: []testing.Param{
			{
				ExtraHardwareDeps: hwdep.D(pre.InputsStableModels),
			},
			{
				Name:              "informational",
				ExtraHardwareDeps: hwdep.D(pre.InputsUnstableModels),
			},
		},
	})
}

func VirtualKeyboardInputFields(ctx context.Context, s *testing.State) {
	cr := s.PreValue().(pre.PreData).Chrome
	tconn := s.PreValue().(pre.PreData).TestAPIConn

	vkbCtx := vkb.NewContext(cr, tconn)

	its, err := testserver.Launch(ctx, cr, tconn)
	if err != nil {
		s.Fatal("Failed to launch inputs test server: ", err)
	}
	defer its.Close()

	subtest := func(testName string, inputMethod ime.InputMethod, message data.Message, inputField testserver.InputField) func(ctx context.Context, s *testing.State) {
		return func(ctx context.Context, s *testing.State) {
			inputData, ok := message.GetInputData(inputMethod)
			if !ok {
				s.Fatalf("Test Data for input method %v does not exist", inputMethod)
			}

			cleanupCtx := ctx
			// Use a shortened context for test operations to reserve time for cleanup.
			ctx, cancel := ctxutil.Shorten(ctx, 10*time.Second)
			defer cancel()

			defer func(ctx context.Context) {
				outDir := filepath.Join(s.OutDir(), testName)
				faillog.DumpUITreeWithScreenshotOnError(ctx, outDir, s.HasError, cr, "ui_tree_"+testName)

				if err := vkbCtx.HideVirtualKeyboard()(ctx); err != nil {
					s.Log("Failed to hide virtual keyboard: ", err)
				}
			}(cleanupCtx)

			if err := its.ValidateVKInputOnField(vkbCtx, inputField, inputData)(ctx); err != nil {
				s.Fatal("Failed to validate virtual keyboard input: ", err)
			}
		}
	}

	for _, inputMethod := range inputFieldTestIMEs {
		if err := inputMethod.InstallAndActivate(tconn)(ctx); err != nil {
			s.Fatalf("Failed to set input method to %q: %v: ", inputMethod, err)
		}

		for inputField, message := range inputFieldToMessage {
			testName := inputMethod.String() + "-" + string(inputField)
			s.Run(ctx, testName, subtest(testName, inputMethod, message, inputField))
		}
	}
}
