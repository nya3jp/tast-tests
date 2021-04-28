// Copyright 2021 The Chromium OS Authors. All rights reserved.
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
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/chrome/uiauto/faillog"
	"chromiumos/tast/local/chrome/uiauto/nodewith"
	"chromiumos/tast/local/chrome/uiauto/role"
	"chromiumos/tast/testing"
	"chromiumos/tast/testing/hwdep"
)

type langTestParameters struct {
	regionCode          string
	appName             string
	openImageButtonName string
}

func init() {
	testing.AddTest(&testing.Test{
		Func: LaunchGalleryLanguage,
		Desc: "Launch Gallery APP in different system languages",
		Contacts: []string{
			"backlight-swe@google.com",
			"shengjun@chromium.org",
		},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"chrome"},
		HardwareDeps: hwdep.D(pre.AppsStableModels),
		Params: []testing.Param{
			{
				Name: "es",
				Val: langTestParameters{
					regionCode:          "es",
					appName:             "Galería",
					openImageButtonName: "Ábrela",
				},
			}, {
				Name: "jp",
				Val: langTestParameters{
					regionCode:          "jp",
					appName:             "ギャラリー",
					openImageButtonName: "画像を開く",
				},
			},
		},
	})
}

func LaunchGalleryLanguage(ctx context.Context, s *testing.State) {
	regionCode := s.Param().(langTestParameters).regionCode
	appName := s.Param().(langTestParameters).appName
	openImageButtonName := s.Param().(langTestParameters).openImageButtonName

	cr, err := chrome.New(ctx, chrome.Region(regionCode))
	if err != nil {
		s.Fatalf("Failed to start Chrome in region %s: %v", regionCode, err)
	}
	defer cr.Close(ctx)

	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to connect Test API: ", err)
	}

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
