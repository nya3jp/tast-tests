// Copyright 2022 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package feedback

import (
	"context"
	"time"

	"chromiumos/tast/ctxutil"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/browser"
	"chromiumos/tast/local/chrome/browser/browserfixt"
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/chrome/uiauto/faillog"
	"chromiumos/tast/local/chrome/uiauto/nodewith"
	"chromiumos/tast/local/chrome/uiauto/role"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         OpenFeedbackFromBrowser,
		LacrosStatus: testing.LacrosVariantExists,
		Desc:         "User is able to open feedback app from browser",
		Contacts: []string{
			"wangdanny@google.com",
			"zhangwenyu@google.com",
			"xiangdongkong@google.com",
			"cros-feedback-app@google.com",
		},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"chrome"},
		Params: []testing.Param{{
			Name:    "ash",
			Fixture: "chromeLoggedInWithOsFeedback",
			Val:     browser.TypeAsh,
		}, {
			Name:    "lacros",
			Fixture: "lacrosOsFeedback",
			Val:     browser.TypeLacros,
		}},
	})
}

const settingLinkAddress = "chrome://settings/help"

// OpenFeedbackFromBrowser verifies the user is able to open
// feedback app from browser.
func OpenFeedbackFromBrowser(ctx context.Context, s *testing.State) {
	cr := s.FixtValue().(chrome.HasChrome).Chrome()
	bt := s.Param().(browser.Type)

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

	conn, _, closeBrowser, err := browserfixt.SetUpWithURL(ctx, cr, bt, settingLinkAddress)
	if err != nil {
		s.Fatal("Failed to setup chrome: ", err)
	}
	defer closeBrowser(cleanupCtx)
	defer conn.Close()

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
