// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package enterprise

import (
	"context"
	"os"
	"path/filepath"
	"regexp"
	"sort"
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
	"chromiumos/tast/local/chrome/ash"
	"chromiumos/tast/local/chrome/uiauto/faillog"
	"chromiumos/tast/local/cryptohome"
	"chromiumos/tast/local/policyutil"
	"chromiumos/tast/local/syslog"
	"chromiumos/tast/testing"
	"chromiumos/tast/timing"
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
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"chrome"},
		VarDeps: []string{
			loginPoolVar,
			packagesVar,
		},
		Params: []testing.Param{
			{
				ExtraSoftwareDeps: []string{"android_p"},
				Val:               withRetries,
				Timeout:           15 * time.Minute,
			},
			{
				Name:              "vm",
				ExtraSoftwareDeps: []string{"android_vm"},
				Val:               withRetries,
				Timeout:           15 * time.Minute,
			},
			{
				Name:              "unstable",
				ExtraSoftwareDeps: []string{"android_p"},
				Val:               withoutRetries,
				Timeout:           10 * time.Minute,
			},
			{
				Name:              "vm_unstable",
				ExtraSoftwareDeps: []string{"android_vm"},
				Val:               withoutRetries,
				Timeout:           10 * time.Minute,
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
		// CloudDPC sign-in timeout set in code is 3 minutes.
		timeoutWaitForPlayStore = 3 * time.Minute
		maxAttempts             = 2
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
		if doRetries && attempts < maxAttempts {
			attempts++
			err = errors.Wrap(err, "failed to "+desc)
			s.Logf("%s. Retrying", err)
			return err
		}
		return exit(desc, err)
	}

	login := chrome.GAIALoginPool(s.RequiredVar(loginPoolVar))
	packages := strings.Split(s.RequiredVar(packagesVar), ",")

	if err := testing.Poll(ctx, func(ctx context.Context) error {
		// Log-in to Chrome and allow to launch ARC if allowed by user policy.
		cr, err := chrome.New(
			ctx,
			login,
			chrome.ARCSupported(),
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
			return retry("start ARC by policy", err)
		}
		defer a.Close(ctx)

		if err := a.EnableVerboseLogging(ctx, "Finsky", "Volley", "PlayCommon"); err != nil {
			return exit("enable verbose logging", err)
		}

		// Increase the logcat buffer size to 10MB.
		if err := a.Command(ctx, "logcat", "-G", "10M").Run(testexec.DumpLogOnError); err != nil {
			return exit("increase logcat buffer size", err)
		}

		sysLogReader, err := openSysLog(ctx)
		if err != nil {
			return exit("initialize syslog reader", err)
		}
		defer sysLogReader.Close()

		if err := optin.LaunchAndWaitForPlayStore(ctx, tconn, cr, timeoutWaitForPlayStore); err != nil {
			if err := optin.DumpLogCat(ctx, strconv.Itoa(attempts)); err != nil {
				s.Logf("WARNING: Failed to dump logcat: %s", err)
			}
			return retry("launch Play Store", err)
		}

		cleanupCtx := ctx
		ctx, cancel := ctxutil.Shorten(ctx, 60*time.Second)
		defer cancel()
		defer dumpBugReportOnError(cleanupCtx, s.HasError, a, filepath.Join(s.OutDir(), "bugreport.zip"))

		// Note: if the user policy for the user is changed, the packages listed in
		// credentials files must be updated.
		if err := a.WaitForPackages(ctx, packages); err != nil {
			if chromeCrashed(ctx, sysLogReader) {
				return retry("wait for packages", errors.Wrap(err, "Chrome process crashed"))
			}
			return exit("wait for packages", err)
		}

		// Ensure that Andriod packages are set as not-uninstallable by ARC policy.
		testing.ContextLog(ctx, "Waiting for packages being marked as not uninstallable")
		if err := waitForBlockUninstall(ctx, cr, a, packages); err != nil {
			return exit("verify packages as marked uninstallable", err)
		}

		if err := ensurePlayStoreNotEmpty(ctx, tconn, a); err != nil {
			faillog.DumpUITree(cleanupCtx, s.OutDir(), tconn)
			return exit("verify Play Store is not empty", err)
		}

		// Delaying the validation of uninstall after Play Store UI has loaded to ensure
		// that the configuration is loaded by Package Manager.
		if err := ensurePackagesUninstallable(ctx, cr, a, packages); err != nil {
			return exit("verify packages are uninstallable", err)
		}

		return nil
	}, nil); err != nil {
		s.Fatal("Provisioning flow failed: ", err)
	}
}

func openSysLog(ctx context.Context) (*syslog.LineReader, error) {
	return syslog.NewLineReader(ctx, syslog.MessageFile, false /*fromStart*/, nil /*opts*/)
}

func chromeCrashed(ctx context.Context, sysLogReader *syslog.LineReader) bool {
	const chromeCrashMessage = "Received crash notification for chrome"
	for {
		line, err := sysLogReader.ReadLine()
		if err != nil {
			return false
		} else if strings.Contains(line, chromeCrashMessage) {
			return true
		}
	}
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
func ensurePlayStoreNotEmpty(ctx context.Context, tconn *chrome.TestConn, a *arc.ARC) error {
	const (
		searchBarTextStart = "Search for apps"
		emptyPlayStoreText = "No results found."
		serverErrorText    = "Server error|Error.*server.*"
		tryAgainButtonText = "Try again"
	)

	d, err := a.NewUIDevice(ctx)
	if err != nil {
		return errors.Wrap(err, "failed to initialize UI Automator")
	}
	defer d.Close(ctx)

	return testing.Poll(ctx, func(ctx context.Context) error {
		// TODO(b/231751280): View does not update when app is installed.
		// When the bug is fixed and Play Store version is upreved on the
		// system image, move this out of the loop.
		if err := launchAssetBrowserActivity(ctx, tconn, a); err != nil {
			return testing.PollBreak(err)
		}

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
		}, &testing.PollOptions{Interval: 1 * time.Second, Timeout: 5 * time.Second})
	}, &testing.PollOptions{Timeout: 60 * time.Second})
}

// launchAssetBrowserActivity starts the activity that displays the available apps.
func launchAssetBrowserActivity(ctx context.Context, tconn *chrome.TestConn, a *arc.ARC) error {
	const (
		playStorePackage     = "com.android.vending"
		assetBrowserActivity = "com.android.vending.AssetBrowserActivity"
	)

	if err := optin.ClosePlayStore(ctx, tconn); err != nil {
		return errors.Wrap(err, "failed to close Play Store")
	}

	ash.WaitForHidden(ctx, tconn, playStorePackage)

	testing.ContextLog(ctx, "Starting Asset Browser activity")
	act, err := arc.NewActivity(a, playStorePackage, assetBrowserActivity)
	if err != nil {
		return errors.Wrap(err, "failed to create new activity")
	}
	if err := act.StartWithDefaultOptions(ctx, tconn); err != nil {
		return errors.Wrap(err, "failed starting Play Store")
	}

	return nil
}

// readPackageRestrictions reads content of package restrictions file.
func readPackageRestrictions(ctx context.Context, a *arc.ARC, cr *chrome.Chrome) ([]byte, error) {
	const packageRestrictionsPath = "/data/system/users/0/package-restrictions.xml"

	// Cryptohome dir for the current user.
	rootCryptDir, err := cryptohome.SystemPath(ctx, cr.User())
	if err != nil {
		return nil, errors.Wrap(err, "failed to get the cryptohome directory for the user")
	}

	// android-data dir under the cryptohome dir (/home/root/${USER_HASH}/android-data)
	androidDataDir := filepath.Join(rootCryptDir, "android-data")

	return a.ReadXMLFile(ctx, filepath.Join(androidDataDir, packageRestrictionsPath))
}

// waitForBlockUninstall waits for Android packages to be set as not uninstallable.
func waitForBlockUninstall(ctx context.Context, cr *chrome.Chrome, a *arc.ARC, packages []string) error {
	ctx, st := timing.Start(ctx, "wait_block_packages")
	defer st.End()

	return testing.Poll(ctx, func(ctx context.Context) error {
		out, err := readPackageRestrictions(ctx, a, cr)
		if err != nil {
			if errors.Is(err, os.ErrNotExist) {
				return errors.Wrap(err, "package-restrictions.xml does not exist yet")
			}
			return testing.PollBreak(errors.Wrap(err, "failed to read package-restrictions.xml"))
		}

		r := regexp.MustCompile(`<block-uninstall packageName="(.*)" />`)
		matches := r.FindAllStringSubmatch(string(out), -1)
		if matches == nil {
			return errors.New("no package marked as block uninstall yet")
		}

		// We must wait for all packages being blocked at the same time. Otherwise
		// previously blocked packages will be able to be uninstalled in a short period.
		notBlockedPackages := make(map[string]bool)
		for _, p := range packages {
			notBlockedPackages[p] = true
		}

		for _, m := range matches {
			packageName := m[1]
			if notBlockedPackages[packageName] {
				delete(notBlockedPackages, packageName)
			}
		}
		if len(notBlockedPackages) != 0 {
			return errors.Errorf("%d package(s) are not blocked yet: %s",
				len(notBlockedPackages),
				strings.Join(makeList(notBlockedPackages), ", "))
		}
		return nil
	}, nil)
}

// makeList returns a list of keys from map.
func makeList(packages map[string]bool) []string {
	var packagesList []string
	for pkg := range packages {
		packagesList = append(packagesList, pkg)
	}
	sort.Strings(packagesList)
	return packagesList
}
