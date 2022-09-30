// Copyright 2022 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package feedback

import (
	"context"
	"time"

	"chromiumos/tast/ctxutil"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/chrome/uiauto/browser"
	"chromiumos/tast/local/chrome/uiauto/faillog"
	"chromiumos/tast/local/chrome/uiauto/nodewith"
	"chromiumos/tast/local/chrome/uiauto/role"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         OpenFeedbackFromBrowser,
		LacrosStatus: testing.LacrosVariantUnneeded,
		Desc:         "User is able to open feedback app from browser",
		Contacts: []string{
			"wangdanny@google.com",
			"zhangwenyu@google.com",
			"xiangdongkong@google.com",
			"cros-feedback-app@google.com",
		},
		Fixture:      "chromeLoggedInWithOsFeedback",
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"chrome"},
	})
}

// OpenFeedbackFromBrowser verifies the user is able to open
// feedback app from browser.
func OpenFeedbackFromBrowser(ctx context.Context, s *testing.State) {
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

	ui := uiauto.New(tconn)

	if _, err := browser.Launch(ctx, tconn, cr, "chrome://settings/help"); err != nil {
		s.Fatal("Failed to open settings in chrome: ", err)
	}

	link := nodewith.Name("Report an issue").Role(role.Link)
	feedbackHeading := nodewith.Name("Send feedback").Role(role.Heading)

	// Open feedback app from browser.
	if err := uiauto.Combine("Open feedback app from browser",
		ui.DoDefault(link),
		ui.WaitUntilExists(feedbackHeading),
	)(ctx); err != nil {
		s.Fatal("Failed to open feedback app from browser: ", err)
	}
}
