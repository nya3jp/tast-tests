// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package enterprise

import (
	"context"
	"fmt"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"chromiumos/tast/common/android/ui"
	"chromiumos/tast/common/policy"
	"chromiumos/tast/common/testexec"
	"chromiumos/tast/ctxutil"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/arc"
	"chromiumos/tast/local/arc/optin"
	"chromiumos/tast/local/arc/playstore"
	"chromiumos/tast/local/bundles/cros/enterprise/arcent"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/uiauto/faillog"
	"chromiumos/tast/local/policyutil"
	"chromiumos/tast/testing"
)

const (
	loginPoolVar   = "arc.managedAccountPool"
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
			loginPoolVar,
			packagesVar,
		},
		// "no_qemu" is added for excluding betty from the target board list.
		// TODO(b/191102176): Remove "no_qemu" after making the test pass on betty.
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
	retry := func(desc string, err error) error {
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
	tempRetry := func(desc string, err error) error {
		if doRetries {
			return retry(desc, err)
		}
		return exit(desc, err)
	}

	login := chrome.GAIALoginPool(s.RequiredVar(loginPoolVar))
	packages := strings.Split(s.RequiredVar(packagesVar), ",")

	if err := testing.Poll(ctx, func(ctx context.Context) (retErr error) {
		// Log-in to Chrome and allow to launch ARC if allowed by user policy.
		cr, err := chrome.New(
			ctx,
			login,
			chrome.ARCSupported(),
			chrome.UnRestrictARCCPU(),
			chrome.ProdPolicy(),
			chrome.ExtraArgs(arc.DisableSyncFlags()...))
		if err != nil {
			return retry("connect to Chrome", err)
		}
		defer cr.Close(ctx)

		tconn, err := cr.TestAPIConn(ctx)
		if err != nil {
			return retry("create test API connection", err)
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
			// TODO(b/242902484): Switch to exit when unstable variant is removed.
			return tempRetry("start ARC by policy", err)
		}
		defer a.Close(ctx)

		if err := enableVerboseLogging(ctx, a); err != nil {
			return exit("enable verbose logging", err)
		}

		// Increase the logcat buffer size to 10MB.
		if err := increaseLogcatBufferSize(ctx, a); err != nil {
			return exit("increase logcat buffer size", err)
		}

		if err := waitForProvisioning(ctx, a, attempts); err != nil {
			// TODO(b/242902484): Switch to exit when unstable variant is removed.
			return tempRetry("wait for provisioning", err)
		}

		cleanupCtx := ctx
		ctx, cancel := ctxutil.Shorten(ctx, time.Minute)
		defer cancel()

		defer dumpBugReportOnError(cleanupCtx, func() bool {
			return s.HasError() || retErr != nil
		}, a, filepath.Join(s.OutDir(), fmt.Sprintf("bugreport_%d.zip", attempts)))

		// Note: if the user policy for the user is changed, the packages listed in
		// credentials files must be updated.
		if err := a.WaitForPackages(ctx, packages); err != nil {
			return exit("wait for packages", err)
		}

		if err := ensurePackagesUninstallable(ctx, cr, a, packages); err != nil {
			return exit("verify packages are uninstallable", err)
		}

		if err := ensurePlayStoreNotEmpty(ctx, tconn, cr, a, s.OutDir(), attempts); err != nil {
			// TODO(b/242902484): Switch to exit when unstable variant is removed.
			return tempRetry("verify Play Store is not empty", err)
		}

		return nil
	}, nil); err != nil {
		s.Fatal("Provisioning flow failed: ", err)
	}
}

func enableVerboseLogging(ctx context.Context, a *arc.ARC) error {
	verboseTags := []string{"clouddpc", "Finsky", "Volley", "PlayCommon"}
	return a.EnableVerboseLogging(ctx, verboseTags...)
}

func increaseLogcatBufferSize(ctx context.Context, a *arc.ARC) error {
	return a.Command(ctx, "logcat", "-G", "10M").Run(testexec.DumpLogOnError)
}

func waitForProvisioning(ctx context.Context, a *arc.ARC, attempt int) error {
	// CloudDPC sign-in timeout set in code is 3 minutes.
	const provisioningTimeout = 3 * time.Minute

	if err := a.WaitForProvisioning(ctx, provisioningTimeout); err != nil {
		if err := optin.DumpLogCat(ctx, strconv.Itoa(attempt)); err != nil {
			testing.ContextLogf(ctx, "WARNING: Failed to dump logcat: %s", err)
		}
		return err
	}
	return nil

}

func dumpBugReportOnError(ctx context.Context, hasError func() bool, a *arc.ARC, filePath string) {
	if !hasError() {
		return
	}

	testing.ContextLog(ctx, "Dumping Bug Report")
	if err := a.BugReport(ctx, filePath); err != nil {
		testing.ContextLog(ctx, "Failed to get bug report: ", err)
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

	if err := launchAssetBrowserActivity(ctx, tconn, a); err != nil {
		return err
	}

	d, err := a.NewUIDevice(ctx)
	if err != nil {
		return errors.Wrap(err, "failed to initialize UI Automator")
	}
	defer d.Close(ctx)

	return testing.Poll(ctx, func(ctx context.Context) error {
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
}

// launchAssetBrowserActivity starts the activity that displays the available apps.
func launchAssetBrowserActivity(ctx context.Context, tconn *chrome.TestConn, a *arc.ARC) error {
	const (
		playStorePackage     = "com.android.vending"
		assetBrowserActivity = "com.android.vending.AssetBrowserActivity"
	)

	testing.ContextLog(ctx, "Starting Asset Browser activity")
	act, err := arc.NewActivity(a, playStorePackage, assetBrowserActivity)
	if err != nil {
		return errors.Wrap(err, "failed to create new activity")
	}
	if err := act.Start(ctx, tconn); err != nil {
		return errors.Wrap(err, "failed starting Play Store")
	}

	return nil
}
