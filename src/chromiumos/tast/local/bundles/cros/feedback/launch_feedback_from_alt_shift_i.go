// Copyright 2022 The ChromiumOS Authors.
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
	"chromiumos/tast/local/chrome/uiauto/nodewith"
	"chromiumos/tast/local/chrome/uiauto/role"
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
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"chrome"},
	})
}

// LaunchFeedbackFromAltShiftI verifies launching feedback app from alt+shift+i.
func LaunchFeedbackFromAltShiftI(ctx context.Context, s *testing.State) {
	cleanupCtx := ctx
	ctx, cancel := ctxutil.Shorten(ctx, 5*time.Second)
	defer cancel()

	s.Log("Setting up chrome")
	cr, err := chrome.New(ctx, chrome.EnableFeatures("OsFeedback"))
	if err != nil {
		s.Fatal("Failed to start Chrome: ", err)
	}
	defer cr.Close(cleanupCtx)

	s.Log("Setting up test API")
	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to connect to Test API: ", err)
	}
	defer faillog.DumpUITreeOnError(cleanupCtx, s.OutDir(), s.HasError, tconn)

	ui := uiauto.New(tconn)

	s.Log("Setting up keyboard")
	kb, err := input.Keyboard(ctx)
	if err != nil {
		s.Fatal("Failed to find keyboard: ", err)
	}
	defer kb.Close()

	if err := testing.Poll(ctx, func(ctx context.Context) error {
		s.Log("Launching Feedback app with alt+shift+i")
		if err := kb.Accel(ctx, "Alt+Shift+I"); err != nil {
			return errors.Wrap(err, "failed pressing alt+shift+i")
		}

		s.Log("Verifying Feedback app is launched")
		if err = ash.WaitForApp(ctx, tconn, apps.Feedback.ID, 20*time.Second); err != nil {
			return errors.Wrap(err, "could not find app in shelf after launch")
		}

		return nil
	}, &testing.PollOptions{Timeout: time.Minute}); err != nil {
		s.Fatal("Failed launching Feedback app: ", err)
	}

	s.Log("Verifying issue description input exists")
	issueDescriptionInput := nodewith.Role(role.TextField)
	if err := ui.WithTimeout(20 * time.Second).WaitUntilExists(issueDescriptionInput)(
		ctx); err != nil {
		s.Error("Failed to find the issue description text input: ", err)
	}

	s.Log("Verifying continue button exists")
	button := nodewith.Name("Continue").Role(role.Button)
	if err := ui.WithTimeout(20 * time.Second).WaitUntilExists(button)(ctx); err != nil {
		s.Error("Failed to find continue button: ", err)
	}

	s.Log("Verifying five default help content links exist")
	helpLink := nodewith.Role(role.Link).Ancestor(nodewith.Role(role.Iframe))
	for i := 0; i < 5; i++ {
		item := helpLink.Nth(i)
		if err := ui.WithTimeout(20 * time.Second).WaitUntilExists(item)(ctx); err != nil {
			s.Error("Failed to find five help links: ", err)
		}
	}
}
