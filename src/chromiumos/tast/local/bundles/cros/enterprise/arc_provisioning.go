// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package enterprise

import (
	"context"
	"io/ioutil"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"

	"chromiumos/tast/common/android/ui"
	"chromiumos/tast/common/policy"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/arc"
	"chromiumos/tast/local/arc/optin"
	"chromiumos/tast/local/bundles/cros/enterprise/arcent"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/cryptohome"
	"chromiumos/tast/local/policyutil"
	"chromiumos/tast/testing"
	"chromiumos/tast/timing"
)

type credentialKeys struct {
	user     string
	password string
	packages string
}

func init() {
	testing.AddTest(&testing.Test{
		Func:         ARCProvisioning,
		LacrosStatus: testing.LacrosVariantUnneeded,
		Desc:         "Checks that ARC is launched when policy is set",
		Contacts:     []string{"pbond@chromium.org", "mhasank@chromium.org", "arc-commercial@google.com"},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"chrome"},
		Timeout:      13 * time.Minute,
		VarDeps: []string{
			"enterprise.ARCProvisioning.user",
			"enterprise.ARCProvisioning.password",
			"enterprise.ARCProvisioning.packages",
			"enterprise.ARCProvisioning.necktie_user",
			"enterprise.ARCProvisioning.necktie_password",
			"enterprise.ARCProvisioning.necktie_packages",
			"enterprise.ARCProvisioning.unmanaged_user",
			"enterprise.ARCProvisioning.unmanaged_password",
			"enterprise.ARCProvisioning.unmanaged_packages",
			"arc.managedAccountPool",
		},
		Params: []testing.Param{
			{
				Val: credentialKeys{
					packages: "enterprise.ARCProvisioning.packages",
				},
				ExtraSoftwareDeps: []string{"android_p"},
			},
			{
				Name: "unmanaged",
				Val: credentialKeys{
					user:     "enterprise.ARCProvisioning.unmanaged_user",
					password: "enterprise.ARCProvisioning.unmanaged_password",
					packages: "enterprise.ARCProvisioning.unmanaged_packages",
				},
				ExtraSoftwareDeps: []string{"android_p"},
			},
			{
				Name: "necktie",
				Val: credentialKeys{
					user:     "enterprise.ARCProvisioning.necktie_user",
					password: "enterprise.ARCProvisioning.necktie_password",
					packages: "enterprise.ARCProvisioning.necktie_packages",
				},
				ExtraSoftwareDeps: []string{"android_p"},
			},
			{
				Name: "vm",
				Val: credentialKeys{
					packages: "enterprise.ARCProvisioning.packages",
				},
				ExtraSoftwareDeps: []string{"android_vm"},
			},
			{
				Name: "unmanaged_vm",
				Val: credentialKeys{
					user:     "enterprise.ARCProvisioning.unmanaged_user",
					password: "enterprise.ARCProvisioning.unmanaged_password",
					packages: "enterprise.ARCProvisioning.unmanaged_packages",
				},
				ExtraSoftwareDeps: []string{"android_vm"},
			},
			{
				Name: "necktie_vm",
				Val: credentialKeys{
					user:     "enterprise.ARCProvisioning.necktie_user",
					password: "enterprise.ARCProvisioning.necktie_password",
					packages: "enterprise.ARCProvisioning.necktie_packages",
				},
				ExtraSoftwareDeps: []string{"android_vm"},
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
	)

	var login chrome.Option
	if s.Param().(credentialKeys).user != "" {
		login = chrome.GAIALogin(chrome.Creds{User: s.RequiredVar(s.Param().(credentialKeys).user), Pass: s.RequiredVar(s.Param().(credentialKeys).password)})
	} else {
		login = chrome.GAIALoginPool(s.RequiredVar("arc.managedAccountPool"))
	}
	packages := strings.Split(s.RequiredVar(s.Param().(credentialKeys).packages), ",")

	// Log-in to Chrome and allow to launch ARC if allowed by user policy.
	cr, err := chrome.New(
		ctx,
		login,
		chrome.ARCSupported(),
		chrome.ProdPolicy(),
		chrome.ExtraArgs(arc.DisableSyncFlags()...))
	if err != nil {
		s.Fatal("Failed to connect to Chrome: ", err)
	}
	defer cr.Close(ctx)

	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to create Test API connection: ", err)
	}

	s.Log("Ensuring ARC is enabled by policy")
	// Ensure chrome://policy shows correct ArcEnabled and ArcPolicy values.
	if err := policyutil.Verify(ctx, tconn, []policy.Policy{&policy.ArcEnabled{Val: true}}); err != nil {
		s.Fatal("Failed to verify ArcEnabled: ", err)
	}

	s.Log("Verifying force-installed apps in ArcPolicy")
	if err := arcent.VerifyArcPolicyForceInstalled(ctx, tconn, packages); err != nil {
		s.Fatal("Failed to verify force-installed apps in ArcPolicy: ", err)
	}

	a, err := arc.NewWithTimeout(ctx, s.OutDir(), bootTimeout)
	if err != nil {
		s.Fatal("Failed to start ARC by user policy: ", err)
	}
	defer a.Close(ctx)

	s.Log("Launching Play Store")
	if err := optin.LaunchAndWaitForPlayStore(ctx, tconn, cr, timeoutWaitForPlayStore); err != nil {
		s.Fatal("Failed to launch Play Store: ", err)
	}

	if err := ensurePackagesUninstallable(ctx, cr, a, packages); err != nil {
		s.Fatal("Package verification failed: ", err)
	}

	if err := launchAssetBrowserActivity(ctx, tconn, a); err != nil {
		s.Fatal("Failed to launch asset browser: ", err)
	}

	if err := ensurePlayStoreNotEmpty(ctx, a); err != nil {
		s.Fatal("Play Store verification failed: ", err)
	}
}

// ensurePackagesUninstallable verifies that force-installed packages can't be uninstalled
func ensurePackagesUninstallable(ctx context.Context, cr *chrome.Chrome, a *arc.ARC, packages []string) error {
	// Ensure that Android packages are force-installed by ARC policy.
	// Note: if the user policy for the user is changed, the packages listed in
	// credentials files must be updated.
	if err := a.WaitForPackages(ctx, packages); err != nil {
		return errors.Wrap(err, "failed to force install packages")
	}

	// Ensure that Andriod packages are set as not-uninstallable by ARC policy.
	testing.ContextLog(ctx, "Waiting for packages being marked as not uninstallable")
	if err := waitForBlockUninstall(ctx, cr, a, packages); err != nil {
		return errors.Wrap(err, "failed to mark packages as not uninstallable")
	}

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
func ensurePlayStoreNotEmpty(ctx context.Context, a *arc.ARC) error {
	const (
		searchBarTextStart = "Search for apps"
		emptyPlayStoreText = "No results found."
	)

	d, err := a.NewUIDevice(ctx)
	if err != nil {
		return errors.Wrap(err, "failed to initialize UI Automator")
	}
	defer d.Close(ctx)

	if err := d.Object(ui.TextStartsWith(searchBarTextStart)).WaitForExists(ctx, 10*time.Second); err != nil {
		return errors.Wrap(err, "unknown Play Store UI screen is shown")
	}

	if err := d.Object(ui.Text(emptyPlayStoreText)).Exists(ctx); err == nil {
		return errors.New("Play Store is empty")
	}

	return nil
}

// launchAssetBrowserActivity starts the activity that displays the available apps.
func launchAssetBrowserActivity(ctx context.Context, tconn *chrome.TestConn, a *arc.ARC) error {
	testing.ContextLog(ctx, "Starting Asset Browser activity")
	act, err := arc.NewActivity(a, "com.android.vending", "com.android.vending.AssetBrowserActivity")
	if err != nil {
		return errors.Wrap(err, "failed to create new activity")
	}
	if err := act.StartWithDefaultOptions(ctx, tconn); err != nil {
		return errors.Wrap(err, "failed starting Play Store or Play Store is empty")
	}

	return nil
}

// readPackageRestrictions reads content of package restrictions file.
func readPackageRestrictions(ctx context.Context, cr *chrome.Chrome) ([]byte, error) {
	const packageRestrictionsPath = "/data/system/users/0/package-restrictions.xml"

	// Cryptohome dir for the current user.
	rootCryptDir, err := cryptohome.SystemPath(ctx, cr.User())
	if err != nil {
		return nil, errors.Wrap(err, "failed to get the cryptohome directory for the user")
	}

	// android-data dir under the cryptohome dir (/home/root/${USER_HASH}/android-data)
	androidDataDir := filepath.Join(rootCryptDir, "android-data")

	return ioutil.ReadFile(filepath.Join(androidDataDir, packageRestrictionsPath))
}

// waitForBlockUninstall waits for Android packages to be set as not uninstallable.
func waitForBlockUninstall(ctx context.Context, cr *chrome.Chrome, a *arc.ARC, packages []string) error {
	ctx, st := timing.Start(ctx, "wait_block_packages")
	defer st.End()

	return testing.Poll(ctx, func(ctx context.Context) error {
		out, err := readPackageRestrictions(ctx, cr)
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
