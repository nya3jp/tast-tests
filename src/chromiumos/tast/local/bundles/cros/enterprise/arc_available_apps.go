// Copyright 2022 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package enterprise

import (
	"context"
	"time"

	"chromiumos/tast/common/android/ui"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/arc"
	"chromiumos/tast/local/arc/playstore"
	"chromiumos/tast/local/bundles/cros/enterprise/arcent"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         ARCAvailableApps,
		LacrosStatus: testing.LacrosVariantUnneeded,
		Desc:         "Checks that available apps can be installed in Play Store",
		Contacts:     []string{"mhasank@chromium.org", "arc-commercial@google.com"},
		Attr:         []string{"group:mainline"},
		SoftwareDeps: []string{"chrome", "play_store"},
		Timeout:      15 * time.Minute,
		VarDeps: []string{
			arcent.LoginPoolVar,
		},
		Params: []testing.Param{
			{
				ExtraSoftwareDeps: []string{"android_p"},
				ExtraAttr:         []string{"informational"},
			},
			{
				Name:              "vm",
				ExtraSoftwareDeps: []string{"android_vm"},
				ExtraAttr:         []string{"informational"},
			}},
	})
}

// ARCAvailableApps verifies that allow-listed app can be installed in Play Store.
func ARCAvailableApps(ctx context.Context, s *testing.State) {
	const (
		bootTimeout          = 4 * time.Minute
		maxAttempts          = 2
		installButtonText    = "install"
		testPackage          = "com.google.android.calculator"
		playStorePackage     = "com.android.vending"
		assetBrowserActivity = "com.android.vending.AssetBrowserActivity"
		defaultUITimeout     = 1 * time.Minute
	)

	packages := []string{testPackage}
	attempts := 1

	// Indicates a failure in the core feature under test so the polling should stop.
	exit := func(desc string, err error) error {
		s.Fatalf("Failed to %s: %v", desc, err)
		return nil
	}

	// Indicates that the error is retryable and unrelated to core feature under test.
	retry := func(desc string, err error) error {
		if attempts < maxAttempts {
			attempts++
			err = errors.Wrap(err, "failed to "+desc)
			s.Logf("%s. Retrying", err)
			return err
		}
		return exit(desc, err)
	}

	creds, err := chrome.PickRandomCreds(s.RequiredVar(arcent.LoginPoolVar))
	if err != nil {
		exit("get login creds", err)
	}
	login := chrome.GAIALogin(creds)

	fdms, err := arcent.SetupPolicyServerWithArcApps(ctx, s.OutDir(), creds.User, packages, arcent.InstallTypeAvailable)
	if err != nil {
		exit("setup fake policy server", err)
	}
	defer fdms.Stop(ctx)

	if err := testing.Poll(ctx, func(ctx context.Context) (retErr error) {
		cr, err := chrome.New(
			ctx,
			login,
			chrome.ARCSupported(),
			chrome.UnRestrictARCCPU(),
			chrome.DMSPolicy(fdms.URL),
			chrome.ExtraArgs(arc.DisableSyncFlags()...))
		if err != nil {
			return retry("connect to Chrome", err)
		}
		defer cr.Close(ctx)

		tconn, err := cr.TestAPIConn(ctx)
		if err != nil {
			return retry("create test API connection", err)
		}

		a, err := arc.NewWithTimeout(ctx, s.OutDir(), bootTimeout)
		if err != nil {
			return exit("start ARC by policy", err)
		}
		defer a.Close(ctx)

		if err := arcent.ConfigureProvisioningLogs(ctx, a); err != nil {
			return exit("configure provisioning logs", err)
		}

		if err := arcent.WaitForProvisioning(ctx, a, attempts); err != nil {
			return exit("wait for provisioning", err)
		}

		if err := arcent.EnsurePlayStoreNotEmpty(ctx, tconn, cr, a, s.OutDir(), attempts); err != nil {
			return exit("verify Play Store is not empty", err)
		}

		if err := playstore.OpenAppPage(ctx, a, testPackage); err != nil {
			exit("open app page", err)
			return
		}

		d, err := a.NewUIDevice(ctx)
		if err != nil {
			exit("initialize UI Automator: ", err)
		}
		defer d.Close(ctx)

		installButton := d.Object(ui.ClassName("android.widget.Button"), ui.TextMatches("(?i)"+installButtonText))
		if err := installButton.WaitForExists(ctx, defaultUITimeout); err != nil {
			exit("find the install button", err)
		}

		if enabled, err := installButton.IsEnabled(ctx); err != nil {
			exit("check install button state", err)
		} else if !enabled {
			exit("verify install button is disabled", nil)
		}

		return nil
	}, nil); err != nil {
		s.Fatal("Allow install test failed: ", err)
	}
}
