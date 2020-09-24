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
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func: SystemWebAppsReinstall,
		Desc: "Checks that system web apps can be reinstalled",
		Contacts: []string{
			"qjw@chromium.org", // Test author
			"chrome-apps-platform-rationalization@google.com", // Backup mailing list
		},
		Timeout:      3 * time.Minute,
		SoftwareDeps: []string{"chrome"},
		Attr:         []string{"group:mainline", "informational"},
		Params: []testing.Param{{
			Name: "default_enabled_apps",
			Val:  []chrome.Option{},
		}, {
			Name: "all_apps",
			Val:  []chrome.Option{chrome.EnableFeatures("EnableAllSystemWebApps")},
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

	defer func(ctx context.Context) {
		if err := cr.Close(ctx); err != nil {
			testing.ContextLog(ctx, "Failed to stop first Chrome instance: ", err)
		}
	}(cleanupCtx)

	if err := waitForSystemWebAppsInstall(ctx, cr); err != nil {
		return nil, nil, errors.Wrap(err, "failed to wait system apps install")
	}

	installedNames, err := installedSystemAppsNames(ctx, cr)
	if err != nil {
		return nil, nil, errors.Wrap(err, "failed to get installed system apps")
	}

	registeredInternalNames, err := registeredSystemAppsInternalNames(ctx, cr)
	if err != nil {
		return nil, nil, errors.Wrap(err, "failed to get registered system apps")
	}

	// Handle the case where Crostini (Terminal App) is installed, but not shown to the user due
	// to hardwares not supporting virtualization.
	crostiniIsAvailable, err := supportCrostini(ctx, cr)
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

// supportCrostini returns whether Crostini is allowed to run on device, by checking relevant
// information with OS Settings.
func supportCrostini(ctx context.Context, cr *chrome.Chrome) (bool, error) {
	conn, err := cr.NewConn(ctx, "chrome://os-settings/")
	if err != nil {
		return false, errors.Wrap(err, "failed to get connection to os settings")
	}
	defer conn.Close()

	var allowCrostini bool
	if err := conn.Eval(ctx, "window.loadTimeData.data_.allowCrostini", &allowCrostini); err != nil {
		return false, errors.Wrap(err, "failed to evaluate window.loadTimeData.data_.allowCrostini")
	}

	return allowCrostini, nil
}

func waitForSystemWebAppsInstall(ctx context.Context, cr *chrome.Chrome) error {
	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		return errors.Wrap(err, "failed to connect Test API")
	}

	err = tconn.Eval(
		ctx,
		`new Promise((resolve, reject) => {
			chrome.autotestPrivate.waitForSystemWebAppsInstall(() => {
				if (chrome.runtime.lastError) {
					reject(new Error(chrome.runtime.lastError.message));
					return;
				}

				resolve();
			});
		});`,
		nil)

	if err != nil {
		return errors.Wrap(err, "failed to get result from Test API")
	}

	return nil
}

// registeredSystemAppsInternalNames returns a string[] that contains system app's internal names.
// It queries System Web App Manager via Test API.
func registeredSystemAppsInternalNames(ctx context.Context, cr *chrome.Chrome) ([]string, error) {
	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		return nil, errors.Wrap(err, "failed to connect Test API")
	}

	var result []string
	err = tconn.Eval(
		ctx,
		`new Promise((resolve, reject) => {
			chrome.autotestPrivate.getRegisteredSystemWebApps((system_apps) => {
				if (chrome.runtime.lastError) {
					reject(new Error(chrome.runtime.lastError.message));
					return;
				}

				resolve(system_apps.map(system_app => system_app.internalName));
			});
		});`, &result)

	if err != nil {
		return nil, errors.Wrap(err, "failed to get result from Test API")
	}

	return result, nil
}

// installedSystemAppsNames returns a string[] that contains system app's visible name to the user.
// It queries App Service via Test API.
func installedSystemAppsNames(ctx context.Context, cr *chrome.Chrome) ([]string, error) {
	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		return nil, errors.Wrap(err, "failed to connect Test API")
	}

	var result []string
	err = tconn.Eval(
		ctx,
		`new Promise((resolve, reject) => {
			chrome.autotestPrivate.getAllInstalledApps((apps) => {
				if (chrome.runtime.lastError) {
					reject(new Error(chrome.runtime.lastError.message));
					return;
				}

				// Note, Terminal has special handling in App Service.
				// It has type 'Crostini' and install source 'User'.
				const system_web_apps = apps.filter(app =>
					   (app.installSource === 'System' && app.type === 'Web')
					|| (app.installSource === 'User' && app.type === 'Crostini')
				)

				resolve(system_web_apps.map(app => app.name));
			});
		});`, &result)

	if err != nil {
		return nil, errors.Wrap(err, "failed to get result from Test API")
	}

	return result, nil
}
