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
		Func:         DisplayCheckboxAndPageURL,
		LacrosStatus: testing.LacrosVariantUnneeded,
		Desc:         "Verify url matches the current website where Feedback app is opened and checkbox is checked by default",
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

// DisplayCheckboxAndPageURL verifies the url matches the current website from where
// user opens the Feedback app and the checkbox should be checked by default.
func DisplayCheckboxAndPageURL(ctx context.Context, s *testing.State) {
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

	// Open chrome browser.
	if err := apps.Launch(ctx, tconn, apps.Chrome.ID); err != nil {
		s.Fatal("Failed to launch chrome app: ", err)
	}

	if err := ash.WaitForApp(ctx, tconn, apps.Chrome.ID, time.Minute); err != nil {
		s.Fatal("Chrome app did not appear in shelf after launch: ", err)
	}

	// Launch feedback app and go to share data page.
	feedbackRootNode, err := feedbackapp.LaunchAndGoToShareDataPage(ctx, tconn)
	if err != nil {
		s.Fatal("Failed to launch feedback app and go to share data page: ", err)
	}

	// Verify url text exists.
	urlText := nodewith.NameContaining("chrome://newtab/").Role(role.Link).Ancestor(feedbackRootNode)
	if err := ui.WaitUntilExists(urlText)(ctx); err != nil {
		s.Fatal("Failed to find element: ", err)
	}

	// Verify share url checkbox is checked by default.
	checkboxAncestor := nodewith.NameContaining("Share URL").Role(
		role.GenericContainer).Ancestor(feedbackRootNode)
	checkbox := nodewith.Role(role.CheckBox).Ancestor(checkboxAncestor)
	if err := ui.WaitUntilExists(checkbox.Attribute("checked", "true"))(ctx); err != nil {
		s.Fatal("Failed to find checked share url checkbox: ", err)
	}
}
