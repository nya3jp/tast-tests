// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package ui

import (
	"context"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/apps"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/ash"
	"chromiumos/tast/local/chrome/ui/faillog"
	"chromiumos/tast/local/chrome/ui/settingsapp"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func: SettingsToggleAdvanced,
		Desc: "Checks that the Advanced section of Settings can be expanded and collapsed",
		Contacts: []string{
			"bhansknecht@chromium.org",
			"kyleshima@chromium.org",
		},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"chrome"},
		Pre:          chrome.LoggedIn(),
	})
}

// SettingsToggleAdvanced tests that we can toggle the Advanced Settings section.
func SettingsToggleAdvanced(ctx context.Context, s *testing.State) {
	cr := s.PreValue().(*chrome.Chrome)
	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to create Test API connection: ", err)
	}
	defer faillog.DumpUITreeOnError(ctx, s.OutDir(), s.HasError, tconn)

	if err := testing.Poll(ctx, func(ctx context.Context) error {
		capps, err := ash.ChromeApps(ctx, tconn)
		if err != nil {
			testing.PollBreak(err)
		}
		for _, app := range capps {
			if app.AppID == apps.Settings.ID {
				return nil
			}
		}
		return errors.New("Settings app not yet found in available Chrome apps")
	}, nil); err != nil {
		s.Fatal("Unable to find the Settings app in the available Chrome apps: ", err)
	}

	// Launch the Settings app.
	settingsApp, err := settingsapp.Launch(ctx, tconn, cr, true)
	if err != nil {
		s.Fatal("Failed to launch Settings app: ", err)
	}
	defer settingsApp.Close(ctx)

	// Verify the initial state of the Settings menu (advanced subsections hidden).
	if shown, err := settingsApp.WaitForAdvancedSectionHidden(ctx); err != nil {
		s.Fatalf("Advanced settings are unexpectedly shown after opening Settings app: %v, shown apps: %v", err, shown)
	}

	// Click the Advanced button to expand the section.
	if err := settingsApp.ToggleAdvanced(ctx); err != nil {
		s.Fatal("Failed expanding advanced settings: ", err)
	}

	// The Advanced settings section should now be expanded.
	if notShown, err := settingsApp.WaitForAdvancedSectionShown(ctx); err != nil {
		if len(notShown) == len(settingsapp.AdvancedSubHeadings) {
			s.Fatal("No Advanced settings present: ", err)
		}
		s.Errorf("Not all Advanced settings were displayed: %v, missing sections: %v", err, notShown)
	}

	// Collapse the Advanced section and verify the subsections are gone.
	if err := settingsApp.ToggleAdvanced(ctx); err != nil {
		s.Fatal("Failed collapsing advanced settings: ", err)
	}

	// Verify that the Advanced subsections are gone.
	if shown, err := settingsApp.WaitForAdvancedSectionHidden(ctx); err != nil {
		s.Fatalf("Advanced settings are unexpectedly shown after collapsing the section: %v, shown apps: %v", err, shown)
	}
}
