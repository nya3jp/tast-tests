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
	"chromiumos/tast/local/chrome/browser"
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/chrome/uiauto/faillog"
	"chromiumos/tast/local/chrome/uiauto/feedbackapp"
	"chromiumos/tast/local/chrome/uiauto/nodewith"
	"chromiumos/tast/local/chrome/uiauto/role"
	"chromiumos/tast/testing"
)

// testParam contains all the data needed to run a single test iteration.
type testParam struct {
	linkName    string
	linkAddress string
}

func init() {
	testing.AddTest(&testing.Test{
		Func:         ViewPolicy,
		LacrosStatus: testing.LacrosVariantUnneeded,
		Desc:         "User is able to view policy, legal help and terms of service",
		Contacts: []string{
			"zhangwenyu@google.com",
			"xiangdongkong@google.com",
			"cros-feedback-app@google.com",
		},
		Fixture:      "chromeLoggedInWithOsFeedback",
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"chrome"},
		Timeout:      3 * time.Minute,
		Params: []testing.Param{{
			Name: "legal_help_page",
			Val: testParam{
				linkName:    "Legal Help page",
				linkAddress: "https://support.google.com/legal/answer/3110420",
			},
		}, {
			Name: "privacy_policy",
			Val: testParam{
				linkName:    "Privacy Policy",
				linkAddress: "https://policies.google.com/privacy",
			},
		}, {
			Name: "terms_of_service",
			Val: testParam{
				linkName:    "Terms of Service",
				linkAddress: "https://policies.google.com/terms",
			},
		}},
	})
}

// ViewPolicy verifies the user is able to view policy, legal help and terms of service.
func ViewPolicy(ctx context.Context, s *testing.State) {
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

	// Launch feedback app and navigate to share data page.
	feedbackRootNode, err := feedbackapp.LaunchAndGoToShareDataPage(ctx, tconn)
	if err != nil {
		s.Fatal("Failed to launch feedback app and go to share data page: ", err)
	}

	// Find link and click.
	link := nodewith.NameContaining(s.Param().(testParam).linkName).Role(
		role.Link).Ancestor(feedbackRootNode)
	if err := ui.DoDefault(link)(ctx); err != nil {
		s.Fatal("Failed to find link: ", err)
	}

	// Verify browser is opened.
	if err = ash.WaitForApp(ctx, tconn, apps.Chrome.ID, time.Minute); err != nil {
		s.Fatal("Could not find browser in shelf after launch: ", err)
	}

	// Verify browser contains correct address.
	if err := testing.Poll(ctx, func(ctx context.Context) error {
		tabs, err := browser.CurrentTabs(ctx, tconn)
		if err != nil {
			return errors.Wrap(err, "failed to get the current tabs")
		}
		if tabs[0].URL != s.Param().(testParam).linkAddress {
			return errors.Wrap(err, "failed to get correct url address")
		}
		return nil
	}, &testing.PollOptions{
		Interval: 5 * time.Second,
		Timeout:  30 * time.Second,
	}); err != nil {
		s.Fatal("Failed to find link address: ", err)
	}
}
