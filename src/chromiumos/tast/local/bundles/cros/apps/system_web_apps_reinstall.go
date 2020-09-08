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
	installedApps, registeredApps, err := runChromeSession(ctx, chromeOpts...)
	if err != nil {
		s.Fatal("First time install failed: ", err)
	}

	if len(installedApps) != len(registeredApps) {
		s.Logf("Installed apps = %s", strings.Join(installedApps, ", "))
		s.Logf("Registered apps = %s", strings.Join(registeredApps, ", "))
		s.Fatalf("Unexpected number of installed apps: want %d; got %d", len(installedApps), len(registeredApps))
	}

	// Next, run on the previous profile with chrome.KeepState(), and ask system web app manager to reinstall apps.
	reinstallOpts := append(chromeOpts, chrome.KeepState(), chrome.EnableFeatures("AlwaysReinstallSystemWebApps"))
	installedApps, registeredApps, err = runChromeSession(ctx, reinstallOpts...)
	if err != nil {
		s.Fatal("Reinstall failed: ", err)
	}

	if len(installedApps) != len(registeredApps) {
		s.Logf("Installed apps = %s", strings.Join(installedApps, ", "))
		s.Logf("Registered apps = %s", strings.Join(registeredApps, ", "))
		s.Fatalf("Unexpected number of installed apps: want %d; got %d", len(installedApps), len(registeredApps))
	}
}

// runChromeSession runs Chrome based on chromeOpts, and return a list of installed system app names, and a list of
// registered system app internal names.
func runChromeSession(ctx context.Context, chromeOpts ...chrome.Option) (installedNames, registeredNames []string, err error) {
	var installedAppNames []string
	var registeredAppNames []string

	cleanupCtx := ctx
	ctx, cancel := ctxutil.Shorten(ctx, 5*time.Second)
	defer cancel()

	cr, err := chrome.New(ctx, chromeOpts...)
	if err != nil {
		return installedAppNames, registeredAppNames, errors.Wrap(err, "failed to start Chrome")
	}

	defer func(ctx context.Context) {
		if err := cr.Close(ctx); err != nil {
			testing.ContextLog(ctx, "Failed to stop first Chrome instance: ", err)
		}
	}(cleanupCtx)

	if err := waitForSystemWebAppsInstall(ctx, cr); err != nil {
		return installedAppNames, registeredAppNames, errors.Wrap(err, "failed to wait system apps install")
	}

	installedApps, err := installedSystemApps(ctx, cr)
	if err != nil {
		return installedAppNames, registeredAppNames, errors.Wrap(err, "failed to get installed system apps")
	}
	defer installedApps.Release(ctx)

	registeredApps, err := registeredSystemApps(ctx, cr)
	if err != nil {
		return installedAppNames, registeredAppNames, errors.Wrap(err, "failed to get registered system apps")
	}
	defer registeredApps.Release(ctx)

	installedApps.Call(ctx, &installedAppNames, "function() { return this.map(app => app.name) }")
	registeredApps.Call(ctx, &registeredAppNames, "function() { return this.map(systemApp => systemApp.internalName) }")

	return installedAppNames, registeredAppNames, nil
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

// registeredSystemApps returns a chrome.JSObject that contains the information (list of SystemApp)
// about registered System Apps by querying System Web App Manager using the test API. Remember to
// call Release() on the returned JSObject after use.
func registeredSystemApps(ctx context.Context, cr *chrome.Chrome) (chrome.JSObject, error) {
	var result chrome.JSObject

	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		return result, errors.Wrap(err, "failed to connect Test API")
	}

	err = tconn.Eval(
		ctx,
		`new Promise((resolve, reject) => {
			chrome.autotestPrivate.getRegisteredSystemWebApps((system_apps) => {
				if (chrome.runtime.lastError) {
					reject(new Error(chrome.runtime.lastError.message));
					return;
				}

				resolve(system_apps);
			});
		});`, &result)

	if err != nil {
		return result, errors.Wrap(err, "failed to get result from Test API")
	}

	return result, nil
}

// installedSystemApps returns a chrome.JSObject that contains the information (list of App)
// about installed System Apps, by querying App Service. Remember to call Release() on the
// returned JSObject after use.
func installedSystemApps(ctx context.Context, cr *chrome.Chrome) (chrome.JSObject, error) {
	var result chrome.JSObject

	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		return result, errors.Wrap(err, "failed to connect Test API")
	}

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

				resolve(system_web_apps);
			});
		});`, &result)

	if err != nil {
		return result, errors.Wrap(err, "failed to get result from Test API")
	}

	return result, nil
}
