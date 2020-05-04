// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package ui

import (
	"context"
	"time"

	"chromiumos/tast/local/apps"
	"chromiumos/tast/local/bundles/cros/ui/faillog"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/ash"
	"chromiumos/tast/local/chrome/ui/launcher"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func: LaunchCanvas,
		Desc: "Launches Chrome Canvas APP through the launcher after user login",
		Contacts: []string{
			"shengjun@chromium.org",
			"xiaoningw@chromium.org",
			"jomag@chromium.org",
		},
		Attr:         []string{"group:mainline", "informational"},
		Timeout:      3 * time.Minute,
		SoftwareDeps: []string{"chrome"},
	})
}

// LaunchCanvas Verfy launching Canvas after OOBE
func LaunchCanvas(ctx context.Context, s *testing.State) {
	cr, err := chrome.New(ctx)
	if err != nil {
		s.Fatal("Failed to start Chrome: ", err)
	}
	defer cr.Close(ctx)

	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to connect Test API: ", err)
	}

	if err := launcher.OpenLauncher(ctx, tconn); err != nil {
		s.Fatal("Failed to open launcher: ", err)
	}

	defer faillog.DumpUITreeOnError(ctx, s, tconn)

	//Canvas is a PWA installed after user login. Set long timeout for download.
	appNode, err := launcher.SearchAppAndAssert(ctx, tconn, apps.Canvas.Name, 2*time.Minute)
	if err != nil {
		s.Fatal("Failed to search app and assert: ", err)
	}

	if err := appNode.LeftClick(ctx); err != nil {
		s.Fatal("Failed to click to launch app "+apps.Canvas.Name+":", err)
	}

	if err := ash.WaitForApp(ctx, tconn, apps.Canvas.ID); err != nil {
		s.Fatal("Fail to wait for "+apps.Canvas.Name+" by app id "+apps.Canvas.ID+":", err)
	}
}
