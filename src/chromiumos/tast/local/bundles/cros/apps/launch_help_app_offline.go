// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package apps

import (
	"context"
	"time"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/bundles/cros/apps/helpapp"
	"chromiumos/tast/local/bundles/cros/apps/pre"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/ui"
	"chromiumos/tast/local/chrome/ui/faillog"
	"chromiumos/tast/local/shill"
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
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"chrome"},
		Params: []testing.Param{
			{
				Name:              "stable",
				ExtraHardwareDeps: pre.AppsStableModels,
			}, {
				Name:              "unstable",
				ExtraHardwareDeps: pre.AppsUnstableModels,
			},
		},
	})
}

func LaunchHelpAppOffline(ctx context.Context, s *testing.State) {
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

	defer faillog.DumpUITreeOnError(ctx, s.OutDir(), s.HasError, tconn)

	offlineSteps := func() error {
		if err := helpapp.Launch(ctx, tconn); err != nil {
			return errors.Wrap(err, "failed to launch help app")
		}
		if err := helpapp.WaitForApp(ctx, tconn); err != nil {
			return errors.Wrap(err, "failed to wait for HelpApp")
		}

		// Verify only help card available when offline
		if isHelpCardOnly, err := isHelpCardShownOnly(ctx, tconn); err != nil {
			return errors.Wrap(err, "failed to verify help card shown only")
		} else if !isHelpCardOnly {
			return errors.New("other than Help cards are shown")
		}

		s.Log("Verify help article category available offline")
		// Expand Help article category by clicking Help tab.
		if err := helpapp.ClickTab(ctx, tconn, helpapp.HelpTab); err != nil {
			return errors.Wrap(err, "failed to click Help tab")
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
	}

	// Run test steps in offline mode.
	if err := shill.ExecFuncOnChromeOffline(ctx, offlineSteps); err != nil {
		s.Error("Failed to verify Help app running offline: ", err)
	}
}

// isHelpCardShownOnly checks if any help cards shown in Overview page.
func isHelpCardShownOnly(ctx context.Context, tconn *chrome.TestConn) (bool, error) {
	cardParams := ui.FindParams{
		Role:      "paragraph",
		ClassName: "category",
	}

	categoryNameParams := ui.FindParams{
		Role: ui.RoleTypeStaticText,
	}

	// Find all showoff-card nodes
	cards, err := helpapp.DescendantsWithTimeout(ctx, tconn, cardParams, 10*time.Second)
	if err != nil {
		return false, errors.Wrapf(err, "failed to get showoff-card with %v", cardParams)
	}
	defer cards.Release(ctx)

	// Get category name of each node
	for _, card := range cards {
		categoryNode, err := card.DescendantWithTimeout(ctx, categoryNameParams, 10*time.Second)
		if err != nil {
			return false, err
		}
		defer categoryNode.Release(ctx)
		if categoryNode.Name != "HELP" {
			return false, nil
		}
	}
	return true, nil
}
