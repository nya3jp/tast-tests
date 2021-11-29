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

func init() {
	testing.AddTest(&testing.Test{
		Func:         LaunchHelpLanguage,
		LacrosStatus: testing.LacrosVariantUnneeded,
		Desc:         "Launch Help APP in different system languages",
		Contacts: []string{
			"showoff-eng@google.com",
			"shengjun@chromium.org",
		},
		Attr:         []string{"group:mainline"},
		SoftwareDeps: []string{"chrome", "chrome_internal"},
		HardwareDeps: hwdep.D(pre.AppsStableModels),
		Fixture:      "chromeLoggedInForEAInJP",
	})
}

func LaunchHelpLanguage(ctx context.Context, s *testing.State) {
	const (
		regionCode       = "jp"
		appName          = "使い方・ヒント"
		helpCategoryName = "新機能"
	)

	cr := s.FixtValue().(*chrome.Chrome)
	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to connect Test API: ", err)
	}

	defer faillog.DumpUITreeOnError(ctx, s.OutDir(), s.HasError, tconn)

	if err := apps.Launch(ctx, tconn, apps.Help.ID); err != nil {
		s.Fatalf("Failed to launch %s: %v", apps.Help.Name, err)
	}

	s.Log("Wait for Help shown in shelf")
	if err := ash.WaitForApp(ctx, tconn, apps.Help.ID, time.Minute); err != nil {
		s.Fatal("Failed to check Help in shelf: ", err)
	}

	s.Logf("Wait for Help app rendering in %s language", regionCode)
	ui := uiauto.New(tconn).WithTimeout(30 * time.Second)

	appRootFinder := nodewith.Name(appName).Role(role.RootWebArea)

	welcomeTextFinder := nodewith.Name(helpCategoryName).First().Ancestor(appRootFinder)
	if err := ui.WaitUntilExists(welcomeTextFinder)(ctx); err != nil {
		s.Fatalf("Failed to launch Help in %s language: %v", regionCode, err)
	}
}
