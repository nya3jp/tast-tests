// Copyright 2022 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package quicksettings

import (
	"context"

	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/settings"
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/chrome/uiauto/faillog"
	"chromiumos/tast/local/chrome/uiauto/quicksettings"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         NightLight,
		LacrosStatus: testing.LacrosVariantUnneeded,
		Desc:         "Checks that the Quick Settings Night Light feature pod button is working correctly",
		Contacts: []string{
			"chromeos-sw-engprod@google.com",
			"leandre@chromium.org",
			"cros-status-area-eng@google.com",
		},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"chrome"},
		Pre:          chrome.LoggedIn(),
	})
}

// NightLight tests that Night Light feature pod button is working correctly.
func NightLight(ctx context.Context, s *testing.State) {
	cr := s.PreValue().(*chrome.Chrome)
	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to create Test API connection: ", err)
	}

	ui := uiauto.New(tconn)

	// Get night light state at the beginning.
	state1, err := settings.NightLightEnabled(ctx, tconn)
	if err != nil {
		s.Fatal("Failed to get Night Light state: ", err)
	}

	if err := quicksettings.Show(ctx, tconn); err != nil {
		s.Fatal("Failed to show Quick Settings: ", err)
	}
	defer quicksettings.Hide(ctx, tconn)
	defer faillog.DumpUITreeOnError(ctx, s.OutDir(), s.HasError, tconn)

	if err := ui.LeftClick(quicksettings.PodIconButton(quicksettings.SettingPodNightLight))(ctx); err != nil {
		s.Fatal("Failed to click the Night Light feature pod icon button: ", err)
	}

	// Get night light state after the first toggle.
	state2, err := settings.NightLightEnabled(ctx, tconn)
	if err != nil {
		s.Fatal("Failed to get Night Light state: ", err)
	}

	if state1 == state2 {
		s.Fatal("Night Light state did not change after toggling the feature pod button")
	}

	if err := ui.LeftClick(quicksettings.PodIconButton(quicksettings.SettingPodNightLight))(ctx); err != nil {
		s.Fatal("Failed to click the Night Light feature pod icon button: ", err)
	}

	// Get night light state after the second toggle.
	state3, err := settings.NightLightEnabled(ctx, tconn)
	if err != nil {
		s.Fatal("Failed to get Night Light state: ", err)
	}

	if state1 != state3 {
		s.Fatal("Night Light state did not change back to the beginning state after toggling twice")
	}
}
