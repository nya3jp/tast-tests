// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package apps

import (
	"context"
	"strings"
	"time"

	"chromiumos/tast/ctxutil"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/apps"
	"chromiumos/tast/local/bundles/cros/apps/pre"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/uiauto/quicksettings"
	"chromiumos/tast/testing"
	"chromiumos/tast/testing/hwdep"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         SystemWebAppsReinstall,
		LacrosStatus: testing.LacrosVariantUnneeded,
		Desc:         "Checks that system web apps can be reinstalled",
		Contacts: []string{
			"qjw@chromium.org", // Test author
			"chrome-apps-platform-rationalization@google.com", // Backup mailing list
		},
		Timeout:      3 * time.Minute,
		SoftwareDeps: []string{"chrome"},
		Attr:         []string{"group:mainline"},
		Params: []testing.Param{{
			Name:              "default_enabled_apps_stable",
			Val:               []chrome.Option{},
			ExtraHardwareDeps: hwdep.D(pre.SystemWebAppsStableModels),
		}, {
			Name:              "all_apps_stable",
			Val:               []chrome.Option{chrome.EnableFeatures("EnableAllSystemWebApps")},
			ExtraHardwareDeps: hwdep.D(pre.SystemWebAppsStableModels),
		}},
	})
}

// SystemWebAppsReinstall tests that system web apps can be reinstalled (i.e. don't crash Chrome).
func SystemWebAppsReinstall(ctx context.Context, s *testing.State) {
	chromeOpts := s.Param().([]chrome.Option)

	// First run on a clean profile, this is when system web apps are first installed.
	installedNames, registeredInternalNames, err := runChromeSession(ctx, chromeOpts...)
	if err != nil {
		s.Fatal("First time install failed: ", err)
	}

	if len(registeredInternalNames) != len(installedNames) {
		s.Logf("Registered apps: %s", strings.Join(registeredInternalNames, ", "))
		s.Logf("Installed apps: %s", strings.Join(installedNames, ", "))
		s.Fatalf("Unexpected number of installed apps: want %d; got %d", len(registeredInternalNames), len(installedNames))
	}

	// Next, run on the previous profile with chrome.KeepState(), and ask system web app manager to reinstall apps.
	reinstallOpts := append(chromeOpts, chrome.KeepState(), chrome.EnableFeatures("AlwaysReinstallSystemWebApps"))
	installedNames, registeredInternalNames, err = runChromeSession(ctx, reinstallOpts...)
	if err != nil {
		s.Fatal("Reinstall failed: ", err)
	}

	if len(registeredInternalNames) != len(installedNames) {
		s.Logf("Registered apps: %s", strings.Join(registeredInternalNames, ", "))
		s.Logf("Installed apps: %s", strings.Join(installedNames, ", "))
		s.Fatalf("Unexpected number of installed apps: want %d; got %d", len(registeredInternalNames), len(installedNames))
	}
}

// runChromeSession runs Chrome based on chromeOpts, and return a list of installed system app names
// (as shown to users), and a list of registered system app internal names (that should be available
// to users). Note, the app name and internal name are different, thus are not comparable.
func runChromeSession(ctx context.Context, chromeOpts ...chrome.Option) ([]string, []string, error) {
	cleanupCtx := ctx
	ctx, cancel := ctxutil.Shorten(ctx, 5*time.Second)
	defer cancel()

	cr, err := chrome.New(ctx, chromeOpts...)
	if err != nil {
		return nil, nil, errors.Wrap(err, "failed to start Chrome")
	}

	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		return nil, nil, errors.Wrap(err, "failed to connect Test API")
	}

	defer func(ctx context.Context, tconn *chrome.TestConn) {
		if err := quicksettings.SignOut(ctx, tconn); err != nil {
			testing.ContextLog(ctx, "Failed to sign-out: ", err)
		}
	}(cleanupCtx, tconn)

	installedSystemWebApps, err := apps.ListSystemWebApps(ctx, tconn)
	if err != nil {
		return nil, nil, errors.Wrap(err, "failed to get installed system apps")
	}

	var installedNames []string
	for _, app := range installedSystemWebApps {
		installedNames = append(installedNames, app.Name)
	}

	registeredInternalNames, err := apps.ListSystemWebAppsInternalNames(ctx, tconn)
	if err != nil {
		return nil, nil, errors.Wrap(err, "failed to get registered system apps")
	}

	// Handle the case where Crostini (Terminal App) is installed, but not shown to the user due
	// to hardwares not supporting virtualization.
	crostiniIsAvailable, err := supportCrostini(ctx, tconn)
	if err != nil {
		return nil, nil, errors.Wrap(err, "failed to determine crostini support")
	}

	for i, internalName := range registeredInternalNames {
		if internalName == "Terminal" && !crostiniIsAvailable {
			registeredInternalNames = append(registeredInternalNames[:i], registeredInternalNames[i+1:]...)
			break
		}
	}

	return installedNames, registeredInternalNames, nil
}

// supportCrostini returns whether Crostini is allowed to run on device.
func supportCrostini(ctx context.Context, tconn *chrome.TestConn) (bool, error) {
	var result bool
	err := tconn.Eval(
		ctx,
		"tast.promisify(chrome.autotestPrivate.couldAllowCrostini)()", &result)

	if err != nil {
		return false, errors.Wrap(err, "failed to get result from Test API")
	}

	return result, nil
}
