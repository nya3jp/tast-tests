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
	"chromiumos/tast/local/bundles/cros/enterprise/arcent"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/policyutil"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         ARCBlockedAppUninstall,
		LacrosStatus: testing.LacrosVariantUnneeded,
		Desc:         "Checks that blocked apps are removed if they are installed",
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

// ARCBlockedAppUninstall force-installs an app and ensures it is removed if blocked by policy.
func ARCBlockedAppUninstall(ctx context.Context, s *testing.State) {
	const (
		bootTimeout = 4 * time.Minute
		maxAttempts = 2
		testPackage = "com.google.android.calculator"
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

	fdms, err := arcent.SetupPolicyServerWithArcApps(ctx, s.OutDir(), creds.User, packages, arcent.InstallTypeForceInstalled)
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

		cleanupCtx := ctx
		ctx, cancel := ctxutil.Shorten(ctx, time.Minute)
		defer cancel()

		defer arcent.DumpBugReportOnError(cleanupCtx, func() bool {
			return s.HasError() || retErr != nil
		}, a, filepath.Join(s.OutDir(), fmt.Sprintf("bugreport_%d.zip", attempts)))

		if err := a.WaitForPackages(ctx, packages); err != nil {
			return retry("wait for packages", err)
		}

		s.Log("Changing the policy to block the installed app")
		arcPolicy := arcent.CreateArcPolicyWithApps(packages, arcent.InstallTypeBlocked)
		arcEnabledPolicy := &policy.ArcEnabled{Val: true}
		policies := []policy.Policy{arcEnabledPolicy, arcPolicy}

		if err := policyutil.ServeAndRefresh(ctx, fdms, cr, policies); err != nil {
			return exit("update policies", err)
		}

		s.Log("Waiting for packages to uninstall")
		for _, packageName := range packages {
			if err := waitForUninstall(ctx, a, packageName); err != nil {
				return exit("package not uninstalled", err)
			}
		}

		return nil
	}, nil); err != nil {
		s.Fatal("Provisioning flow failed: ", err)
	}
}

func waitForUninstall(ctx context.Context, a *arc.ARC, blockedPackage string) error {
	return testing.Poll(ctx, func(ctx context.Context) error {
		if installed, err := a.PackageInstalled(ctx, blockedPackage); err != nil {
			return testing.PollBreak(err)
		} else if installed {
			return errors.New("Package not yet uninstalled")
		}
		return nil
	}, &testing.PollOptions{Interval: 1 * time.Second})
}
