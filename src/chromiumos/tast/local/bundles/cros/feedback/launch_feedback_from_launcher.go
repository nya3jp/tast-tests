// Copyright 2022 The ChromiumOS Authors.
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
	"chromiumos/tast/local/chrome/uiauto/faillog"
	"chromiumos/tast/local/chrome/uiauto/launcher"
	"chromiumos/tast/local/input"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         LaunchFeedbackFromLauncher,
		LacrosStatus: testing.LacrosVariantUnneeded,
		Desc:         "Feedback app can be found and launched from the launcher",
		Contacts: []string{
			"zhangwenyu@google.com",
			"xiangdongkong@google.com",
			"cros-peripherals@google.com",
		},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"chrome"},
	})
}

// LaunchFeedbackFromLauncher verifies launching Feedback app from the launcher.
func LaunchFeedbackFromLauncher(ctx context.Context, s *testing.State) {
	s.Log("Set up chrome")
	cr, err := chrome.New(ctx, chrome.EnableFeatures("OsFeedback"))
	if err != nil {
		s.Fatal("Failed to start Chrome: ", err)
	}
	defer cr.Close(ctx)

	cleanupCtx := ctx
	ctx, cancel := ctxutil.Shorten(ctx, 5*time.Second)
	defer cancel()

	s.Log("Set up test API")
	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to connect Test API: ", err)
	}
	defer faillog.DumpUITreeOnError(cleanupCtx, s.OutDir(), s.HasError, tconn)

	s.Log("Set up keyboard")
	kb, err := input.Keyboard(ctx)
	if err != nil {
		s.Fatal("Failed to find keyboard: ", err)
	}
	defer kb.Close()

	s.Log("Launch Feedback app from launcher")
	if err := launcher.SearchAndLaunchWithQuery(
		tconn, kb, "feedback", apps.Feedback.Name)(ctx); err != nil {
		s.Fatal("Failed to search and launch app: ", err)
	}

	s.Log("Verify Feedback app is launched")
	err = ash.WaitForApp(ctx, tconn, apps.Feedback.ID, time.Minute)
	if err != nil {
		s.Fatal("Could not find app in shelf after launch: ", err)
	}
}
