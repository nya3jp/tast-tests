// Copyright 2020 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package apps

import (
	"context"
	"regexp"
	"time"

	"chromiumos/tast/ctxutil"
	"chromiumos/tast/local/bundles/cros/apps/fixture"
	"chromiumos/tast/local/bundles/cros/apps/helpapp"
	"chromiumos/tast/local/bundles/cros/apps/pre"
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/chrome/uiauto/faillog"
	"chromiumos/tast/local/chrome/uiauto/nodewith"
	"chromiumos/tast/local/chrome/uiauto/ossettings"
	"chromiumos/tast/local/chrome/uiauto/role"
	"chromiumos/tast/testing"
	"chromiumos/tast/testing/hwdep"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         LaunchReleaseNotesFromSettings,
		LacrosStatus: testing.LacrosVariantUnneeded,
		Desc:         "Help app release notes can be launched from Settings",
		Contacts: []string{
			"showoff-eng@google.com",
		},
		SoftwareDeps: []string{"chrome", "chrome_internal"},
		Fixture:      fixture.LoggedIn,
		Params: []testing.Param{
			{
				Name:              "stable",
				ExtraHardwareDeps: hwdep.D(pre.AppsStableModels),
				ExtraAttr:         []string{"group:mainline"},
			}, {
				Name:              "unstable",
				ExtraHardwareDeps: hwdep.D(pre.AppsUnstableModels),
				// b:238260020 - disable aged (>1y) unpromoted informational tests
				// ExtraAttr:         []string{"group:mainline", "informational"},
			},
		},
	})
}

// LaunchReleaseNotesFromSettings verifies launching Help app at the release notes page from ChromeOS settings.
func LaunchReleaseNotesFromSettings(ctx context.Context, s *testing.State) {
	tconn := s.FixtValue().(fixture.FixtData).TestAPIConn

	cleanupCtx := ctx
	ctx, cancel := ctxutil.Shorten(ctx, 5*time.Second)
	defer cancel()

	defer faillog.DumpUITreeOnError(cleanupCtx, s.OutDir(), s.HasError, tconn)

	ui := uiauto.New(tconn)

	settings, err := ossettings.LaunchAtPage(ctx, tconn, ossettings.AboutChromeOS)
	if err != nil {
		s.Fatal("Failed to launch Settings: ", err)
	}

	whatsNewTabFinder := nodewith.NameRegex(regexp.MustCompile("What('|’)s (n|N)ew")).Role(role.Heading).Ancestor(helpapp.RootFinder)

	if err := uiauto.Combine("launch WhatsNew and verify landing page",
		settings.LaunchWhatsNew(),
		ui.WaitUntilExists(whatsNewTabFinder),
	)(ctx); err != nil {
		s.Error(`Failed to launch WhatsNew or verify landing page: `, err)
	}
}
