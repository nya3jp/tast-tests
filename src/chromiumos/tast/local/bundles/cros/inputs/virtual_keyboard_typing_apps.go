// Copyright 2020 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package inputs

import (
	"context"
	"strings"
	"time"

	"chromiumos/tast/local/apps"
	"chromiumos/tast/local/bundles/cros/inputs/fixture"
	"chromiumos/tast/local/bundles/cros/inputs/pre"
	"chromiumos/tast/local/bundles/cros/inputs/util"
	"chromiumos/tast/local/chrome/ash"
	"chromiumos/tast/local/chrome/ime"
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/chrome/uiauto/faillog"
	"chromiumos/tast/local/chrome/uiauto/nodewith"
	"chromiumos/tast/local/chrome/uiauto/role"
	"chromiumos/tast/local/chrome/uiauto/vkb"
	"chromiumos/tast/local/chrome/useractions"
	"chromiumos/tast/testing"
	"chromiumos/tast/testing/hwdep"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         VirtualKeyboardTypingApps,
		LacrosStatus: testing.LacrosVariantUnneeded,
		Desc:         "Checks that the virtual keyboard works in apps",
		Contacts:     []string{"essential-inputs-gardener-oncall@google.com", "essential-inputs-team@google.com"},
		Attr:         []string{"group:mainline", "group:input-tools"},
		SoftwareDeps: []string{"chrome", "google_virtual_keyboard"},
		Fixture:      fixture.TabletVK,
		SearchFlags: []*testing.StringPair{
			{
				Key:   "ime",
				Value: ime.EnglishUS.Name,
			},
		},
		Timeout: 5 * time.Minute,
		Params: []testing.Param{
			{
				ExtraHardwareDeps: hwdep.D(pre.InputsStableModels),
				ExtraAttr:         []string{"group:input-tools-upstream"},
			},
			{
				Name:              "informational",
				ExtraHardwareDeps: hwdep.D(pre.InputsUnstableModels),
				ExtraAttr:         []string{"informational"},
			},
		},
	})
}

func VirtualKeyboardTypingApps(ctx context.Context, s *testing.State) {
	// typingKeys indicates a key series that tapped on virtual keyboard.
	// Input string should start with lower case letter because VK layout is not auto-capitalized in the settings search bar.
	const typingKeys = "language"

	cr := s.FixtValue().(fixture.FixtData).Chrome
	tconn := s.FixtValue().(fixture.FixtData).TestAPIConn
	uc := s.FixtValue().(fixture.FixtData).UserContext

	defer faillog.DumpUITreeOnError(ctx, s.OutDir(), s.HasError, tconn)

	app := apps.Settings
	s.Logf("Launching %s", app.Name)
	if err := apps.Launch(ctx, tconn, app.ID); err != nil {
		s.Fatalf("Failed to launch %s: %c", app.Name, err)
	}
	if err := ash.WaitForApp(ctx, tconn, app.ID, time.Minute); err != nil {
		s.Fatalf("%s did not appear in shelf after launch: %v", app.Name, err)
	}

	vkbCtx := vkb.NewContext(cr, tconn)
	searchFieldFinder := nodewith.Role(role.SearchBox).Name("Search settings")

	validateAction := uiauto.Combine("test virtual keyboard input in settings app",
		vkbCtx.ClickUntilVKShown(searchFieldFinder),
		vkbCtx.WaitForDecoderEnabled(true),
		vkbCtx.TapKeysIgnoringCase(strings.Split(typingKeys, "")),
		// Hide virtual keyboard to submit candidate.
		vkbCtx.HideVirtualKeyboard(),
		// Validate text.
		util.WaitForFieldTextToBeIgnoringCase(tconn, searchFieldFinder, typingKeys),
	)

	if err := uiauto.UserAction("VK typing input",
		validateAction,
		uc,
		&useractions.UserActionCfg{
			Attributes: map[string]string{
				useractions.AttributeInputField: "OS setting search field",
				useractions.AttributeFeature:    useractions.FeatureVKTyping,
			},
		},
	)(ctx); err != nil {
		s.Fatal("Failed to verify virtual keyboard input in settings: ", err)
	}
}
