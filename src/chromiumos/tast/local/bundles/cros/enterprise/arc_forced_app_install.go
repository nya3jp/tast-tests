// Copyright 2021 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package enterprise

import (
	"context"
	"fmt"
	"path/filepath"
	"time"

	"chromiumos/tast/common/policy"
	"chromiumos/tast/ctxutil"
	"chromiumos/tast/local/arc"
	"chromiumos/tast/local/arc/arcent"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/policyutil"
	"chromiumos/tast/local/retry"
	"chromiumos/tast/testing"
)

const (
	withRetries    = true
	withoutRetries = false
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         ARCForcedAppInstall,
		LacrosStatus: testing.LacrosVariantUnneeded,
		Desc:         "Checks that ARC is launched when policy is set",
		Contacts:     []string{"mhasank@chromium.org", "arc-commercial@google.com"},
		Attr:         []string{"group:mainline"},
		SoftwareDeps: []string{"chrome", "play_store"},
		Timeout:      15 * time.Minute,
		VarDeps: []string{
			arcent.LoginPoolVar,
		},
		Params: []testing.Param{
			{
				ExtraSoftwareDeps: []string{"android_p", "no_qemu"},
				Val:               withoutRetries,
			},
			{
				Name:              "vm",
				ExtraSoftwareDeps: []string{"android_vm", "no_qemu"},
				Val:               withoutRetries,
			},
			{
				Name:              "betty",
				ExtraSoftwareDeps: []string{"android_p", "qemu"},
				Val:               withRetries,
				ExtraAttr:         []string{"informational"},
			},
			{
				Name:              "vm_betty",
				ExtraSoftwareDeps: []string{"android_vm", "qemu"},
				Val:               withRetries,
				ExtraAttr:         []string{"informational"},
			}},
	})
}

// ARCForcedAppInstall runs the app force install test:
// - login with managed account,
// - check that ARC is launched by user policy,
// - check that chrome://policy page shows ArcEnabled and ArcPolicy force-installed apps list,
// - check that force-installed by policy Android packages are installed,
// - check that force-installed Android packages cannot be uninstalled.
func ARCForcedAppInstall(ctx context.Context, s *testing.State) {
	const (
		bootTimeout = 4 * time.Minute
		testPackage = "com.google.android.calculator"
	)

	rl := &retry.Loop{Attempts: 1,
		MaxAttempts: 2,
		DoRetries:   s.Param().(bool),
		Fatalf:      s.Fatalf,
		Logf:        s.Logf}

	creds, err := chrome.PickRandomCreds(s.RequiredVar(arcent.LoginPoolVar))
	if err != nil {
		rl.Exit("get login creds", err)
	}
	login := chrome.GAIALogin(creds)

	packages := []string{testPackage}

	cleanupCtx := ctx
	ctx, cancel := ctxutil.Shorten(ctx, time.Minute)
	defer cancel()

	fdms, err := arcent.SetupPolicyServerWithArcApps(ctx, s.OutDir(), creds.User, packages, arcent.InstallTypeForceInstalled)
	if err != nil {
		rl.Exit("setup fake policy server", err)
	}
	defer fdms.Stop(cleanupCtx)

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
		defer cr.Close(cleanupCtx)

		tconn, err := cr.TestAPIConn(ctx)
		if err != nil {
			return rl.Retry("create test API connection", err)
		}

		if err := policyutil.Verify(ctx, tconn, []policy.Policy{&policy.ArcEnabled{Val: true}}); err != nil {
			return rl.Exit("verify ArcEnabled in policy", err)
		}

		if err := arcent.VerifyArcPolicyForceInstalled(ctx, tconn, packages); err != nil {
			return rl.Exit("verify force-installed apps", err)
		}

		a, err := arc.NewWithTimeout(ctx, s.OutDir(), bootTimeout)
		if err != nil {
			return rl.Exit("start ARC by policy", err)
		}
		defer a.Close(cleanupCtx)

		if err := arcent.ConfigureProvisioningLogs(ctx, a); err != nil {
			return rl.Exit("configure provisioning logs", err)
		}

		if err := arcent.WaitForProvisioning(ctx, a, rl.Attempts); err != nil {
			return rl.Exit("wait for provisioning", err)
		}

		defer arcent.DumpBugReportOnError(cleanupCtx, func() bool {
			return s.HasError() || retErr != nil
		}, a, filepath.Join(s.OutDir(), fmt.Sprintf("bugreport_%d.zip", rl.Attempts)))

		installCtx, cancel := context.WithTimeout(ctx, arcent.InstallTimeout)
		defer cancel()
		if err := a.WaitForPackages(installCtx, packages); err != nil {
			return rl.Exit("wait for packages", err)
		}

		if err := arcent.EnsurePackagesUninstall(ctx, cr, a, packages, false); err != nil {
			return rl.Exit("verify packages are uninstallable", err)
		}

		if err := arcent.EnsurePlayStoreNotEmpty(ctx, tconn, cr, a, s.OutDir(), rl.Attempts); err != nil {
			return rl.Exit("verify Play Store is not empty", err)
		}

		return nil
	}, nil); err != nil {
		s.Fatal("Provisioning flow failed: ", err)
	}
}
