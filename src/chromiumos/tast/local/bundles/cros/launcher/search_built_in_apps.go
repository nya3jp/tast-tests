// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package launcher

import (
	"context"

	"chromiumos/tast/local/apps"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/ash"
	"chromiumos/tast/local/chrome/ui/faillog"
	"chromiumos/tast/local/chrome/ui/launcher"
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
		Attr:         []string{"group:mainline"},
		SoftwareDeps: []string{"chrome"},
		Params: []testing.Param{
			{
				Pre: chrome.LoggedIn(),
				Val: apps.Settings,
			},
			{
				Name:              "lacros",
				Val:               apps.Lacros,
				Pre:               lacroslauncher.StartedByDataUI(),
				ExtraAttr:         []string{"informational"},
				ExtraData:         []string{lacroslauncher.DataArtifact},
				ExtraSoftwareDeps: []string{"lacros"},
			}},
	})
}

// SearchBuiltInApps searches for the Settings app in the Launcher.
func SearchBuiltInApps(ctx context.Context, s *testing.State) {
	cr, err := lacros.GetChrome(ctx, s.PreValue())
	if err != nil {
		s.Fatal("Failed to get a Chrome instance: ", err)
	}
	app := s.Param().(apps.App)
	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to connect Test API: ", err)
	}
	defer faillog.DumpUITreeOnError(ctx, s.OutDir(), s.HasError, tconn)

	if err := launcher.SearchAndLaunch(ctx, tconn, app.Name); err != nil {
		s.Fatal("Failed to launch app: ", err)
	}

	if err := ash.WaitForApp(ctx, tconn, app.ID); err != nil {
		s.Fatal("Failed to wait for app: ", err)
	}
}
