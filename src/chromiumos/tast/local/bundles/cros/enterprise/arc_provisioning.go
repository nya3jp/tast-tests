// Copyright 2021 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package enterprise

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"
	"time"

	"chromiumos/tast/common/policy"
	"chromiumos/tast/ctxutil"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/arc"
	"chromiumos/tast/local/bundles/cros/enterprise/arcent"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/policyutil"
	"chromiumos/tast/local/retry"
	"chromiumos/tast/testing"
	"chromiumos/tast/testing/hwdep"
)

const (
	packagesVar    = "enterprise.ARCProvisioning.packages"
	withRetries    = true
	withoutRetries = false
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         ARCProvisioning,
		LacrosStatus: testing.LacrosVariantUnneeded,
		Desc:         "Checks that ARC is launched when policy is set",
		Contacts:     []string{"mhasank@chromium.org", "arc-commercial@google.com"},
		Attr:         []string{"group:mainline"},
		SoftwareDeps: []string{"chrome", "play_store"},
		Timeout:      15 * time.Minute,
		VarDeps: []string{
			arcent.LoginPoolVar,
			packagesVar,
		},
		Params: []testing.Param{
			{
				ExtraSoftwareDeps: []string{"android_p", "no_qemu"},
				// TODO(b/254838300): Memory pressure on kukui-arc-r causes test to fail.
				ExtraHardwareDeps: hwdep.D(hwdep.SkipOnModel("kakadu", "katsu", "kodama", "krane")),
				Val:               withRetries,
			},
			{
				Name:              "vm",
				ExtraSoftwareDeps: []string{"android_vm", "no_qemu"},
				// TODO(b/254838300): Memory pressure on kukui-arc-r causes test to fail.
				ExtraHardwareDeps: hwdep.D(hwdep.SkipOnModel("kakadu", "katsu", "kodama", "krane")),
				Val:               withRetries,
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
			},
			{
				Name:              "unstable",
				ExtraSoftwareDeps: []string{"android_p"},
				Val:               withoutRetries,
				ExtraAttr:         []string{"informational"},
			},
			{
				Name:              "vm_unstable",
				ExtraSoftwareDeps: []string{"android_vm"},
				Val:               withoutRetries,
				ExtraAttr:         []string{"informational"},
			}},
	})
}

// ARCProvisioning runs the provisioning smoke test:
// - login with managed account,
// - check that ARC is launched by user policy,
// - check that chrome://policy page shows ArcEnabled and ArcPolicy force-installed apps list,
// - check that force-installed by policy Android packages are installed,
// - check that force-installed Android packages cannot be uninstalled.
func ARCProvisioning(ctx context.Context, s *testing.State) {
	const (
		bootTimeout = 4 * time.Minute
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

	packages := strings.Split(s.RequiredVar(packagesVar), ",")

	fdms, err := arcent.SetupPolicyServerWithArcApps(ctx, s.OutDir(), creds.User, packages, arcent.InstallTypeForceInstalled)
	if err != nil {
		rl.Exit("setup fake policy server", err)
	}
	defer fdms.Stop(ctx)

	if err := testing.Poll(ctx, func(ctx context.Context) (retErr error) {
		// Log-in to Chrome and allow to launch ARC if allowed by user policy.
		cr, err := chrome.New(
			ctx,
			login,
			chrome.ARCSupported(),
			chrome.UnRestrictARCCPU(),
			chrome.DMSPolicy(fdms.URL),
			chrome.ExtraArgs(arc.DisableSyncFlags()...))
		if err != nil {
			return rl.RetryForAll("connect to Chrome", err)
		}
		defer cr.Close(ctx)

		tconn, err := cr.TestAPIConn(ctx)
		if err != nil {
			return rl.RetryForAll("create test API connection", err)
		}

		// Ensure chrome://policy shows correct ArcEnabled and ArcPolicy values.
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
		defer a.Close(ctx)

		if err := arcent.ConfigureProvisioningLogs(ctx, a); err != nil {
			return rl.Exit("configure provisioning logs", err)
		}

		if err := arcent.WaitForProvisioning(ctx, a, rl.Attempts); err != nil {
			return rl.Exit("wait for provisioning", err)
		}

		cleanupCtx := ctx
		ctx, cancel := ctxutil.Shorten(ctx, time.Minute)
		defer cancel()

		defer arcent.DumpBugReportOnError(cleanupCtx, func() bool {
			return s.HasError() || retErr != nil
		}, a, filepath.Join(s.OutDir(), fmt.Sprintf("bugreport_%d.zip", rl.Attempts)))

		// Note: if the user policy for the user is changed, the packages listed in
		// credentials files must be updated.
		if err := a.WaitForPackages(ctx, packages); err != nil {
			// TODO(b/242902484): Switch to exit when unstable variant is removed.
			return rl.Retry("wait for packages", err)
		}

		if err := ensurePackagesUninstallable(ctx, cr, a, packages); err != nil {
			return rl.Exit("verify packages are uninstallable", err)
		}

		if err := arcent.EnsurePlayStoreNotEmpty(ctx, tconn, cr, a, s.OutDir(), attempts); err != nil {
			return rl.Exit("verify Play Store is not empty", err)
		}

		return nil
	}, nil); err != nil {
		s.Fatal("Provisioning flow failed: ", err)
	}
}

// ensurePackagesUninstallable verifies that force-installed packages can't be uninstalled
func ensurePackagesUninstallable(ctx context.Context, cr *chrome.Chrome, a *arc.ARC, packages []string) error {
	// Try uninstalling packages with ADB, should fail.
	testing.ContextLog(ctx, "Trying to uninstall packages")
	for _, p := range packages {
		if a.Uninstall(ctx, p) == nil {
			return errors.Errorf("Package %q can be uninstalled", p)
		}
	}

	return nil
}
