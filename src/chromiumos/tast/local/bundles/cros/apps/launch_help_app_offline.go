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

	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to connect Test API: ", err)
	}

	offlineSteps := func(offlineContext context.Context) error {
		if err := helpapp.Launch(offlineContext, tconn); err != nil {
			return errors.Wrap(err, "failed to launch help app")
		}

		if err := helpapp.WaitForApp(offlineContext, tconn); err != nil {
			return errors.Wrap(err, "failed to wait for HelpApp")
		}

		// Verify overview card rendered offline.
		cardParams := ui.FindParams{
			Role:      "paragraph",
			ClassName: "category",
		}

		if _, err := helpapp.DescendantWithTimeout(offlineContext, tconn, cardParams, 10*time.Second); err != nil {
			return errors.Wrap(err, "failed to render overview card")
		}

		// Expand Help article category by clicking Help tab.
		if err := helpapp.ClickTab(offlineContext, tconn, helpapp.HelpTab); err != nil {
			return errors.Wrap(err, "failed to click Help tab")
		}

		// Verify help article category available offline.
		getStartedHelpCategory := ui.FindParams{
			Name:      "Get started",
			Role:      ui.RoleTypeLink,
			ClassName: "heading",
		}

		getStartedHelpCategoryNode, err := helpapp.DescendantWithTimeout(offlineContext, tconn, getStartedHelpCategory, 10*time.Second)
		if err != nil {
			return errors.Wrapf(err, "failed to find get started help category with %v", getStartedHelpCategory)
		}
		defer getStartedHelpCategoryNode.Release(offlineContext)

		return nil
	}

	// Run test steps in offline mode.
	shill.ExecFuncOffline(ctx, offlineSteps)
}
