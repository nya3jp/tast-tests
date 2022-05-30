// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package inputs

import (
	"context"
	"time"

	"go.chromium.org/chromiumos/tast/ctxutil"
	"go.chromium.org/chromiumos/tast-tests/local/bundles/cros/inputs/fixture"
	"go.chromium.org/chromiumos/tast-tests/local/bundles/cros/inputs/pre"
	"go.chromium.org/chromiumos/tast-tests/local/bundles/cros/inputs/testserver"
	"go.chromium.org/chromiumos/tast-tests/local/chrome/uiauto/faillog"
	"go.chromium.org/chromiumos/tast-tests/local/input"
	"go.chromium.org/chromiumos/tast/testing"
	"go.chromium.org/chromiumos/tast/testing/hwdep"
)

func init() {

	testing.AddTest(&testing.Test{
		Func:         PhysicalKeyboardEmojiSearch,
		LacrosStatus: testing.LacrosVariantExists,
		Desc:         "Check that emoji search works well",
		Contacts:     []string{"jopalmer@chromium.org", "essential-inputs-team@google.com"},
		Attr:         []string{"group:input-tools", "group:mainline"},
		SoftwareDeps: []string{"chrome"},
		HardwareDeps: hwdep.D(hwdep.Model(pre.StableModels...), hwdep.SkipOnModel("kodama", "kefka")),
		Params: []testing.Param{
			{
				Fixture:   fixture.ClamshellNonVK,
				ExtraAttr: []string{"group:input-tools-upstream"},
			},
			{
				Name:              "lacros",
				Fixture:           fixture.LacrosClamshellNonVK,
				ExtraSoftwareDeps: []string{"lacros"},
				ExtraAttr:         []string{"informational"},
			},
		},
	})
}

func PhysicalKeyboardEmojiSearch(ctx context.Context, s *testing.State) {
	cleanupCtx := ctx
	ctx, cancel := ctxutil.Shorten(ctx, 5*time.Second)
	defer cancel()

	cr := s.FixtValue().(fixture.FixtData).Chrome
	tconn := s.FixtValue().(fixture.FixtData).TestAPIConn
	uc := s.FixtValue().(fixture.FixtData).UserContext
	uc.SetTestName(s.TestName())

	defer faillog.DumpUITreeWithScreenshotOnError(cleanupCtx, s.OutDir(), s.HasError, cr, "ui_tree")

	keyboard, err := input.Keyboard(ctx)
	if err != nil {
		s.Fatal("Failed to access keyboard: ", err)
	}
	defer keyboard.Close()

	its, err := testserver.LaunchBrowser(ctx, s.FixtValue().(fixture.FixtData).BrowserType, cr, tconn)
	if err != nil {
		s.Fatal("Failed to launch inputs test server: ", err)
	}
	defer its.CloseAll(cleanupCtx)

	if err := its.InputEmojiWithEmojiPickerSearch(uc, testserver.TextAreaInputField, keyboard, "melting face", "🫠")(ctx); err != nil {
		s.Fatal("Failed to verify emoji picker: ", err)
	}
}
