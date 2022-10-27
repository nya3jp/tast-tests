// Copyright 2022 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package feedback

import (
	"context"
	"time"

	"chromiumos/tast/ctxutil"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/chrome/uiauto/faillog"
	"chromiumos/tast/local/chrome/uiauto/feedbackapp"
	"chromiumos/tast/local/chrome/uiauto/nodewith"
	"chromiumos/tast/local/chrome/uiauto/role"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         ChooseNotShareEmailWillDisableCheckbox,
		LacrosStatus: testing.LacrosVariantUnneeded,
		Desc:         "Verify the checkbox is disabled if user chooses not to share email",
		Contacts: []string{
			"wangdanny@google.com",
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

// ChooseNotShareEmailWillDisableCheckbox verifies if users choose not to share
// email, they won't be able to check the consent checkbox.
func ChooseNotShareEmailWillDisableCheckbox(ctx context.Context, s *testing.State) {
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

	// Launch feedback app and go to share data page.
	_, err = feedbackapp.LaunchAndGoToShareDataPage(ctx, tconn)
	if err != nil {
		s.Fatal("Failed to launch feedback app and navigating to share data page: ", err)
	}

	emailDropdown := nodewith.Name("Select email").Role(role.ListBox)
	dontIncludeEmailOption := nodewith.Name("anonymous user").Role(role.ListBoxOption)

	// Choose not to include email.
	if err := uiauto.Combine("choose not to include Email",
		ui.LeftClickUntil(emailDropdown, ui.WithTimeout(
			2*time.Second).WaitUntilExists(dontIncludeEmailOption)),
		ui.LeftClick(dontIncludeEmailOption),
	)(ctx); err != nil {
		s.Fatal("Failed to choose not include Email: ", err)
	}

	checkboxAncestor := nodewith.Name("Allow Google to email you about this issue").Role(
		role.GenericContainer)
	checkbox := nodewith.Role(role.CheckBox).Ancestor(checkboxAncestor)

	// Verify the user should not be able to check the consent checkbox.
	if err := testing.Poll(ctx, func(ctx context.Context) error {
		checkboxInfo, err := ui.Info(ctx, checkbox)
		if err != nil {
			return errors.Wrap(err, "failed to get checkbox info")
		}
		disabled := checkboxInfo.HTMLAttributes["aria-disabled"]
		if disabled != "true" {
			return errors.New("failed to stop polling because still loading")
		}
		return nil
	}, &testing.PollOptions{
		Interval: 2 * time.Second,
		Timeout:  10 * time.Second,
	}); err != nil {
		s.Fatal("Failed to find disabled checkbox: ", err)
	}
}
