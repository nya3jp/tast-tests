// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package ui

import (
	"context"
	"time"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/apps"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/ash"
	"chromiumos/tast/local/chrome/ui"
	"chromiumos/tast/local/chrome/ui/faillog"
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

// Params for Advanced Settings sub-headings for verification.
var advancedSubHeadings = []ui.FindParams{
	{
		Role: ui.RoleTypeHeading,
		Name: "Date and time",
	},
	{
		Role: ui.RoleTypeHeading,
		Name: "Privacy and security",
	},
	{
		Role: ui.RoleTypeHeading,
		Name: "Languages and input",
	},
	{
		Role: ui.RoleTypeHeading,
		Name: "Files",
	},
	{
		Role: ui.RoleTypeHeading,
		Name: "Printing",
	},
	{
		Role: ui.RoleTypeHeading,
		Name: "Accessibility",
	},
	{
		Role: ui.RoleTypeHeading,
		Name: "Reset settings",
	},
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

	// Launch the Settings app and wait for it to open
	if err := apps.Launch(ctx, tconn, apps.Settings.ID); err != nil {
		s.Fatal("Failed to launch the Settings app: ", err)
	}

	if err := ash.WaitForApp(ctx, tconn, apps.Settings.ID); err != nil {
		s.Fatal("Settings app did not appear in the shelf: ", err)
	}

	// Find the "Advanced" heading and associated button.
	advHeadingParams := ui.FindParams{
		Role: ui.RoleTypeHeading,
		Name: "Advanced",
	}
	advHeading, err := ui.FindWithTimeout(ctx, tconn, advHeadingParams, 5*time.Second)
	if err != nil {
		s.Fatal("Waiting to find Advanced heading failed: ", err)
	}
	defer advHeading.Release(ctx)

	advBtn, err := advHeading.DescendantWithTimeout(ctx, ui.FindParams{Role: ui.RoleTypeButton}, 5*time.Second)
	if err != nil {
		s.Fatal("Waiting to find Advanced button failed: ", err)
	}
	defer advBtn.Release(ctx)

	// Verify the initial state of the Settings menu (advanced subsections hidden)
	for _, heading := range advancedSubHeadings {
		if exists, err := ui.Exists(ctx, tconn, heading); err != nil {
			s.Fatal("Error in checking presence of subsection: ", err)
		} else if exists {
			s.Error("Subsection found unexpectedly: ", heading.Name)
		}
	}

	// Click the Advanced button to expand the section
	// We need to focus the button first so it will be clickable
	if err := advBtn.FocusAndWait(ctx, 5*time.Second); err != nil {
		s.Fatal("Failed to call focus() on the Advanced button: ", err)
	}

	if err := advBtn.LeftClick(ctx); err != nil {
		s.Fatal("Failed to click the Advanced button and open Advanced subsection: ", err)
	}

	// Check for the subsection headings
	for _, heading := range advancedSubHeadings {
		if err := ui.WaitUntilExists(ctx, tconn, heading, 5*time.Second); err != nil {
			s.Errorf("%v subsection heading not found: %v", heading.Name, err)
		}
	}

	// Hide the Advanced section by clicking it again, and verify the subsections are gone
	if err := advBtn.LeftClick(ctx); err != nil {
		s.Fatal("Failed to click and close the Advanced subsection button: ", err)
	}

	for _, heading := range advancedSubHeadings {
		if err := ui.WaitUntilGone(ctx, tconn, heading, 5*time.Second); err != nil {
			s.Errorf("%v subsection heading found, but it should not be present: %v", heading.Name, err)
		}
	}
}
