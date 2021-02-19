// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package arc

import (
	"context"
	"time"

	"chromiumos/tast/common/policy"
	"chromiumos/tast/common/policy/fakedms"
	"chromiumos/tast/local/apps"
	"chromiumos/tast/local/chrome/ash"
	"chromiumos/tast/local/chrome/familylink"
	"chromiumos/tast/local/policyutil"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         UnicornDefaultApps,
		Desc:         "Verifies Default arc apps for Unicorn Account",
		Contacts:     []string{"rnanjappan@chromium.org", "cros-arc-te@google.com"},
		Attr:         []string{"group:mainline", "informational", "group:arc-functional"},
		Timeout:      10 * time.Minute,
		SoftwareDeps: []string{"chrome"},
		Params: []testing.Param{{
			ExtraSoftwareDeps: []string{"android_p"},
		}, {
			Name:              "vm",
			ExtraSoftwareDeps: []string{"android_vm"},
		}},
		Fixture: "familyLinkUnicornArcPolicyLogin",
	})
}

func UnicornDefaultApps(ctx context.Context, s *testing.State) {

	cr := s.FixtValue().(*familylink.FixtData).Chrome
	tconn := s.FixtValue().(*familylink.FixtData).TestConn
	fdms := s.FixtValue().(*familylink.FixtData).FakeDMS

	arcEnabledPolicy := &policy.ArcEnabled{Val: true}

	policies := []policy.Policy{arcEnabledPolicy}

	pb := fakedms.NewPolicyBlob()
	pb.AddPolicies(policies)
	if err := policyutil.ServeBlobAndRefresh(ctx, fdms, cr, pb); err != nil {
		s.Fatal("Failed to serve policies: ", err)
	}
	if err := policyutil.Verify(ctx, tconn, policies); err != nil {
		s.Fatal("Failed to verify policies: ", err)
	}

	testing.Sleep(ctx, 1*time.Minute)

	// Lookup for ARC++ default apps
	apps := []apps.App{
		apps.PlayBooks,
		apps.PlayGames,
		apps.PlayMovies,
		apps.Photos,
		apps.Clock,
		apps.Contacts,
	}

	for _, app := range apps {
		installed, err := ash.ChromeAppInstalled(ctx, tconn, app.ID)
		if err != nil {
			s.Fatal("Failed to check ChromeAppInstalled: ", err)
		} else if installed {
			s.Errorf("App %s (%s) is installed on child account", app.Name, app.ID)
		}
	}

}
