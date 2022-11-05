// Copyright 2022 The ChromiumOS Authors
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
	"chromiumos/tast/local/bundles/cros/enterprise/arcent"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/policyutil"
	"chromiumos/tast/local/retry"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         ARCAvailableAppUninstall,
		LacrosStatus: testing.LacrosVariantUnneeded,
		Desc:         "Checks that an available app can be uninstalled",
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

// ARCAvailableAppUninstall verifies that an available app can be uninstalled.
func ARCAvailableAppUninstall(ctx context.Context, s *testing.State) {
	const (
		bootTimeout = 4 * time.Minute
		testPackage = "com.google.android.calculator"
	)

	rl := &retry.Loop{Attempts: 1,
		MaxAttempts: 2,
		DoRetries:   true,
		Fatalf:      s.Fatalf,
		Logf:        s.Logf}

	packages := []string{testPackage}
	attempts := 1

	creds, err := chrome.PickRandomCreds(s.RequiredVar(arcent.LoginPoolVar))
	if err != nil {
		rl.Exit("get login creds", err)
	}
	login := chrome.GAIALogin(creds)

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

		a, err := arc.NewWithTimeout(ctx, s.OutDir(), bootTimeout)
		if err != nil {
			return rl.Exit("start ARC by policy", err)
		}
		defer a.Close(cleanupCtx)

		if err := arcent.ConfigureProvisioningLogs(ctx, a); err != nil {
			return rl.Exit("configure provisioning logs", err)
		}

		if err := arcent.WaitForProvisioning(ctx, a, attempts); err != nil {
			return rl.Retry("wait for provisioning", err)
		}

		installCtx, cancel := context.WithTimeout(ctx, arcent.InstallTimeout)
		defer cancel()
		if err := a.WaitForPackages(installCtx, packages); err != nil {
			return rl.Retry("wait for packages", err)
		}

		defer arcent.DumpBugReportOnError(cleanupCtx, func() bool {
			return s.HasError() || retErr != nil
		}, a, filepath.Join(s.OutDir(), fmt.Sprintf("bugreport_%d.zip", rl.Attempts)))

		s.Log("Changing the policy to make the app available")
		arcPolicy := arcent.CreateArcPolicyWithApps(packages, arcent.InstallTypeAvailable)
		arcEnabledPolicy := &policy.ArcEnabled{Val: true}
		policies := []policy.Policy{arcEnabledPolicy, arcPolicy}

		if err := policyutil.ServeAndRefresh(ctx, fdms, cr, policies); err != nil {
			return rl.Exit("update policies", err)
		}

		return testing.Poll(ctx, func(ctx context.Context) error {
			return arcent.EnsurePackagesUninstall(ctx, cr, a, packages, true)
		}, &testing.PollOptions{Timeout: 60 * time.Second, Interval: 1 * time.Second})
	}, nil); err != nil {
		s.Fatal("Availability transition test failed: ", err)
	}
}
