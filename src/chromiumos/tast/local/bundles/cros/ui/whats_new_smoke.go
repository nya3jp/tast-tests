// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package ui

import (
	"context"
	"time"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/apps"
	"chromiumos/tast/local/bundles/cros/ui/faillog"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/ash"
	"chromiumos/tast/local/chrome/ui"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func: WhatsNewSmoke,
		Desc: "Checks that the What's New PWA can be opened",
		Contacts: []string{
			"bhansknecht@chromium.org",
			"kyleshima@chromium.org",
			"yulunwu@chromium.org",
		},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"chrome"},
		Pre:          chrome.LoggedIn(),
	})
}

// WhatsNewSmoke tests that we can open the What's New PWA from the Settings app entry point.
func WhatsNewSmoke(ctx context.Context, s *testing.State) {
	cr := s.PreValue().(*chrome.Chrome)
	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to create Test API connection: ", err)
	}
	defer faillog.DumpUITreeOnError(ctx, s, tconn)

	// Wait for What's New to be available in the list of all Chrome apps.
	// Without this step, sometimes What's New will launch as a Chrome window instead of a PWA.
	if err := testing.Poll(ctx, func(ctx context.Context) error {
		capps, err := ash.ChromeApps(ctx, tconn)
		if err != nil {
			testing.PollBreak(err)
		}
		for _, capp := range capps {
			if capp.AppID == apps.WhatsNew.ID {
				return nil
			}
		}
		return errors.New("App not yet found in available Chrome apps")
	}, nil); err != nil {
		s.Fatal("Unable to find What's New in the available Chrome apps: ", err)
	}

	// Launch the Settings app and wait for it to open
	if err := apps.Launch(ctx, tconn, apps.Settings.ID); err != nil {
		s.Fatal("Failed to launch the Settings app: ", err)
	}

	if err := ash.WaitForApp(ctx, tconn, apps.Settings.ID); err != nil {
		s.Fatal("Settings app did not appear in the shelf: ", err)
	}

	// Parameters of the UI elements we will click to launch What's New.
	// menuBtnParams - hamburger icon to expand the sidebar. Only appears in tablet mode or when zoomed in.
	// aboutParams - "About Chrome OS" link in the sidebar.
	// whatsNewParams - "See what's new" link in the about page.
	menuBtnParams := ui.FindParams{Role: ui.RoleTypeButton, Name: "Main menu"}
	aboutParams := ui.FindParams{Role: ui.RoleTypeLink, Name: "About Chrome OS"}
	whatsNewParams := ui.FindParams{Role: ui.RoleTypeLink, Name: "See what's new"}

	// Look for both the menu button and the "About Chrome OS" link.
	// If the menu button is found first, we'll have to click it to expand the sidebar.
	// If the "About Chrome OS" button is found first, there's no need to click the menu button.
	foundMenuButton := false
	if err := testing.Poll(ctx, func(ctx context.Context) error {
		found, err := ui.Exists(ctx, tconn, menuBtnParams)
		if err != nil {
			return testing.PollBreak(err)
		}
		if found {
			foundMenuButton = true
			return nil
		}

		found, err = ui.Exists(ctx, tconn, aboutParams)
		if err != nil {
			return testing.PollBreak(err)
		}
		if found {
			return nil
		}

		return errors.New("didn't find menu button or 'about chrome os' link")
	}, nil); err != nil {
		s.Fatal("Failed to find menu button or 'about chrome os' link in Settings app: ", err)
	}

	var toClick []ui.FindParams
	if foundMenuButton {
		s.Log("Found sidebar menu button; DUT is in tablet mode or display is zoomed in")
		toClick = append(toClick, menuBtnParams)
	}
	toClick = append(toClick, []ui.FindParams{aboutParams, whatsNewParams}...)

	for _, param := range toClick {
		n, err := ui.FindWithTimeout(ctx, tconn, param, 10*time.Second)
		if err != nil {
			s.Fatalf("Waiting to find %v node failed: %v", param.Name, err)
		}
		defer n.Release(ctx)

		// Use DoDefault instead of LeftClick, since the "See what's new" link
		// will sometimes move in between finding it and clicking it on certain
		// boards (banjo, guado, ultima, ...) where the "Powerwash for added security"
		// link appears in the About Chrome OS page. The test will periodically fail
		// in this case with LeftClick, since the click will go to the old location of
		// "See what's new" and be received by the powerwash link which displaces it.
		if err := n.DoDefault(ctx); err != nil {
			s.Fatalf("Failed to click the %v link: %v", param.Name, err)
		}
	}

	// Wait for What's New to open by checking in the shelf, and looking for something via UI
	if err := ash.WaitForApp(ctx, tconn, apps.WhatsNew.ID); err != nil {
		s.Fatal("What's New did not appear in the shelf: ", err)
	}

	// The large text at the top of the page seems like a natural choice since it's easily
	// recognizable and unlikely to change frequently. It would be better to have a
	// successful launch indicator that didn't rely on a string, though.
	// Particularly in this case, the apostrophe in What’s is not actually the normal
	// apostrophe character, but instead the "right single quotation mark" character (&rsquo;).
	titleParams := ui.FindParams{Role: ui.RoleTypeStaticText, Name: "What’s new with your Chromebook?"}
	if err := ui.WaitUntilExists(ctx, tconn, titleParams, 10*time.Second); err != nil {
		s.Fatal("Failed to find What's New PWA's title text in the UI: ", err)
	}
}
