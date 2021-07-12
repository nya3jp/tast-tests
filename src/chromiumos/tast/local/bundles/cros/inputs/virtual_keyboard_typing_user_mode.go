// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package inputs

import (
	"context"
	"time"

	"chromiumos/tast/local/bundles/cros/inputs/pre"
	"chromiumos/tast/local/bundles/cros/inputs/testserver"
	"chromiumos/tast/local/chrome/ime"
	"chromiumos/tast/local/chrome/uiauto/faillog"
	"chromiumos/tast/testing"
	"chromiumos/tast/testing/hwdep"
)

type testParams struct {
	inputMethodID ime.InputMethodCode
	userMode      string
}

func init() {
	testing.AddTest(&testing.Test{
		Func:         VirtualKeyboardTypingUserMode,
		Desc:         "Checks that virtual keyboard works in different user modes",
		Contacts:     []string{"shengjun@chromium.org", "essential-inputs-team@google.com"},
		Attr:         []string{"group:mainline", "informational", "group:input-tools-upstream", "group:input-tools"},
		SoftwareDeps: []string{"chrome", "google_virtual_keyboard"},
		HardwareDeps: hwdep.D(pre.InputsStableModels),
		Timeout:      5 * time.Minute,
		Params: []testing.Param{{
			Name: "us_en_guest",
			Val: testParams{
				inputMethodID: ime.INPUTMETHOD_XKB_US_ENG,
				userMode:      "guest",
			},
			Pre: pre.VKEnabledInGuest,
		}, {
			Name: "us_en_incognito",
			Val: testParams{
				inputMethodID: ime.INPUTMETHOD_XKB_US_ENG,
				userMode:      "incognito",
			},
			Pre: pre.VKEnabledReset,
		}, {
			Name: "jp_jpn_guest",
			Val: testParams{
				inputMethodID: ime.INPUTMETHOD_XKB_JP_JPN,
				userMode:      "guest",
			},
			Pre: pre.VKEnabledReset,
		}, {
			Name: "jp_jpn_incognito",
			Val: testParams{
				inputMethodID: ime.INPUTMETHOD_XKB_JP_JPN,
				userMode:      "incognito",
			},
			Pre: pre.VKEnabledReset,
		}},
	})
}

func VirtualKeyboardTypingUserMode(ctx context.Context, s *testing.State) {
	testParams := s.Param().(testParams)

	cr := s.PreValue().(pre.PreData).Chrome
	tconn := s.PreValue().(pre.PreData).TestAPIConn

	its, err := testserver.LaunchInMode(ctx, cr, tconn, testParams.userMode == "incognito")
	if err != nil {
		s.Fatal("Failed to launch inputs test server: ", err)
	}
	defer its.Close()

	defer faillog.DumpUITreeOnError(ctx, s.OutDir(), s.HasError, tconn)

	if err := its.ValidateVKInputOnField(testserver.TextAreaInputField, testParams.inputMethodID)(ctx); err != nil {
		s.Fatalf("Failed to VK input in %q mode using input method %q", testParams.userMode, string(testParams.inputMethodID))
	}
}
