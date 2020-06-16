// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package arc

import (
	"context"
	"time"

	"chromiumos/tast/local/apps"
	"chromiumos/tast/local/arc"
	"chromiumos/tast/local/chrome/ash"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         VerifyDefaultApps,
		Desc:         "Verifies Default arc apps",
		Contacts:     []string{"vkrishan@google.com", "rohitbm@google.com"},
		Attr:         []string{"group:mainline", "informational"},
		Timeout:      3 * time.Minute,
		SoftwareDeps: []string{"chrome"},
		Params: []testing.Param{{
			ExtraSoftwareDeps: []string{"android_p"},
			Pre:               arc.Booted(),
		}, {
			Name:              "vm",
			ExtraSoftwareDeps: []string{"android_vm"},
			Pre:               arc.VMBooted(),
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
	if err := ash.WaitForChromeAppInstalled(ctx, tconn, apps.PlayStore.ID, 3*time.Minute); err != nil {
		s.Fatal("Failed to wait for installed app: ", err)
	}

	if err := ash.WaitForChromeAppInstalled(ctx, tconn, apps.Duo.ID, 3*time.Minute); err != nil {
		s.Fatal("Failed to wait for installed app: ", err)
	}

	if err := ash.WaitForChromeAppInstalled(ctx, tconn, apps.PlayMusic.ID, 3*time.Minute); err != nil {
		s.Fatal("Failed to wait for installed app: ", err)
	}

	if err := ash.WaitForChromeAppInstalled(ctx, tconn, apps.PlayBooks.ID, 3*time.Minute); err != nil {
		s.Fatal("Failed to wait for installed app: ", err)
	}

	if err := ash.WaitForChromeAppInstalled(ctx, tconn, apps.PlayGames.ID, 3*time.Minute); err != nil {
		s.Fatal("Failed to wait for installed app: ", err)
	}

	if err := ash.WaitForChromeAppInstalled(ctx, tconn, apps.PlayMovies.ID, 3*time.Minute); err != nil {
		s.Fatal("Failed to wait for installed app: ", err)
	}
}
