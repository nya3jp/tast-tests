// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package inputs

import (
	"context"
	"strings"
	"time"

	"chromiumos/tast/ctxutil"
	"chromiumos/tast/local/bundles/cros/inputs/fixture"
	"chromiumos/tast/local/bundles/cros/inputs/pre"
	"chromiumos/tast/local/bundles/cros/inputs/testserver"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/uiauto/faillog"
	"chromiumos/tast/local/chrome/useractions"
	"chromiumos/tast/local/input"
	"chromiumos/tast/testing"
	"chromiumos/tast/testing/hwdep"
)

func init() {

	testing.AddTest(&testing.Test{
		Func:         PhysicalKeyboardEmojiSearch,
		LacrosStatus: testing.LacrosVariantUnknown,
		Desc:         "Check that emoji search works well",
		Contacts:     []string{"jopalmer@chromium.org", "essential-inputs-team@google.com"},
		Attr:         []string{"group:input-tools", "group:mainline"},
		SoftwareDeps: []string{"chrome"},
		HardwareDeps: hwdep.D(hwdep.Model(pre.StableModels...), hwdep.SkipOnModel("kodama", "kefka")),
		Params: []testing.Param{
			{
				Pre:       pre.NonVKClamshell,
				ExtraAttr: []string{"group:input-tools-upstream"},
			},
			{
				Name:      "fixture",
				Fixture:   fixture.ClamshellNonVK,
				ExtraAttr: []string{"informational"},
			},
		},
	})
}

func PhysicalKeyboardEmojiSearch(ctx context.Context, s *testing.State) {
	cleanupCtx := ctx
	ctx, cancel := ctxutil.Shorten(ctx, 5*time.Second)
	defer cancel()

	var cr *chrome.Chrome
	var tconn *chrome.TestConn
	var uc *useractions.UserContext
	if strings.Contains(s.TestName(), "fixture") {
		cr = s.FixtValue().(fixture.FixtData).Chrome
		tconn = s.FixtValue().(fixture.FixtData).TestAPIConn
		uc = s.FixtValue().(fixture.FixtData).UserContext
		uc.SetTestName(s.TestName())
	} else {
		cr = s.PreValue().(pre.PreData).Chrome
		tconn = s.PreValue().(pre.PreData).TestAPIConn
		uc = s.PreValue().(pre.PreData).UserContext
	}

	defer faillog.DumpUITreeWithScreenshotOnError(cleanupCtx, s.OutDir(), s.HasError, cr, "ui_tree")

	keyboard, err := input.Keyboard(ctx)
	if err != nil {
		s.Fatal("Failed to access keyboard: ", err)
	}
	defer keyboard.Close()

	its, err := testserver.Launch(ctx, cr, tconn)
	if err != nil {
		s.Fatal("Failed to launch inputs test server: ", err)
	}
	defer its.Close()

	if err := its.InputEmojiWithEmojiPickerSearch(uc, testserver.TextAreaInputField, keyboard, "melting face", "🫠")(ctx); err != nil {
		s.Fatal("Failed to verify emoji picker: ", err)
	}
}
