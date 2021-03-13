// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package inputs

import (
	"context"
	"time"

	"chromiumos/tast/ctxutil"
	"chromiumos/tast/local/bundles/cros/inputs/testserver"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/ime"
	"chromiumos/tast/local/chrome/uiauto/faillog"
	"chromiumos/tast/local/input"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         PhysicalKeyboardInputFields,
		Desc:         "Checks that physical keyboard works on different input fields",
		Contacts:     []string{"shengjun@chromium.org", "essential-inputs-team@google.com"},
		Attr:         []string{"group:mainline", "group:input-tools", "group:input-tools-upstream"},
		SoftwareDeps: []string{"chrome", "google_virtual_keyboard"},
		Pre:          chrome.LoggedIn(),
		Timeout:      5 * time.Minute,
		Params: []testing.Param{
			{
				Name: "us_en",
				Val:  ime.INPUTMETHOD_XKB_US_ENG,
			},
		},
	})
}

func PhysicalKeyboardInputFields(ctx context.Context, s *testing.State) {
	cr := s.PreValue().(*chrome.Chrome)

	cleanupCtx := ctx
	ctx, cancel := ctxutil.Shorten(ctx, 5*time.Second)
	defer cancel()

	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to connect Test API: ", err)
	}
	defer faillog.DumpUITreeOnError(cleanupCtx, s.OutDir(), s.HasError, tconn)

	// Add IME for testing.
	imeCode := ime.IMEPrefix + string(s.Param().(ime.InputMethodCode))

	s.Logf("Set current input method to: %s", imeCode)
	if err := ime.AddAndSetInputMethod(ctx, tconn, imeCode); err != nil {
		s.Fatalf("Failed to set input method to %s: %v: ", imeCode, err)
	}

	keyboard, err := input.Keyboard(ctx)
	if err != nil {
		s.Fatal("Failed to get keyboard: ", err)
	}
	defer keyboard.Close()

	its, err := testserver.Launch(ctx, cr, tconn)
	if err != nil {
		s.Fatal("Fail to launch inputs test server: ", err)
	}
	defer its.Close()

	type testData struct {
		inputField   string
		inputFunc    func(ctx context.Context) error
		expectedText string
	}

	var subtests []testData

	switch s.Param().(ime.InputMethodCode) {
	case ime.INPUTMETHOD_XKB_US_ENG:
		subtests = []testData{
			{
				inputField: testserver.TextAreaInputField,
				inputFunc: func(ctx context.Context) error {
					return keyboard.Type(ctx, `1234567890-=!@#$%^&*()_+[]{};'\:"|,./<>?~`)
				},
				expectedText: `1234567890-=!@#$%^&*()_+[]{};'\:"|,./<>?~`,
			}, {
				inputField: testserver.TextInputField,
				inputFunc: func(ctx context.Context) error {
					return keyboard.Type(ctx, "qwertyuiopasdfghjklzxcvbnmQWERTYUIOPASDFGHJKLZXCVBNM")
				},
				expectedText: "qwertyuiopasdfghjklzxcvbnmQWERTYUIOPASDFGHJKLZXCVBNM",
			},
		}
		break
	default:
		s.Fatalf("%s is not supported", imeCode)
	}

	for _, subtest := range subtests {
		s.Run(ctx, string(subtest.inputField), func(ctx context.Context, s *testing.State) {
			defer faillog.DumpUITreeOnError(ctx, s.OutDir(), s.HasError, tconn)
			inputField := subtest.inputField

			if err := its.ValidateInputOnField(inputField, subtest.inputFunc, subtest.expectedText)(ctx); err != nil {
				s.Fatalf("Failed to validate keys input in %s: %v", inputField, err)
			}
		})
	}
}
