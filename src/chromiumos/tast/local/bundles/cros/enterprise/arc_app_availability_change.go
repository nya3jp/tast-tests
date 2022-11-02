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
	"chromiumos/tast/local/arc/optin"
	"chromiumos/tast/local/arc/playstore"
	"chromiumos/tast/local/bundles/cros/enterprise/arcent"
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
			return rl.Exit("initialize UI Automator", err)
		}
		defer d.Close(cleanupCtx)

		if err := playstore.OpenAppPage(ctx, a, testPackage); err != nil {
			return rl.Exit("open app page", err)
		}

		if installButton, err := arcent.WaitForInstallButton(ctx, d); err != nil {
			return rl.Exit("find the install button", err)
		} else if enabled, err := installButton.IsEnabled(ctx); err != nil {
			return rl.Exit("check install button state", err)
		} else if !enabled {
			return rl.Exit("verify install button is enabled", nil)
		}

		s.Log("Changing the policy to block the installed app")
		arcPolicy := arcent.CreateArcPolicyWithApps(packages, arcent.InstallTypeBlocked)
		arcEnabledPolicy := &policy.ArcEnabled{Val: true}
		policies := []policy.Policy{arcEnabledPolicy, arcPolicy}

		if err := policyutil.ServeAndRefresh(ctx, fdms, cr, policies); err != nil {
			return rl.Exit("update policies", err)
		}

		tconn, err := cr.TestAPIConn(ctx)

		if err := testing.Poll(ctx, func(ctx context.Context) error {
			if err := arcent.WaitForAppUnavailableMessage(ctx, d, time.Minute); err == nil {
				return nil
			}

			optin.ClosePlayStore(ctx, tconn)

			if err := playstore.OpenAppPage(ctx, a, testPackage); err != nil {
				return testing.PollBreak(err)
			}

			return errors.New("App unavailable message not found")
		}, &testing.PollOptions{Timeout: 5 * time.Minute}); err != nil {
			return testing.PollBreak(err)
		}

		return nil
	}, nil); err != nil {
		s.Fatal("Availability transition test failed: ", err)
	}
}
