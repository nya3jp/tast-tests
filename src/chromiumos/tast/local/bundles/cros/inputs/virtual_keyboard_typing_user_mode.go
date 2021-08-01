// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package inputs

import (
	"context"
	"path/filepath"
	"strings"
	"time"

	"chromiumos/tast/ctxutil"
	"chromiumos/tast/local/bundles/cros/inputs/data"
	"chromiumos/tast/local/bundles/cros/inputs/pre"
	"chromiumos/tast/local/bundles/cros/inputs/testserver"
	"chromiumos/tast/local/bundles/cros/inputs/util"
	"chromiumos/tast/local/chrome/ime"
	"chromiumos/tast/local/chrome/uiauto/faillog"
	"chromiumos/tast/local/chrome/uiauto/vkb"
	"chromiumos/tast/testing"
	"chromiumos/tast/testing/hwdep"
)

var userModeTestIMEs = []ime.InputMethod{
	ime.EnglishUS,
	ime.JapaneseWithUSKeyboard,
}

var userModeTestMessages = map[util.InputModality]data.Message{
	util.InputWithVK:          data.TypingMessageHello,
	util.InputWithHandWriting: data.HandwritingMessageHello,
	util.InputWithVoice:       data.VoiceMessageHello,
}

func init() {
	testing.AddTest(&testing.Test{
		Func:         VirtualKeyboardTypingUserMode,
		Desc:         "Checks that virtual keyboard works in different user modes",
		Contacts:     []string{"shengjun@chromium.org", "essential-inputs-team@google.com"},
		Attr:         []string{"group:mainline", "informational", "group:input-tools-upstream", "group:input-tools"},
		Data:         util.ExtractExternalFilesFromMap(userModeTestMessages, userModeTestIMEs),
		SoftwareDeps: []string{"chrome", "google_virtual_keyboard"},
		HardwareDeps: hwdep.D(pre.InputsStableModels),
		Timeout:      time.Duration(len(userModeTestIMEs)) * time.Duration(len(userModeTestMessages)) * time.Minute,
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

	its, err := testserver.LaunchInMode(ctx, cr, tconn, strings.HasSuffix(s.TestName(), "incognito"))
	if err != nil {
		s.Fatal("Failed to launch inputs test server: ", err)
	}
	defer its.Close()

	inputField := testserver.TextAreaInputField
	vkbCtx := vkb.NewContext(cr, tconn)

	subtest := func(testName string, modality util.InputModality, inputData data.InputData) func(ctx context.Context, s *testing.State) {
		return func(ctx context.Context, s *testing.State) {
			cleanupCtx := ctx
			// Use a shortened context for test operations to reserve time for cleanup.
			ctx, shortCancel := ctxutil.Shorten(ctx, 5*time.Second)
			defer shortCancel()

			defer func(ctx context.Context) {
				outDir := filepath.Join(s.OutDir(), testName)
				faillog.DumpUITreeWithScreenshotOnError(ctx, outDir, s.HasError, cr, "ui_tree_"+testName)

				if err := vkbCtx.HideVirtualKeyboard()(ctx); err != nil {
					s.Log("Failed to hide virtual keyboard: ", err)
				}
			}(cleanupCtx)

			if err := its.ValidateInputFieldForMode(inputField, modality, inputData, s.DataPath)(ctx); err != nil {
				s.Fatal("Failed to validate virtual keyboard input: ", err)
			}
		}
	}

	// Run defined subtest on virtual keyboard typing per input method and message combination.
	util.RunSubtestsPerInputMethodAndModalidy(ctx, tconn, s, userModeTestIMEs, userModeTestMessages, subtest)
}
