// Copyright 2022 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package personalization

import (
	"context"
	"time"

	"chromiumos/tast/ctxutil"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/chrome/uiauto/faillog"
	"chromiumos/tast/local/chrome/uiauto/nodewith"
	"chromiumos/tast/local/chrome/uiauto/quicksettings"
	"chromiumos/tast/local/chrome/uiauto/role"
	"chromiumos/tast/local/personalization"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         OpenPersonalizationHubFromQuickSettings,
		LacrosStatus: testing.LacrosVariantUnneeded,
		Desc:         "Test opening personalization hub app from Dark theme Quick Settings",
		Contacts: []string{
			"thuongphan@google.com",
			"chromeos-sw-engprod@google.com",
			"assistive-eng@google.com",
		},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"chrome"},
		Timeout:      5 * time.Minute,
		Fixture:      "chromeLoggedIn",
	})
}

func OpenPersonalizationHubFromQuickSettings(ctx context.Context, s *testing.State) {
	cr := s.FixtValue().(*chrome.Chrome)

	cleanupCtx := ctx
	ctx, cancel := ctxutil.Shorten(ctx, 10*time.Second)
	defer cancel()

	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to create Test API connection: ", err)
	}
	defer faillog.DumpUITreeOnError(cleanupCtx, s.OutDir(), s.HasError, tconn)

	// The test has a dependency of network speed, so we give uiauto.Context ample
	// time to wait for nodes to load.
	ui := uiauto.New(tconn).WithTimeout(30 * time.Second)

	if err := openPersonalizationHub(ctx, tconn, ui); err != nil {
		s.Fatal("Failed to open Personalization Hub from Quick Settings: ", err)
	}

	if err := ui.WaitUntilExists(personalization.PersonalizationHubWindow)(ctx); err != nil {
		s.Fatal("Failed to validate Personalization Hub open: ", err)
	}
}

func openPersonalizationHub(ctx context.Context, tconn *chrome.TestConn, ui *uiauto.Context) error {
	if err := quicksettings.Expand(ctx, tconn); err != nil {
		return errors.Wrap(err, "failed to expand quick settings")
	}

	darkThemePodLabelButton := quicksettings.PodLabelButton(quicksettings.SettingPodDarkThemeSettings)
	if err := ui.WaitUntilExists(darkThemePodLabelButton)(ctx); err != nil {
		return errors.Wrap(err, "dark theme pod label button is not found")
	}

	pageIndicators := nodewith.Role(role.Button).ClassName("PageIndicatorView")
	pages, err := ui.NodesInfo(ctx, pageIndicators)
	if err != nil {
		return errors.Wrap(err, "failed to get page indicator")
	}

	// If there is no page indicator (which means only one page of pod icons in Quick Settings),
	// try to click on Dark theme pod label button.
	if len(pages) == 0 {
		if err := ui.LeftClick(darkThemePodLabelButton)(ctx); err != nil {
			return errors.Wrap(err, "failed to open Dark theme settings")
		}
		return nil
	}

	// Although Dark theme pod label button is available in Quick Settings, we don't know the
	// exact page it resides. If we click on the Dark theme label button in a wrong page, it
	// would close Quick Settings bubble. Hence, we need to reopen the bubble in case it closes
	// and try to click on the Dark theme label button in all the pages.
	// TODO: update the tast test when Quick Settings adds new infrastructure to not close
	// the bubble accidentally.
	for _, page := range pages {
		if shown, err := quicksettings.Shown(ctx, tconn); err != nil {
			return errors.Wrap(err, "failed to check quick settings visibility status")
		} else if !shown {
			if err := quicksettings.Expand(ctx, tconn); err != nil {
				return errors.Wrap(err, "failed to expand quick settings")
			}
		}
		if err := uiauto.Combine("Open Personalization Hub",
			ui.LeftClick(nodewith.Role(page.Role).ClassName(page.ClassName).Name(page.Name)),
			ui.LeftClick(darkThemePodLabelButton),
		)(ctx); err != nil {
			return errors.Wrap(err, "failed to open Dark theme settings")
		}
	}
	return nil
}
