// Copyright 2021 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package apps

import (
	"context"
	"time"

	"chromiumos/tast/local/apps"
	"chromiumos/tast/local/bundles/cros/apps/fixture"
	"chromiumos/tast/local/bundles/cros/apps/pre"
	"chromiumos/tast/local/chrome/ash"
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/chrome/uiauto/faillog"
	"chromiumos/tast/local/chrome/uiauto/nodewith"
	"chromiumos/tast/local/chrome/uiauto/role"
	"chromiumos/tast/testing"
	"chromiumos/tast/testing/hwdep"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         LaunchGalleryLanguage,
		LacrosStatus: testing.LacrosVariantExists,
		Desc:         "Launch Gallery APP in different system languages",
		Contacts: []string{
			"backlight-swe@google.com",
		},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"chrome", "chrome_internal"},
		HardwareDeps: hwdep.D(pre.AppsStableModels),
		Params: []testing.Param{
			{
				Fixture: fixture.LoggedInJP,
			},
			{
				Name:              "lacros",
				Fixture:           fixture.LacrosLoggedInJP,
				ExtraSoftwareDeps: []string{"lacros_stable"},
			},
		},
	})
}

func LaunchGalleryLanguage(ctx context.Context, s *testing.State) {
	const (
		regionCode          = "jp"
		appName             = "ギャラリー"
		openImageButtonName = "画像を開く"
	)

	tconn := s.FixtValue().(fixture.FixtData).TestAPIConn

	defer faillog.DumpUITreeOnError(ctx, s.OutDir(), s.HasError, tconn)

	// SWA installation is not guaranteed during startup.
	// Using this wait to check installation finished before starting test.
	s.Log("Wait for Gallery to be installed")
	if err := ash.WaitForChromeAppInstalled(ctx, tconn, apps.Gallery.ID, 2*time.Minute); err != nil {
		s.Fatal("Failed to wait for installed app: ", err)
	}

	if err := apps.Launch(ctx, tconn, apps.Gallery.ID); err != nil {
		s.Fatal("Failed to launch Gallery: ", err)
	}

	s.Log("Wait for Gallery shown in shelf")
	if err := ash.WaitForApp(ctx, tconn, apps.Gallery.ID, time.Minute); err != nil {
		s.Fatal("Failed to check Gallery in shelf: ", err)
	}

	s.Logf("Wait for Gallery app rendering in %s language", regionCode)
	ui := uiauto.New(tconn).WithTimeout(30 * time.Second)

	appRootFinder := nodewith.Name(appName).Role(role.RootWebArea)
	openImageButtonFinder := nodewith.Role(role.Button).Name(openImageButtonName).Ancestor(appRootFinder)
	if err := ui.WaitUntilExists(openImageButtonFinder)(ctx); err != nil {
		s.Fatalf("Failed to launch Gallery in %s language: %v", regionCode, err)
	}
}
