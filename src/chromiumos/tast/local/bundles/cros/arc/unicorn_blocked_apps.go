// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package arc

import (
	"context"
	"sort"
	"time"

	"chromiumos/tast/common/policy"
	"chromiumos/tast/common/policy/fakedms"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/android/ui"
	"chromiumos/tast/local/apps"
	"chromiumos/tast/local/arc"
	"chromiumos/tast/local/arc/optin"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/familylink"
	"chromiumos/tast/local/chrome/uiauto/launcher"
	"chromiumos/tast/local/policyutil"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         UnicornBlockedApps,
		Desc:         "Checks if blocked apps cannot be installed from Child Account",
		Contacts:     []string{"rnanjappan@chromium.org", "cros-arc-te@google.com"},
		Attr:         []string{"group:mainline", "informational", "group:arc-functional"},
		SoftwareDeps: []string{"chrome"},
		VarDeps:      []string{"arc.parentUser", "arc.parentPassword", "arc.childUser", "arc.childPassword"},
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
		DefaultUITimeout  = 60 * time.Second
		installButtonText = "install"
	)
	pkgName := []string{"com.google.android.apps.youtube.creator"}

	fdms := s.FixtValue().(*familylink.FixtData).FakeDMS
	cr := s.FixtValue().(*familylink.FixtData).Chrome
	tconn := s.FixtValue().(*familylink.FixtData).TestConn

	// Update the policy with blocked apps list.
	arcEnabledPolicy := &policy.ArcEnabled{Val: true}
	blockedApps := []policy.Application{
		{
			PackageName: pkgName[0],
			InstallType: "BLOCKED",
		},
	}
	blockedAppsPolicy := &policy.ArcPolicy{
		Val: &policy.ArcPolicyValue{
			Applications: blockedApps,
		},
	}
	policies := []policy.Policy{blockedAppsPolicy, arcEnabledPolicy}
	pb := fakedms.NewPolicyBlob()
	pb.PolicyUser = s.FixtValue().(*familylink.FixtData).PolicyUser
	pb.AddPolicies(policies)

	if err := policyutil.ServeBlobAndRefresh(ctx, fdms, cr, pb); err != nil {
		s.Fatal("Failed to serve policies: ", err)
	}

	// Ensure chrome://policy shows correct ArcEnabled and ArcPolicy values.
	if err := policyutil.Verify(ctx, tconn, []policy.Policy{&policy.ArcEnabled{Val: true}}); err != nil {
		s.Fatal("Failed to verify ArcEnabled: ", err)
	}

	if err := verifyArcPolicyBlockedApps(ctx, tconn, pkgName); err != nil {
		s.Fatal("Failed to verify blocked apps in ArcPolicy: ", err)
	}

	if err := launcher.LaunchApp(tconn, apps.PlayStore.Name)(ctx); err != nil {
		s.Fatal("Failed to launch Play Store")
	}

	// Setup ARC.
	a, err := arc.New(ctx, s.OutDir())
	if err != nil {
		s.Fatal("Failed to connect to ARC: ", err)
	}
	defer a.Close(ctx)

	d, err := a.NewUIDevice(ctx)
	if err != nil {
		s.Fatal("Failed initializing UI Automator: ", err)
	}
	defer d.Close(ctx)

	// Verify PlayStore is Open.
	if err := optin.WaitForPlayStoreShown(ctx, tconn, time.Minute); err != nil {
		// Ensure Play Store is not empty.
		s.Log("Starting Play Store")
		act, err := arc.NewActivity(a, "com.android.vending", "com.android.vending.AssetBrowserActivity")
		if err != nil {
			s.Fatal("Failed to create new activity: ", err)
		}
		defer act.Close()

		if err := act.Start(ctx, tconn); err != nil {
			s.Fatal("Failed starting Play Store or Play Store is empty: ", err)
		}
	}

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
	installButton := d.Object(ui.ClassName("android.widget.Button"), ui.TextMatches("(?i)"+installButtonText), ui.Enabled(true))
	if err := installButton.Exists(ctx); err == nil {
		s.Fatal("Install Button Exisits for Blocked App: ", err)
	}

}

// verifyArcPolicyBlockedApps matches ArcPolicy BLOCKD apps list with expected packages.
func verifyArcPolicyBlockedApps(ctx context.Context, tconn *chrome.TestConn, appslist []string) error {
	dps, err := policyutil.PoliciesFromDUT(ctx, tconn)
	if err != nil {
		return err
	}

	expected := &policy.ArcPolicy{}
	actual, ok := dps.Chrome[expected.Name()]
	if !ok {
		return errors.New("no such a policy ArcPolicy")
	}
	value, err := expected.UnmarshalAs(actual.ValueJSON)
	if err != nil {
		return err
	}
	arcPolicyValue, ok := value.(policy.ArcPolicyValue)
	if !ok {
		return errors.Errorf("cannot extract ArcPolicyValue %s", value)
	}

	apps := arcPolicyValue.Applications
	blocked := make(map[string]bool)
	for _, application := range apps {
		packageName := application.PackageName
		installType := application.InstallType
		if installType == "BLOCKED" {
			blocked[packageName] = true
		}
	}
	for _, app := range appslist {
		if blocked[app] {
			delete(blocked, app)
		} else {
			return errors.Errorf("the package not BLOCKED by policy: %s", app)
		}
	}
	if len(blocked) != 0 {
		return errors.Errorf("Extra BLOCKED packages in ArcPolicy: %s", makeList(blocked))
	}
	return nil
}

// makeList returns a list of keys from map.
func makeList(packages map[string]bool) []string {
	var packagesList []string
	for pkg := range packages {
		packagesList = append(packagesList, pkg)
	}
	sort.Strings(packagesList)
	return packagesList
}
