// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package apps

import (
	"context"
	"time"

	"chromiumos/tast/ctxutil"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/bundles/cros/apps/helpapp"
	"chromiumos/tast/local/bundles/cros/apps/pre"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/ui"
	"chromiumos/tast/local/chrome/ui/faillog"
	"chromiumos/tast/local/network"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func: LaunchHelpAppOffline,
		Desc: "Help app can be launched offline with bundled content",
		Contacts: []string{
			"showoff-eng@google.com",
			"shengjun@chromium.org", // Test author
		},
		Attr:         []string{"group:mainline"},
		SoftwareDeps: []string{"chrome"},
		Params: []testing.Param{
			{
				Name:              "stable",
				ExtraHardwareDeps: pre.AppsStableModels,
			}, {
				Name:              "unstable",
				ExtraHardwareDeps: pre.AppsUnstableModels,
				ExtraAttr:         []string{"informational"},
			},
		},
	})
}

func LaunchHelpAppOffline(ctx context.Context, s *testing.State) {
	cleanupCtx := ctx
	// Use a shortened context for test operations to reserve time for cleanup.
	ctx, shortCancel := ctxutil.Shorten(ctx, 10*time.Second)
	defer shortCancel()

	// Must use new chrome instance to make sure help app never launched.
	cr, err := chrome.New(ctx)
	if err != nil {
		s.Fatal("Failed to start Chrome: ", err)
	}
	defer cr.Close(ctx)

	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to connect Test API: ", err)
	}

	defer faillog.DumpUITreeOnError(cleanupCtx, s.OutDir(), s.HasError, tconn)

	offlineSteps := func() error {
		if err := helpapp.Launch(ctx, tconn); err != nil {
			return errors.Wrap(err, "failed to launch help app")
		}
		if err := helpapp.WaitForApp(ctx, tconn); err != nil {
			return errors.Wrap(err, "failed to wait for HelpApp")
		}

		// Verify only help card available when offline.
		cardParams := ui.FindParams{
			Role:      "paragraph",
			ClassName: "category",
		}

		categoryNameParams := ui.FindParams{
			Role: ui.RoleTypeStaticText,
		}

		// Find all showoff-card nodes.
		cards, err := helpapp.DescendantsWithTimeout(ctx, tconn, cardParams, 10*time.Second)
		if err != nil {
			return errors.Wrapf(err, "failed to get showoff-card with %v", cardParams)
		}
		defer cards.Release(ctx)

		// Get category name of each node.
		for _, card := range cards {
			categoryNode, err := card.DescendantWithTimeout(ctx, categoryNameParams, 10*time.Second)
			if err != nil {
				return err
			}
			defer categoryNode.Release(ctx)
			if categoryNode.Name != "HELP" {
				return errors.Errorf("%s card shown in overview page when offline", categoryNode.Name)
			}
		}

		s.Log("Verify help article category available offline")

		// Clicking tab is not very reliable on rendering. Using Poll to stabilize the test.
		return testing.Poll(ctx, func(context.Context) error {
			// Expand Help article category by clicking Help tab.
			if err := helpapp.ClickTab(ctx, tconn, helpapp.HelpTab); err != nil {
				return testing.PollBreak(errors.Wrap(err, "failed to click Help tab"))
			}

			getStartedHelpCategory := ui.FindParams{
				Name: "Get started",
				Role: ui.RoleTypeTreeItem,
			}
			getStartedHelpCategoryNode, err := helpapp.DescendantWithTimeout(ctx, tconn, getStartedHelpCategory, 10*time.Second)
			if err != nil {
				return errors.Wrapf(err, "failed to find get started help category with %v", getStartedHelpCategory)
			}
			defer getStartedHelpCategoryNode.Release(ctx)

			return nil
		}, &testing.PollOptions{Timeout: 10 * time.Second, Interval: 1 * time.Second})
	}

	// Run test steps in offline mode.
	if err := network.ExecFuncOnChromeOffline(ctx, offlineSteps); err != nil {
		s.Error("Failed to verify Help app running offline: ", err)
	}
}
