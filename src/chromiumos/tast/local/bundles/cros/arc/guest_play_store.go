// Copyright 2021 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package arc

import (
	"context"
	"time"

	"chromiumos/tast/local/apps"
	"chromiumos/tast/local/arc"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/ash"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         GuestPlayStore,
		LacrosStatus: testing.LacrosVariantUnneeded,
		Desc:         "Check PlayStore is Off in Guest mode",
		Contacts:     []string{"cpiao@google.com", "cros-arc-te@google.com"},
		Attr:         []string{"group:mainline", "group:arc-functional"},
		SoftwareDeps: []string{"chrome"},
		Fixture:      "chromeLoggedInGuest",
		Timeout:      chrome.LoginTimeout + arc.BootTimeout + 30*time.Second,
		Params: []testing.Param{{
			ExtraSoftwareDeps: []string{"android_p"},
		}, {
			Name:              "vm",
			ExtraSoftwareDeps: []string{"android_vm"},
		}},
	})
}

func GuestPlayStore(ctx context.Context, s *testing.State) {
	cr := s.FixtValue().(*chrome.Chrome)

	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to connect Test API: ", err)
	}

	s.Log("Verify None of Default ARC Apps are Installed")
	installedApps, err := ash.ChromeApps(ctx, tconn)
	if err != nil {
		s.Fatal("Failed to get installed apps: ", err)
	}
	for _, app := range []apps.App{apps.PlayStore, apps.Duo, apps.PlayBooks, apps.PlayGames, apps.GoogleTV, apps.Clock, apps.Contacts} {
		for _, installedapp := range installedApps {
			if app.ID == installedapp.AppID {
				s.Fatalf("%s (%s) App is installed", app.Name, app.ID)
			}
		}
	}
}
