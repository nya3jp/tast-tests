// Copyright 2022 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package feedback

import (
	"context"
	"time"

	"chromiumos/tast/ctxutil"
	"chromiumos/tast/local/apps"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/ash"
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/chrome/uiauto/faillog"
	"chromiumos/tast/local/chrome/uiauto/feedbackapp"
	"chromiumos/tast/local/chrome/uiauto/nodewith"
	"chromiumos/tast/local/chrome/uiauto/role"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         LaunchFeedbackFromAboutChromeOS,
		LacrosStatus: testing.LacrosVariantUnneeded,
		Desc:         "Feedback app can be launched via About ChromeOS -> Send feedback",
		Contacts: []string{
			"zhangwenyu@google.com",
			"xiangdongkong@google.com",
			"cros-feedback-app@google.com",
		},
		Fixture:      "chromeLoggedInWithOsFeedback",
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"chrome"},
		Timeout:      2 * time.Minute,
	})
}

// LaunchFeedbackFromAboutChromeOS verifies launching Feedback app via Send feedback
// from About ChromeOS in the OS Settings.
func LaunchFeedbackFromAboutChromeOS(ctx context.Context, s *testing.State) {
	cr := s.FixtValue().(*chrome.Chrome)

	cleanupCtx := ctx
	ctx, cancel := ctxutil.Shorten(ctx, 5*time.Second)
	defer cancel()

	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to connect to Test API: ", err)
	}
	defer faillog.DumpUITreeWithScreenshotOnError(cleanupCtx, s.OutDir(), s.HasError, cr,
		"ui_dump")

	ui := uiauto.New(tconn).WithTimeout(20 * time.Second)

	// Open OS Settings app.
	if err := apps.Launch(ctx, tconn, apps.Settings.ID); err != nil {
		s.Fatal("Failed to launch Settings app: ", err)
	}

	if err := ash.WaitForApp(ctx, tconn, apps.Settings.ID, time.Minute); err != nil {
		s.Fatal("Settings app did not appear in shelf after launch: ", err)
	}

	// Handle narrow screen. Click menu button if it exists.
	menuButton := nodewith.Name("Main menu").Role(role.Button)
	defaultPolling := testing.PollOptions{Interval: time.Second, Timeout: 20 * time.Second}
	if err := uiauto.IfSuccessThen(ui.WaitUntilExists(menuButton),
		ui.WithPollOpts(defaultPolling).LeftClick(menuButton))(ctx); err != nil {
		s.Fatal("Failed to click menu button: ", err)
	}

	// Click About ChromeOS tab.
	aboutCrOSTab := nodewith.NameContaining("About ChromeOS").Role(role.StaticText)
	if err := ui.DoDefault(aboutCrOSTab)(ctx); err != nil {
		s.Fatal("Failed to click About ChromeOS tab: ", err)
	}

	// Click Send feedback button.
	feedbackButton := nodewith.NameContaining("Send feedback").Role(role.Link)
	if err := ui.DoDefault(feedbackButton)(ctx); err != nil {
		s.Fatal("Failed to click Send feedback button: ", err)
	}

	if err := feedbackapp.VerifyFeedbackAppIsLaunched(ctx, tconn, ui); err != nil {
		s.Fatal("Failed to verify that the Feedback app is launched: ", err)
	}
}
