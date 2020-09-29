// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package apps

import (
	"context"
	"time"

	"chromiumos/tast/local/apps"
	"chromiumos/tast/local/bundles/cros/apps/pre"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/ash"
	"chromiumos/tast/local/chrome/ui/faillog"
	"chromiumos/tast/local/chrome/ui/launcher"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func: StadiaDefault,
		Desc: "Check Stadia is installed by default",
		Contacts: []string{
			"jshikaram@chromium.org",
			"benreich@chromium.org",
		},
		Attr:         []string{"group:mainline", "informational"},
		Timeout:      15 * time.Minute,
		SoftwareDeps: []string{"chrome"},
		Vars:         []string{"apps.StadiaDefault.username", "apps.StadiaDefault.password"},
		Params: []testing.Param{
			{
				Name:              "available",
				Val:               true,
				ExtraHardwareDeps: pre.StadiaPreloadedModels,
			}, {
				Name:              "unavailable",
				Val:               false,
				ExtraHardwareDeps: pre.StadiaUnavailableModels,
			},
		},
	})
}

// StadiaDefault verifies launching Gallery on opening supported files.
func StadiaDefault(ctx context.Context, s *testing.State) {
	stadiaPreloaded := s.Param().(bool)
	username := s.RequiredVar("apps.StadiaDefault.username")
	password := s.RequiredVar("apps.StadiaDefault.password")

	s.Log("Expecting Stadia to be preloaded: ", stadiaPreloaded)

	cr, err := chrome.New(ctx, chrome.Auth(username, password, ""), chrome.GAIALogin(), chrome.EnableWebAppInstall())
	if err != nil {
		s.Fatal("Failed to start Chrome: ", err)
	}

	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to connect Test API: ", err)
	}
	defer faillog.DumpUITreeOnError(ctx, s.OutDir(), s.HasError, tconn)

	// If this model requires Stadia to be preloaded, wait for it to be installed.
	if stadiaPreloaded {
		if err := ash.WaitForChromeAppInstalled(ctx, tconn, apps.Stadia.ID, 2*time.Minute); err != nil {
			s.Fatal("Failed to wait for Stadia to be installed: ", err)
		}
	}

	if err := launcher.OpenLauncher(ctx, tconn); err != nil {
		s.Fatal("Failed to open launcher: ", err)
	}

	// Search for "Stadia".
	if err := launcher.Search(ctx, tconn, "Stadia"); err != nil {
		s.Fatal("Failed to search for get help: ", err)
	}

	// Stadia should be one of the search results.
	if _, err = launcher.WaitForAppResult(ctx, tconn, apps.Stadia.Name, time.Minute); err != nil && stadiaPreloaded {
		s.Fatal("Stadia does not exist in search result: ", err)
	}
}
