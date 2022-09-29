// Copyright 2022 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package arc

import (
	"context"
	"path/filepath"
	"time"

	"chromiumos/tast/common/android/ui"
	"chromiumos/tast/common/policy"
	"chromiumos/tast/common/policy/fakedms"
	"chromiumos/tast/common/testexec"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/arc"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/familylink"
	"chromiumos/tast/local/policyutil"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         UnicornBlockedApps,
		LacrosStatus: testing.LacrosVariantUnneeded,
		Desc:         "Checks if blocked apps cannot be installed from Child Account",
		Contacts:     []string{"mhasank@google.com", "arc-commercial@google.com"},
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
		logcatBufferSize     = "10M"
		blockedPackage       = "com.google.android.apps.youtube.creator"
	)
	fdms := s.FixtValue().(fakedms.HasFakeDMS).FakeDMS()
	cr := s.FixtValue().(chrome.HasChrome).Chrome()
	tconn := s.FixtValue().(familylink.HasTestConn).TestConn()
	arcEnabledPolicy := &policy.ArcEnabled{Val: true}
	blockedApps := []policy.Application{
		{
			PackageName: blockedPackage,
			InstallType: "BLOCKED",
		},
	}
	blockedAppsPolicy := &policy.ArcPolicy{
		Val: &policy.ArcPolicyValue{
			Applications: blockedApps,
		},
	}
	policies := []policy.Policy{blockedAppsPolicy, arcEnabledPolicy}
	pb := policy.NewBlob()
	pb.PolicyUser = s.FixtValue().(familylink.HasPolicyUser).PolicyUser()
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

	verboseTags := []string{"clouddpc", "Finsky", "Volley", "PlayCommon"}
	if err := a.EnableVerboseLogging(ctx, verboseTags...); err != nil {
		s.Fatal("Unable to change log level: ", err)
	}

	if err := a.Command(ctx, "logcat", "-G", logcatBufferSize).Run(testexec.DumpLogOnError); err != nil {
		s.Fatal("Unable to increase buffer size: ", err)
	}

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

	installButton := d.Object(ui.ClassName("android.widget.Button"), ui.TextMatches("(?i)"+installButtonText))
	if err := installButton.WaitForExists(ctx, DefaultUITimeout); err != nil {
		s.Fatal("Failed to find the install button for blocked app: ", err)
	}

	if enabled, err := installButton.IsEnabled(ctx); err != nil {
		s.Fatal("Failed to check install button state")
	} else if !enabled {
		testing.ContextLog(ctx, "Install button is disabled")
	} else if err := validateAutoUninstall(ctx, a, installButton, blockedPackage); err != nil {
		dumpBugReport(ctx, a, s.OutDir())
		s.Fatal("Blocked package did not uninstall: ", err)
	}
}

func validateAutoUninstall(ctx context.Context, a *arc.ARC, installButton *ui.Object, blockedPackage string) error {
	testing.ContextLog(ctx, "Install button is enabled. Attempting install")
	if err := installButton.Click(ctx); err != nil {
		return errors.Wrap(err, "failed to click the install button")
	}

	if err := a.WaitForPackages(ctx, []string{blockedPackage}); err != nil {
		return errors.Wrap(err, "package installation failed")
	}

	testing.ContextLog(ctx, "Waiting for package to uninstall")
	if err := waitForUninstall(ctx, a, blockedPackage); err != nil {
		return errors.Wrap(err, "package not uninstalled")
	}

	return nil
}

func waitForUninstall(ctx context.Context, a *arc.ARC, blockedPackage string) error {
	return testing.Poll(ctx, func(ctx context.Context) error {
		if installed, err := a.PackageInstalled(ctx, blockedPackage); err != nil {
			return testing.PollBreak(err)
		} else if installed {
			return errors.New("Package not yet uninstalled")
		}
		return nil
	}, &testing.PollOptions{Interval: time.Second})
}

func dumpBugReport(ctx context.Context, a *arc.ARC, outDir string) {
	if err := a.BugReport(ctx, filepath.Join(outDir, "bugreport.zip")); err != nil {
		testing.ContextLog(ctx, "Failed to get bug report: ", err)
	}
}
