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

	// Establish a Chrome connection to the Settings app and wait for it to finish loading
	settingsConn, err := cr.NewConnForTarget(ctx, chrome.MatchTargetURL("chrome://os-settings/"))
	if err != nil {
		s.Fatal("Failed to get Chrome connection to Settings app: ", err)
	}
	defer settingsConn.Close()

	if err := settingsConn.WaitForExpr(ctx, `document.readyState === "complete"`); err != nil {
		s.Fatal("Failed waiting for Settings app document state to be ready: ", err)
	}

	// Launch What's New using the Settings page JS functions. The same JS is tied to the UI link's on-click property.
	if err := settingsConn.Eval(ctx,
		"settings.AboutPageBrowserProxyImpl.getInstance().launchReleaseNotes()",
		nil); err != nil {
		s.Fatal("Failed to run Javascript to launch What's New: ", err)
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
