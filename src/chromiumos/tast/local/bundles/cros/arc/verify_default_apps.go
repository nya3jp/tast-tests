// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package arc

import (
	"context"
	"time"

	"chromiumos/tast/ctxutil"
	"chromiumos/tast/local/apps"
	"chromiumos/tast/local/arc"
	"chromiumos/tast/local/chrome/ash"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         VerifyDefaultApps,
		Desc:         "Verifies Default arc apps are installed",
		Contacts:     []string{"vkrishan@google.com", "arc-eng@google.com", "cros-arc-te@google.com"},
		Attr:         []string{"group:mainline", "informational", "group:arc-functional"},
		Timeout:      3 * time.Minute,
		SoftwareDeps: []string{"chrome"},
		Fixture:      "arcBooted",
		Params: []testing.Param{{
			ExtraSoftwareDeps: []string{"android_p"},
		}, {
			Name:              "vm",
			ExtraSoftwareDeps: []string{"android_vm"},
		}},
	})
}

// VerifyDefaultApps checks that default ARC apps are available after boot.
// Some of these apps are installed through PAI and will only have "promise
// icon" stubs available on first boot. This PAI integration is tested
// separately by arc.PlayAutoInstall.
func VerifyDefaultApps(ctx context.Context, s *testing.State) {

	cr := s.FixtValue().(*arc.PreData).Chrome
	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to connect Test API: ", err)
	}

	// Lookup for ARC++ default apps
	apps := []apps.App{
		apps.PlayStore,
		apps.PlayBooks,
		apps.PlayGames,
		apps.PlayMovies,
		apps.Photos,
		apps.Clock,
		apps.Contacts,
	}
	for _, app := range apps {
		if err := ash.WaitForChromeAppInstalled(ctx, tconn, app.ID, ctxutil.MaxTimeout); err != nil {
			s.Fatalf("Failed to wait for %s (%s) to be installed: %v", app.Name, app.ID, err)
		}
	}
}
