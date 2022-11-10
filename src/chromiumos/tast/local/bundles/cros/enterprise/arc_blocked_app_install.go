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
		Func:         ARCBlockedAppInstall,
		LacrosStatus: testing.LacrosVariantUnneeded,
		Desc:         "Checks that blocked apps cannot be installed in Play Store",
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

// ARCBlockedAppInstall Verifies that blocked app cannot be installed.
func ARCBlockedAppInstall(ctx context.Context, s *testing.State) {
	const (
		bootTimeout           = 4 * time.Minute
		installButtonText     = "install"
		testPackage           = "com.google.android.calculator"
		defaultUITimeout      = 1 * time.Minute
		appUnavailableMessage = "Your administrator has not given you access to this item."
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

	arcPolicy := arcent.CreateArcPolicyWithApps(packages, arcent.InstallTypeBlocked)
	arcPolicy.Val.PlayStoreMode = arcent.PlayStoreModeBlockList
	arcEnabledPolicy := &policy.ArcEnabled{Val: true}
	policies := []policy.Policy{arcEnabledPolicy, arcPolicy}

	cleanupCtx := ctx
	ctx, cancel := ctxutil.Shorten(ctx, time.Minute)
	defer cancel()

	fdms, err := policyutil.SetUpFakePolicyServer(ctx, s.OutDir(), creds.User, policies)
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

		a, err := arc.NewWithTimeout(ctx, s.OutDir(), bootTimeout)
		if err != nil {
			return rl.Exit("start ARC by policy", err)
		}
		defer a.Close(cleanupCtx)

		if err := arcent.ConfigureProvisioningLogs(ctx, a); err != nil {
			return rl.Exit("configure provisioning logs", err)
		}

		defer arcent.DumpBugReportOnError(cleanupCtx, func() bool {
			return s.HasError() || retErr != nil
		}, a, filepath.Join(s.OutDir(), fmt.Sprintf("bugreport_%d.zip", rl.Attempts)))

		if err := arcent.WaitForProvisioning(ctx, a, rl.Attempts); err != nil {
			return rl.Exit("wait for provisioning", err)
		}

		defer a.DumpUIHierarchyOnError(cleanupCtx, s.OutDir(), func() bool {
			return s.HasError() || retErr != nil
		})

		if err := arcent.EnsurePlayStoreNotEmpty(ctx, tconn, cr, a, s.OutDir(), rl.Attempts); err != nil {
			return rl.Exit("verify Play Store is not empty", err)
		}

		d, err := a.NewUIDevice(ctx)
		if err != nil {
			return rl.Exit("initialize UI Automator", err)
		}
		defer d.Close(cleanupCtx)

		if err := arcent.PollAppPageState(ctx, tconn, a, testPackage, func(ctx context.Context) error {
			if err := arcent.WaitForAppUnavailableMessage(ctx, d, time.Minute); err == nil {
				return nil
			}

			return errors.New("App unavailable message not found")
		}, 5*time.Minute); err != nil {
			return rl.Exit("confirm unavailability", err)
		}

		return nil
	}, nil); err != nil {
		rl.Exit("verify blocked app cannot be installed: ", err)
	}
}
