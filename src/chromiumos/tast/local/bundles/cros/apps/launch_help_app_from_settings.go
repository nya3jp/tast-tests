// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package apps

import (
	"context"
	"time"

	"chromiumos/tast/ctxutil"
	"chromiumos/tast/local/bundles/cros/apps/helpapp"
	"chromiumos/tast/local/bundles/cros/apps/pre"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/chrome/uiauto/faillog"
	"chromiumos/tast/local/chrome/uiauto/ossettings"
	"chromiumos/tast/testing"
	"chromiumos/tast/testing/hwdep"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         LaunchHelpAppFromSettings,
		LacrosStatus: testing.LacrosVariantUnneeded,
		Desc:         "Help app can be launched from Settings",
		Contacts: []string{
			"carpenterr@chromium.org", // test author.
			"showoff-eng@google.com",
		},
		Attr:         []string{"group:mainline"},
		SoftwareDeps: []string{"chrome", "chrome_internal"},
		Fixture:      "chromeLoggedInForEA",
		Params: []testing.Param{
			{
				Name:              "stable",
				ExtraHardwareDeps: hwdep.D(pre.AppsStableModels),
			}, {
				Name:              "unstable",
				ExtraHardwareDeps: hwdep.D(pre.AppsUnstableModels),
				ExtraAttr:         []string{"informational"},
			},
		},
	})
}

// LaunchHelpAppFromSettings verifies launching Help app from ChromeOS settings.
func LaunchHelpAppFromSettings(ctx context.Context, s *testing.State) {
	cr := s.FixtValue().(*chrome.Chrome)

	cleanupCtx := ctx
	ctx, cancel := ctxutil.Shorten(ctx, 5*time.Second)
	defer cancel()

	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to connect Test API: ", err)
	}
	defer faillog.DumpUITreeOnError(cleanupCtx, s.OutDir(), s.HasError, tconn)

	settings, err := ossettings.LaunchAtPage(ctx, tconn, ossettings.AboutChromeOS)
	if err != nil {
		s.Fatal("Failed to launch Settings: ", err)
	}

	if err := uiauto.Combine(`launch help app on "about ChromeOS" page`,
		settings.LaunchHelpApp(),
		helpapp.NewContext(cr, tconn).WaitForApp(),
	)(ctx); err != nil {
		s.Fatal("Failed to launch help App: ", err)
	}
}
