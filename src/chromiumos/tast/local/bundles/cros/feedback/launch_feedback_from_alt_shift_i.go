// Copyright 2022 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package feedback

import (
	"context"
	"time"

	"chromiumos/tast/ctxutil"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/apps"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/ash"
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/chrome/uiauto/faillog"
	"chromiumos/tast/local/chrome/uiauto/feedbackapp"
	"chromiumos/tast/local/input"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         LaunchFeedbackFromAltShiftI,
		LacrosStatus: testing.LacrosVariantUnneeded,
		Desc:         "Feedback app can be launched from alt+shift+i",
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

// LaunchFeedbackFromAltShiftI verifies launching feedback app from alt+shift+i.
func LaunchFeedbackFromAltShiftI(ctx context.Context, s *testing.State) {
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

	// Set up keyboard.
	kb, err := input.Keyboard(ctx)
	if err != nil {
		s.Fatal("Failed to find keyboard: ", err)
	}
	defer kb.Close()

	if err := testing.Poll(ctx, func(ctx context.Context) error {
		// Launch Feedback app with alt+shift+i.
		if err := kb.Accel(ctx, "Alt+Shift+I"); err != nil {
			return errors.Wrap(err, "failed pressing alt+shift+i")
		}

		// Verify Feedback app is launched.
		if err = ash.WaitForApp(ctx, tconn, apps.Feedback.ID, 20*time.Second); err != nil {
			return errors.Wrap(err, "could not find app in shelf after launch")
		}

		return nil
	}, &testing.PollOptions{Timeout: time.Minute}); err != nil {
		s.Fatal("Failed launching Feedback app: ", err)
	}

	if err := feedbackapp.VerifyFeedbackAppIsLaunched(ctx, tconn, ui); err != nil {
		s.Fatal("Failed to verify that the Feedback app is launched: ", err)
	}
}
