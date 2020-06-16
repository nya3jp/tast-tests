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
		Contacts:     []string{"vkrishan@google.com", "rohitbm@google.com", "arc-eng@google.com"},
		Attr:         []string{"group:mainline", "informational"},
		Timeout:      3 * time.Minute,
		SoftwareDeps: []string{"chrome"},
		Pre:          arc.Booted(),
		Params: []testing.Param{{
			ExtraSoftwareDeps: []string{"android_p"},
		}, {
			Name:              "vm",
			ExtraSoftwareDeps: []string{"android_vm"},
		}},
	})
}

func VerifyDefaultApps(ctx context.Context, s *testing.State) {

	cr := s.PreValue().(arc.PreData).Chrome
	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to connect Test API: ", err)
	}

	// Lookup for ARC++ default apps
	for _, app := range []apps.App{apps.PlayStore, apps.Duo, apps.PlayMusic, apps.PlayBooks, apps.PlayGames, apps.PlayMovies} {
		if err := ash.WaitForChromeAppInstalled(ctx, tconn, app.ID, ctxutil.MaxTimeout); err != nil {
			s.Fatalf("Failed to wait for %s (%s) to be installed: %v", app.Name, app.ID, err)
		}
	}
}
