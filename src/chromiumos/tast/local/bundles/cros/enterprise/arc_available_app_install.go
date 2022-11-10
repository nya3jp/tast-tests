// Copyright 2022 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package enterprise

import (
	"context"
	"time"

	"chromiumos/tast/local/arc"
	"chromiumos/tast/local/arc/arcent"
	"chromiumos/tast/local/arc/playstore"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/retry"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         ARCAvailableAppInstall,
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

// ARCAvailableAppInstall verifies that allow-listed app can be installed in Play Store.
func ARCAvailableAppInstall(ctx context.Context, s *testing.State) {
	const (
		bootTimeout      = 4 * time.Minute
		testPackage      = "com.google.android.calculator"
		defaultUITimeout = time.Minute
	)

	packages := []string{testPackage}

	rl := &retry.Loop{Attempts: 1,
		MaxAttempts: 2,
		DoRetries:   true,
		Fatalf:      s.Fatalf,
		Logf:        s.Logf}

	creds, err := chrome.PickRandomCreds(s.RequiredVar(arcent.LoginPoolVar))
	if err != nil {
		rl.Exit("get login creds", err)
	}
	login := chrome.GAIALogin(creds)

	fdms, err := arcent.SetupPolicyServerWithArcApps(ctx, s.OutDir(), creds.User, packages, arcent.InstallTypeAvailable)
	if err != nil {
		rl.Exit("setup fake policy server", err)
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
			return rl.Retry("connect to Chrome", err)
		}
		defer cr.Close(ctx)

		tconn, err := cr.TestAPIConn(ctx)
		if err != nil {
			return rl.Retry("create test API connection", err)
		}

		a, err := arc.NewWithTimeout(ctx, s.OutDir(), bootTimeout)
		if err != nil {
			return rl.Retry("start ARC by policy", err)
		}
		defer a.Close(ctx)

		if err := arcent.ConfigureProvisioningLogs(ctx, a); err != nil {
			return rl.Exit("configure provisioning logs", err)
		}

		if err := arcent.WaitForProvisioning(ctx, a, rl.Attempts); err != nil {
			return rl.Retry("wait for provisioning", err)
		}

		if err := arcent.EnsurePlayStoreNotEmpty(ctx, tconn, cr, a, s.OutDir(), rl.Attempts); err != nil {
			return rl.Exit("verify Play Store is not empty", err)
		}

		if err := playstore.OpenAppPage(ctx, a, testPackage); err != nil {
			return rl.Exit("open app page", err)
		}

		d, err := a.NewUIDevice(ctx)
		if err != nil {
			return rl.Exit("initialize UI Automator: ", err)
		}
		defer d.Close(ctx)

		if installButton, err := arcent.WaitForInstallButton(ctx, d); err != nil {
			return rl.Exit("find the install button", err)
		} else if enabled, err := installButton.IsEnabled(ctx); err != nil {
			return rl.Exit("check install button state", err)
		} else if !enabled {
			return rl.Exit("verify install button is enabled", nil)
		}

		return nil
	}, nil); err != nil {
		s.Fatal("Allow install test failed: ", err)
	}
}
