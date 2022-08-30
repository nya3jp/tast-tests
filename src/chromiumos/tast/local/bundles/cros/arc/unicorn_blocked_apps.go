// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package arc

import (
	"context"
	"time"

	"chromiumos/tast/common/android/ui"
	"chromiumos/tast/common/policy"
	"chromiumos/tast/local/arc"
	"chromiumos/tast/local/chrome/familylink"
	"chromiumos/tast/local/policyutil"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         UnicornBlockedApps,
		LacrosStatus: testing.LacrosVariantUnneeded,
		Desc:         "Checks if blocked apps cannot be installed from Child Account",
		Contacts:     []string{"rnanjappan@chromium.org", "cros-arc-te@google.com"},
		Attr:         []string{"group:mainline", "informational", "group:arc-functional"},
		SoftwareDeps: []string{
			"chrome",
			"chrome_internal",
			"play_store",
		},
		Timeout: 6 * time.Minute,
		VarDeps: []string{"arc.parentUser", "arc.parentPassword", "arc.childUser", "arc.childPassword"},
		Params: []testing.Param{{
			ExtraSoftwareDeps: []string{"android_p"},
		}, {
			Name:              "vm",
			ExtraSoftwareDeps: []string{"android_vm"},
		}},
		Fixture: "familyLinkUnicornArcPolicyLogin",
	})
}

func UnicornBlockedApps(ctx context.Context, s *testing.State) {
	const (
		DefaultUITimeout     = 1 * time.Minute
		installButtonText    = "install"
		provisioningTimeout  = 3 * time.Minute
		maxAttempts          = 2
		playStorePackage     = "com.android.vending"
		assetBrowserActivity = "com.android.vending.AssetBrowserActivity"
	)
	fdms := s.FixtValue().(*familylink.FixtData).FakeDMS
	cr := s.FixtValue().(*familylink.FixtData).Chrome
	tconn := s.FixtValue().(*familylink.FixtData).TestConn
	arcEnabledPolicy := &policy.ArcEnabled{Val: true}

	policies := []policy.Policy{arcEnabledPolicy}
	pb := policy.NewBlob()
	pb.PolicyUser = s.FixtValue().(*familylink.FixtData).PolicyUser
	pb.AddPolicies(policies)

	if err := policyutil.ServeBlobAndRefresh(ctx, fdms, cr, pb); err != nil {
		s.Fatal("Failed to serve policies: ", err)
	}

	// Setup ARC.
	a, err := arc.New(ctx, s.OutDir())
	if err != nil {
		s.Fatal("Failed to connect to ARC: ", err)
	}
	defer a.Close(ctx)

	if err := a.WaitForProvisioning(ctx, provisioningTimeout); err != nil {
		s.Fatal("Failed to wait for provisioning: ", err)
	}

	s.Log("Starting Play Store")
	act, err := arc.NewActivity(a, playStorePackage, assetBrowserActivity)
	if err != nil {
		s.Fatal("Failed to create new activity: ", err)
	}
	defer act.Close()

	if err := act.Start(ctx, tconn); err != nil {
		s.Fatal("Failed starting Play Store or Play Store is empty: ", err)
	}

	d, err := a.NewUIDevice(ctx)
	if err != nil {
		s.Fatal("Failed initializing UI Automator: ", err)
	}
	defer d.Close(ctx)

	searchText := d.Object(ui.ClassName("android.widget.TextView"), ui.Text("Search for apps & games"))
	if err := searchText.WaitForExists(ctx, DefaultUITimeout); err != nil {
		s.Error("searchText doesn't exist: ", err)
	} else if err := searchText.Click(ctx); err != nil {
		s.Fatal("Failed to click on searchText: ", err)
	}

	searchTextEdit := d.Object(ui.ClassName("android.widget.EditText"), ui.Text("Search for apps & games"))
	if err := searchTextEdit.SetText(ctx, "youtube.creator"); err != nil {
		s.Fatal("Failed to searchText: ", err)
	} else if err := d.PressKeyCode(ctx, ui.KEYCODE_ENTER, 0); err != nil {
		s.Fatal("Failed to click on KEYCODE_ENTER button: ", err)
	}

	// Verify that install button is disabled for the blocked app.
	installButton := d.Object(ui.ClassName("android.widget.Button"), ui.TextMatches("(?i)"+installButtonText))
	if err := installButton.WaitForExists(ctx, DefaultUITimeout); err != nil {
		s.Fatal("Failed to find the install button for blocked app: ", err)
	}

	if enabled, err := installButton.IsEnabled(ctx); err != nil {
		s.Fatal("Failed to check install button state")
	} else if enabled {
		s.Fatal("Install button is enabled")
	}
}
