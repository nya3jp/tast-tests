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
	"chromiumos/tast/local/chrome/uiauto/faillog"
	"chromiumos/tast/local/chrome/uiauto/feedbackapp"
	"chromiumos/tast/local/chrome/uiauto/nodewith"
	"chromiumos/tast/local/chrome/uiauto/role"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         ViewMetrics,
		LacrosStatus: testing.LacrosVariantUnneeded,
		Desc:         "Verify user can click and view metrics",
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

// ViewMetrics verifies user can click and view metrics.
func ViewMetrics(ctx context.Context, s *testing.State) {
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
	feedbackRootNode, err := feedbackapp.LaunchAndGoToShareDataPage(ctx, tconn)
	if err != nil {
		s.Fatal("Failed to launch feedback app and go to share data page: ", err)
	}

	// Click metrics link.
	metrics := nodewith.Name("metrics").Role(role.Link).Ancestor(feedbackRootNode)
	if err := ui.DoDefault(metrics)(ctx); err != nil {
		s.Fatal("Failed to find and click metrics link: ", err)
	}

	// Verify user can view metrics.
	histograms := nodewith.Name("Histograms").First()
	if err := ui.WaitUntilExists(histograms)(ctx); err != nil {
		s.Error("Failed to view metrics: ", err)
	}
}
