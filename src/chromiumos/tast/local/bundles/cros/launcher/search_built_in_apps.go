// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package launcher

import (
	"context"

	"chromiumos/tast/local/apps"
	"chromiumos/tast/local/chrome/uiauto/faillog"
	"chromiumos/tast/local/chrome/uiauto/launcher"
	"chromiumos/tast/local/input"
	"chromiumos/tast/local/lacros"
	lacroslauncher "chromiumos/tast/local/lacros/launcher"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func: SearchBuiltInApps,
		Desc: "Launches a built-in app through the launcher",
		Contacts: []string{
			"chromeos-sw-engprod@google.com",
			"tbarzic@chromium.org",
		},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"chrome"},
		Params: []testing.Param{
			{
				Val:     apps.Settings,
				Fixture: "chromeLoggedIn",
			},
			{
				Name:              "lacros",
				Val:               apps.Lacros,
				Fixture:           "lacrosStartedByDataUI",
				ExtraAttr:         []string{"informational"},
				ExtraData:         []string{lacroslauncher.DataArtifact},
				ExtraSoftwareDeps: []string{"lacros"},
			}},
	})
}

// SearchBuiltInApps searches for the Settings app in the Launcher.
func SearchBuiltInApps(ctx context.Context, s *testing.State) {
	// TODO(crbug.com/1127165): Remove the artifactPath argument when we can use Data in fixtures.
	if s.Param().(apps.App) == apps.Lacros {
		if err := lacroslauncher.EnsureLacrosChrome(ctx, s.FixtValue().(lacroslauncher.FixtData), s.DataPath(lacroslauncher.DataArtifact)); err != nil {
			s.Fatal("Failed to extract lacros binary: ", err)
		}
	}

	cr, err := lacros.GetChrome(ctx, s.FixtValue())
	if err != nil {
		s.Fatal("Failed to get a Chrome instance: ", err)
	}
	app := s.Param().(apps.App)
	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to connect Test API: ", err)
	}
	defer faillog.DumpUITreeOnError(ctx, s.OutDir(), s.HasError, tconn)

	kb, err := input.Keyboard(ctx)
	if err != nil {
		s.Fatal("Failed to find keyboard: ", err)
	}
	defer kb.Close()
	if err := launcher.SearchAndWaitForAppOpen(tconn, kb, app)(ctx); err != nil {
		s.Fatal("Failed to launch app: ", err)
	}
}
