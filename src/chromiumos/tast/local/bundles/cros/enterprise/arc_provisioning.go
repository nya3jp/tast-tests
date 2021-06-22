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

	"chromiumos/tast/common/policy"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/android/ui"
	"chromiumos/tast/local/arc"
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
		Desc:         "Checks that ARC is launched when policy is set",
		Contacts:     []string{"pbond@chromium.org", "arc-eng-muc@google.com"},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"chrome"},
		Timeout:      13 * time.Minute,
		Vars: []string{
			"enterprise.ARCProvisioning.user",
			"enterprise.ARCProvisioning.password",
			"enterprise.ARCProvisioning.packages",
			"enterprise.ARCProvisioning.necktie_user",
			"enterprise.ARCProvisioning.necktie_password",
			"enterprise.ARCProvisioning.necktie_packages",
			"enterprise.ARCProvisioning.unmanaged_user",
			"enterprise.ARCProvisioning.unmanaged_password",
			"enterprise.ARCProvisioning.unmanaged_packages",
		},
		Params: []testing.Param{
			{
				Val: credentialKeys{
					user:     "enterprise.ARCProvisioning.user",
					password: "enterprise.ARCProvisioning.password",
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
					user:     "enterprise.ARCProvisioning.user",
					password: "enterprise.ARCProvisioning.password",
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
		searchBarTextStart = "Search for apps"
		emptyPlayStoreText = "No results found."
	)

	user := s.RequiredVar(s.Param().(credentialKeys).user)
	password := s.RequiredVar(s.Param().(credentialKeys).password)
	packages := strings.Split(s.RequiredVar(s.Param().(credentialKeys).packages), ",")
	// Log-in to Chrome and allow to launch ARC if allowed by user policy.
	cr, err := chrome.New(
		ctx,
		chrome.GAIALogin(chrome.Creds{User: user, Pass: password}),
		chrome.ARCSupported(),
		chrome.ProdPolicy(),
		chrome.ExtraArgs(arc.DisableSyncFlags()...))
	if err != nil {
		s.Fatal("Failed to connect to Chrome: ", err)
	}
	defer cr.Close(ctx)

	// Ensure that ARC is launched.
	a, err := arc.New(ctx, s.OutDir())
	if err != nil {
		s.Fatal("Failed to start ARC by user policy: ", err)
	}
	defer a.Close(ctx)

	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to create Test API connection: ", err)
	}

	// Ensure chrome://policy shows correct ArcEnabled and ArcPolicy values.
	if err := policyutil.Verify(ctx, tconn, []policy.Policy{&policy.ArcEnabled{Val: true}}); err != nil {
		s.Fatal("Failed to verify ArcEnabled: ", err)
	}

	if err := verifyArcPolicyForceInstalled(ctx, tconn, packages); err != nil {
		s.Fatal("Failed to verify force-installed apps in ArcPolicy: ", err)
	}

	// Ensure that Android packages are force-installed by ARC policy.
	// Note: if the user policy for the user is changed, the packages listed in
	// credentials files must be updated.
	if err := a.WaitForPackages(ctx, packages); err != nil {
		s.Fatal("Failed to force install packages: ", err)
	}

	// Ensure that Andriod packages are set as not-uninstallable by ARC policy.
	s.Log("Waiting for packages being marked as not uninstallable")
	if err := waitForBlockUninstall(ctx, cr, a, packages); err != nil {
		s.Fatal("Failed to mark packages as not uninstallable: ", err)
	}

	// Try uninstalling packages with ADB, should fail.
	s.Log("Trying to uninstall packages")
	for _, p := range packages {
		if a.Uninstall(ctx, p) == nil {
			s.Fatalf("Package %q can be uninstalled", p)
		}
	}

	// Ensure Play Store is not empty.
	s.Log("Starting Play Store")
	act, err := arc.NewActivity(a, "com.android.vending", "com.android.vending.AssetBrowserActivity")
	if err != nil {
		s.Fatal("Failed to create new activity: ", err)
	}
	defer act.Close()

	if err := act.Start(ctx, tconn); err != nil {
		s.Fatal("Failed starting Play Store or Play Store is empty: ", err)
	}

	d, err := a.NewUIDevice(ctx)
	if err != nil {
		s.Fatal("Failed initializing UI Automator: ", err)
	}
	defer d.Close(ctx)

	if err := d.Object(ui.TextStartsWith(searchBarTextStart)).WaitForExists(ctx, 10*time.Second); err != nil {
		s.Fatal("Unknown Play Store UI screen is shown: ", err)
	}

	if err := d.Object(ui.Text(emptyPlayStoreText)).Exists(ctx); err == nil {
		s.Fatal("Play Store is empty")
	}
}

// readPackageRestrictions reads content of package restrictions file.
func readPackageRestrictions(ctx context.Context, cr *chrome.Chrome) ([]byte, error) {
	const packageRestrictionsPath = "/data/system/users/0/package-restrictions.xml"

	// Cryptohome dir for the current user.
	rootCryptDir, err := cryptohome.SystemPath(cr.User())
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

// verifyArcPolicyForceInstalled matches ArcPolicy FORCE_INSTALLED apps list with expected packages.
func verifyArcPolicyForceInstalled(ctx context.Context, tconn *chrome.TestConn, forceInstalledPackages []string) error {
	dps, err := policyutil.PoliciesFromDUT(ctx, tconn)
	if err != nil {
		return err
	}

	expected := &policy.ArcPolicy{}
	actual, ok := dps.Chrome[expected.Name()]
	if !ok {
		return errors.New("no such a policy ArcPolicy")
	}
	value, err := expected.UnmarshalAs(actual.ValueJSON)
	if err != nil {
		return err
	}
	arcPolicyValue, ok := value.(policy.ArcPolicyValue)
	if !ok {
		return errors.Errorf("cannot extract ArcPolicyValue %s", value)
	}

	apps := arcPolicyValue.Applications
	forceInstalled := make(map[string]bool)
	for _, application := range apps {
		packageName := application.PackageName
		installType := application.InstallType
		if installType == "FORCE_INSTALLED" {
			forceInstalled[packageName] = true
		}
	}
	for _, p := range forceInstalledPackages {
		if forceInstalled[p] {
			delete(forceInstalled, p)
		} else {
			return errors.Errorf("the next package is not FORCE_INSTALLED by policy: %s", p)
		}
	}
	if len(forceInstalled) != 0 {
		return errors.Errorf("Extra FORCE_INSTALLED packages in ArcPolicy: %s", makeList(forceInstalled))
	}
	return nil
}
