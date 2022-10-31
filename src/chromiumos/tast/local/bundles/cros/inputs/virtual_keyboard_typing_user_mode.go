// Copyright 2021 The ChromiumOS Authors
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
	"chromiumos/tast/local/bundles/cros/inputs/fixture"
	"chromiumos/tast/local/bundles/cros/inputs/pre"
	"chromiumos/tast/local/bundles/cros/inputs/testserver"
	"chromiumos/tast/local/bundles/cros/inputs/util"
	"chromiumos/tast/local/chrome/ime"
	"chromiumos/tast/local/chrome/uiauto/faillog"
	"chromiumos/tast/local/chrome/uiauto/vkb"
	"chromiumos/tast/local/chrome/useractions"
	"chromiumos/tast/local/mountns"
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
		LacrosStatus: testing.LacrosVariantExists,
		Desc:         "Checks that virtual keyboard works in different user modes",
		Contacts:     []string{"essential-inputs-gardener-oncall@google.com", "essential-inputs-team@google.com"},
		Attr:         []string{"group:mainline", "group:input-tools"},
		SearchFlags:  util.IMESearchFlags(typingModeTestIMEs),
		SoftwareDeps: []string{"chrome", "google_virtual_keyboard"},
		HardwareDeps: hwdep.D(pre.InputsStableModels),
		Timeout:      time.Duration(len(typingModeTestIMEs)) * time.Duration(len(typingModeTestMessages)) * time.Minute,
		Params: []testing.Param{
			{
				Name:      "guest",
				ExtraAttr: []string{"group:input-tools-upstream"},
				Fixture:   fixture.AnyVKInGuest,
			},
			{
				Name:      "incognito",
				ExtraAttr: []string{"group:input-tools-upstream"},
				Fixture:   fixture.AnyVK,
			},
			{
				Name:              "guest_lacros",
				ExtraAttr:         []string{"informational"},
				ExtraSoftwareDeps: []string{"lacros_stable"},
				Fixture:           fixture.LacrosAnyVKInGuest,
			},
			{
				Name:              "incognito_lacros",
				ExtraAttr:         []string{"informational"},
				ExtraSoftwareDeps: []string{"lacros_stable"},
				Fixture:           fixture.LacrosAnyVK,
			},
		},
	})
}

func VirtualKeyboardTypingUserMode(ctx context.Context, s *testing.State) {
	// In order for the "guest_lacros" case to work correctly, we need to
	// run the test body in the user mount namespace. See b/244513681.
	if err := mountns.WithUserSessionMountNS(ctx, func(ctx context.Context) error {
		virtualKeyboardTypingUserMode(ctx, s)
		return nil
	}); err != nil {
		s.Fatal("Failed to run test in correct mount namespace: ", err)
	}
}

func virtualKeyboardTypingUserMode(ctx context.Context, s *testing.State) {
	cr := s.FixtValue().(fixture.FixtData).Chrome
	tconn := s.FixtValue().(fixture.FixtData).TestAPIConn
	uc := s.FixtValue().(fixture.FixtData).UserContext

	cleanupCtx := ctx
	ctx, cancel := ctxutil.Shorten(ctx, 5*time.Second)
	defer cancel()

	its, err := testserver.LaunchBrowserInMode(ctx, cr, tconn, s.FixtValue().(fixture.FixtData).BrowserType, strings.Contains(s.TestName(), "incognito"))
	if err != nil {
		s.Fatal("Failed to launch inputs test server: ", err)
	}
	defer its.CloseAll(cleanupCtx)

	if strings.Contains(s.TestName(), "incognito") {
		uc.SetAttribute(useractions.AttributeIncognitoMode, strconv.FormatBool(true))
	}

	inputField := testserver.TextAreaInputField
	vkbCtx := vkb.NewContext(cr, tconn)

	subtest := func(testName string, inputData data.InputData) func(ctx context.Context, s *testing.State) {
		return func(ctx context.Context, s *testing.State) {
			cleanupCtx := ctx
			// Use a shortened context for test operations to reserve time for cleanup.
			ctx, shortCancel := ctxutil.Shorten(ctx, 10*time.Second)
			defer shortCancel()

			defer func(ctx context.Context) {
				outDir := filepath.Join(s.OutDir(), testName)
				faillog.DumpUITreeWithScreenshotOnError(ctx, outDir, s.HasError, cr, "ui_tree_"+testName)

				if err := vkbCtx.HideVirtualKeyboard()(ctx); err != nil {
					s.Log("Failed to hide virtual keyboard: ", err)
				}
			}(cleanupCtx)

			if err := its.ValidateInputFieldForMode(uc, inputField, util.InputWithVK, inputData, s.DataPath)(ctx); err != nil {
				s.Fatal("Failed to validate virtual keyboard input: ", err)
			}
		}
	}

	// Run defined subtest per input method and message combination.
	util.RunSubtestsPerInputMethodAndMessage(ctx, uc, s, typingModeTestIMEs, typingModeTestMessages, subtest)
}
