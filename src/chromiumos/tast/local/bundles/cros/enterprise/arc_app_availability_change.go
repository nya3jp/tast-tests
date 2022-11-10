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
	"chromiumos/tast/errors"
	"chromiumos/tast/local/arc"
	"chromiumos/tast/local/arc/arcent"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/policyutil"
	"chromiumos/tast/local/retry"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         ARCAppAvailabilityChange,
		LacrosStatus: testing.LacrosVariantUnneeded,
		Desc:         "Checks that an app availability change is reflected in Play Store",
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

// ARCAppAvailabilityChange verifies that app availability change is reflected in the Play Store.
func ARCAppAvailabilityChange(ctx context.Context, s *testing.State) {
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

	creds, err := chrome.PickRandomCreds(s.RequiredVar(arcent.LoginPoolVar))
	if err != nil {
		rl.Exit("get login creds", err)
	}
	login := chrome.GAIALogin(creds)

	cleanupCtx := ctx
	ctx, cancel := ctxutil.Shorten(ctx, time.Minute)
	defer cancel()

	fdms, err := arcent.SetupPolicyServerWithArcApps(ctx, s.OutDir(), creds.User, packages, arcent.InstallTypeAvailable)
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

		if err := arcent.WaitForProvisioning(ctx, a, rl.Attempts); err != nil {
			return rl.Exit("wait for provisioning", err)
		}

		defer arcent.DumpBugReportOnError(cleanupCtx, func() bool {
			return s.HasError() || retErr != nil
		}, a, filepath.Join(s.OutDir(), fmt.Sprintf("bugreport_%d.zip", rl.Attempts)))

		d, err := a.NewUIDevice(ctx)
		if err != nil {
			return rl.Exit("created UIAutomator", err)
		}
		defer d.Close(cleanupCtx)

		tconn, err := cr.TestAPIConn(ctx)
		if err != nil {
			rl.Exit("create test API Connection", err)
		}

		if err := arcent.PollAppPageState(ctx, tconn, a, testPackage, func(ctx context.Context) error {
			installButton, err := arcent.WaitForInstallButton(ctx, d)
			if err != nil {
				return errors.Wrap(err, "failed to find the install button")
			}

			enabled, err := installButton.IsEnabled(ctx)
			if err != nil {
				return errors.Wrap(err, "failed to check the install button state")
			}

			if !enabled {
				return errors.New("install button is disabled")
			}

			return nil
		}, time.Minute); err != nil {
			rl.Exit("confirm availability", err)
		}

		s.Log("Changing the policy to block the installed app")
		arcPolicy := arcent.CreateArcPolicyWithApps(packages, arcent.InstallTypeBlocked)
		arcEnabledPolicy := &policy.ArcEnabled{Val: true}
		policies := []policy.Policy{arcEnabledPolicy, arcPolicy}

		if err := policyutil.ServeAndRefresh(ctx, fdms, cr, policies); err != nil {
			return rl.Exit("update policies", err)
		}

		if err := arcent.PollAppPageState(ctx, tconn, a, testPackage, func(ctx context.Context) error {
			// UIAutomator connection breaks due to policy change causing automator to be uninstalled.
			d, err := a.NewUIDevice(ctx)
			if err != nil {
				return testing.PollBreak(err)
			}
			defer d.Close(ctx)

			if err := arcent.WaitForAppUnavailableMessage(ctx, d, time.Minute); err == nil {
				return nil
			}

			return errors.New("App unavailable message not found")
		}, 5*time.Minute); err != nil {
			return rl.Exit("confirm unavailability", err)
		}

		return nil
	}, nil); err != nil {
		s.Fatal("Availability transition test failed: ", err)
	}
}
