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

	"chromiumos/tast/common/android/ui"
	"chromiumos/tast/common/policy"
	"chromiumos/tast/ctxutil"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/arc"
	"chromiumos/tast/local/arc/playstore"
	"chromiumos/tast/local/bundles/cros/enterprise/arcent"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/uiauto/faillog"
	"chromiumos/tast/local/policyutil"
	"chromiumos/tast/testing"
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
				Val:               withRetries,
			},
			{
				Name:              "vm",
				ExtraSoftwareDeps: []string{"android_vm", "no_qemu"},
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
		maxAttempts = 2
	)

	attempts := 1
	doRetries := s.Param().(bool)

	// Indicates a failure in the core feature under test so the polling should stop.
	exit := func(desc string, err error) error {
		s.Fatalf("Failed to %s: %v", desc, err)
		return nil
	}

	// Indicates that the error is retryable and unrelated to core feature under test.
	retryForAll := func(desc string, err error) error {
		if attempts < maxAttempts {
			attempts++
			err = errors.Wrap(err, "failed to "+desc)
			s.Logf("%s. Retrying", err)
			return err
		}
		return exit(desc, err)
	}

	// Indicates that the error is being retried only to stabilize the test temporarily.
	// TODO(b/242902484): Replace the calls to this with exit() when unstable variant is removed.
	retry := func(desc string, err error) error {
		if doRetries {
			return retryForAll(desc, err)
		}
		return exit(desc, err)
	}

	creds, err := chrome.PickRandomCreds(s.RequiredVar(arcent.LoginPoolVar))
	if err != nil {
		exit("get login creds", err)
	}
	login := chrome.GAIALogin(creds)

	packages := strings.Split(s.RequiredVar(packagesVar), ",")

	fdms, err := arcent.SetupPolicyServerWithArcApps(ctx, s.OutDir(), creds.User, packages, arcent.InstallTypeForceInstalled)
	if err != nil {
		exit("setup fake policy server", err)
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
			return retryForAll("connect to Chrome", err)
		}
		defer cr.Close(ctx)

		tconn, err := cr.TestAPIConn(ctx)
		if err != nil {
			return retryForAll("create test API connection", err)
		}

		// Ensure chrome://policy shows correct ArcEnabled and ArcPolicy values.
		if err := policyutil.Verify(ctx, tconn, []policy.Policy{&policy.ArcEnabled{Val: true}}); err != nil {
			return exit("verify ArcEnabled in policy", err)
		}

		if err := arcent.VerifyArcPolicyForceInstalled(ctx, tconn, packages); err != nil {
			return exit("verify force-installed apps", err)
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

		cleanupCtx := ctx
		ctx, cancel := ctxutil.Shorten(ctx, time.Minute)
		defer cancel()

		defer arcent.DumpBugReportOnError(cleanupCtx, func() bool {
			return s.HasError() || retErr != nil
		}, a, filepath.Join(s.OutDir(), fmt.Sprintf("bugreport_%d.zip", attempts)))

		// Note: if the user policy for the user is changed, the packages listed in
		// credentials files must be updated.
		if err := a.WaitForPackages(ctx, packages); err != nil {
			// TODO(b/242902484): Switch to exit when unstable variant is removed.
			return retry("wait for packages", err)
		}

		if err := ensurePackagesUninstallable(ctx, cr, a, packages); err != nil {
			return exit("verify packages are uninstallable", err)
		}

		if err := ensurePlayStoreNotEmpty(ctx, tconn, cr, a, s.OutDir(), attempts); err != nil {
			return exit("verify Play Store is not empty", err)
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

// ensurePlayStoreNotEmpty ensures that the asset browser does not display empty screen.
func ensurePlayStoreNotEmpty(ctx context.Context, tconn *chrome.TestConn, cr *chrome.Chrome, a *arc.ARC, outDir string, runID int) (retErr error) {
	const (
		searchBarTextStart = "Search for apps"
		emptyPlayStoreText = "No results found."
		serverErrorText    = "Server error|Error.*server.*"
		tryAgainButtonText = "Try again"
	)

	defer faillog.SaveScreenshotToFileOnError(ctx, cr, outDir, func() bool {
		return retErr != nil
	}, fmt.Sprintf("play_store_%d.png", runID))

	d, err := a.NewUIDevice(ctx)
	if err != nil {
		return errors.Wrap(err, "failed to initialize UI Automator")
	}
	defer d.Close(ctx)

	return testing.Poll(ctx, func(ctx context.Context) error {
		// if GMS Core updates after launch, it can cause Play Store to be closed so we have to
		// launch it again.
		act, err := launchAssetBrowserActivity(ctx, tconn, a)
		if err != nil {
			return err
		}
		defer act.Close()

		return testing.Poll(ctx, func(ctx context.Context) error {
			if running, err := act.IsRunning(ctx); err != nil {
				return testing.PollBreak(err)
			} else if !running {
				return testing.PollBreak(errors.New("Play Store closed"))
			}

			if err := d.Object(ui.Text(emptyPlayStoreText)).Exists(ctx); err == nil {
				return errors.New("Play Store is empty")
			}

			if err := playstore.FindAndDismissDialog(ctx, d, serverErrorText, tryAgainButtonText, 2*time.Second); err != nil {
				return testing.PollBreak(err)
			}

			if err := d.Object(ui.TextStartsWith(searchBarTextStart)).Exists(ctx); err != nil {
				return errors.Wrap(err, "Play Store UI screen not shown")
			}

			return nil
		}, &testing.PollOptions{Interval: 1 * time.Second, Timeout: 30 * time.Second})
	}, &testing.PollOptions{Interval: 10 * time.Second})
}

// launchAssetBrowserActivity starts the activity that displays the available apps.
func launchAssetBrowserActivity(ctx context.Context, tconn *chrome.TestConn, a *arc.ARC) (*arc.Activity, error) {
	const (
		playStorePackage     = "com.android.vending"
		assetBrowserActivity = "com.android.vending.AssetBrowserActivity"
	)

	testing.ContextLog(ctx, "Starting Asset Browser activity")
	act, err := arc.NewActivity(a, playStorePackage, assetBrowserActivity)
	if err != nil {
		return nil, errors.Wrap(err, "failed to create new activity")
	}
	if err := act.Start(ctx, tconn); err != nil {
		return nil, errors.Wrap(err, "failed starting Play Store")
	}

	return act, nil
}
