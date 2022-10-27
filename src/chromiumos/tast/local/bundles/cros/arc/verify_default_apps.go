// Copyright 2020 The ChromiumOS Authors
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
	"chromiumos/tast/local/chrome/uiauto/faillog"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         VerifyDefaultApps,
		LacrosStatus: testing.LacrosVariantUnneeded,
		Desc:         "Verifies Default arc apps are installed",
		Contacts:     []string{"cpiao@google.com", "arc-eng@google.com", "cros-arc-te@google.com"},
		Attr:         []string{"group:mainline", "informational", "group:arc-functional"},
		Timeout:      5 * time.Minute,
		SoftwareDeps: []string{"chrome"},
		Fixture:      "arcBootedWithPlayStore",
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
	defer faillog.DumpUITreeOnError(ctx, s.OutDir(), s.HasError, tconn)

	// Lookup for ARC++ default apps
	apps := []apps.App{
		apps.PlayStore,
		apps.PlayBooks,
		apps.PlayGames,
		apps.GoogleTV,
		apps.Photos,
		apps.Clock,
		apps.Contacts,
	}

	for _, app := range apps {
		if err := ash.WaitForChromeAppInstalled(ctx, tconn, app.ID, ctxutil.MaxTimeout); err != nil {
			s.Errorf("Failed to wait for %s (%s) to be installed: %v", app.Name, app.ID, err)
		}
	}
}
