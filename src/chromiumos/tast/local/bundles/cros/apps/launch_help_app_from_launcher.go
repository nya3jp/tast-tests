// Copyright 2020 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package apps

import (
	"context"
	"time"

	"chromiumos/tast/ctxutil"
	"chromiumos/tast/local/apps"
	"chromiumos/tast/local/bundles/cros/apps/fixture"
	"chromiumos/tast/local/bundles/cros/apps/helpapp"
	"chromiumos/tast/local/bundles/cros/apps/pre"
	"chromiumos/tast/local/chrome/uiauto/faillog"
	"chromiumos/tast/local/chrome/uiauto/launcher"
	"chromiumos/tast/local/input"
	"chromiumos/tast/testing"
	"chromiumos/tast/testing/hwdep"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         LaunchHelpAppFromLauncher,
		LacrosStatus: testing.LacrosVariantUnneeded,
		Desc:         "Help app can be found and launched from the launcher",
		Contacts: []string{
			"carpenterr@chromium.org", // test author.
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

// LaunchHelpAppFromLauncher verifies launching Help app from the launcher.
func LaunchHelpAppFromLauncher(ctx context.Context, s *testing.State) {
	cr := s.FixtValue().(fixture.FixtData).Chrome
	tconn := s.FixtValue().(fixture.FixtData).TestAPIConn

	cleanupCtx := ctx
	ctx, cancel := ctxutil.Shorten(ctx, 5*time.Second)
	defer cancel()

	defer faillog.DumpUITreeOnError(cleanupCtx, s.OutDir(), s.HasError, tconn)

	kb, err := input.Keyboard(ctx)
	if err != nil {
		s.Fatal("Failed to find keyboard: ", err)
	}
	defer kb.Close()
	if err := launcher.SearchAndWaitForAppOpen(tconn, kb, apps.Help)(ctx); err != nil {
		s.Fatal("Failed to launch help app: ", err)
	}

	// App should be launched at the overview page.
	if err := helpapp.NewContext(cr, tconn).WaitForApp()(ctx); err != nil {
		s.Fatal("Failed to wait for help app: ", err)
	}
}
