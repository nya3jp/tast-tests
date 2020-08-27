// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package apps

import (
	"context"
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
	if err := runChromeSession(ctx, chromeOpts...); err != nil {
		s.Fatal("First time install failed: ", err)
	}

	// Next, run on the previous profile with chrome.KeepState(), and ask system web app manager to reinstall apps.
	reinstallOpts := append(chromeOpts, chrome.KeepState(), chrome.EnableFeatures("AlwaysReinstallSystemWebApps"))
	if err := runChromeSession(ctx, reinstallOpts...); err != nil {
		s.Fatal("Reinstall failed: ", err)
	}
}

func runChromeSession(ctx context.Context, chromeOpts ...chrome.Option) error {
	cleanupCtx := ctx
	ctx, cancel := ctxutil.Shorten(ctx, 5*time.Second)
	defer cancel()

	cr, err := chrome.New(ctx, chromeOpts...)
	if err != nil {
		return errors.Wrap(err, "failed to start Chrome")
	}

	defer func(ctx context.Context) {
		if err := cr.Close(ctx); err != nil {
			testing.ContextLog(ctx, "Failed to stop first Chrome instance: ", err)
		}
	}(cleanupCtx)

	if err := waitForSystemWebAppsInstall(ctx, cr); err != nil {
		return errors.Wrap(err, "failed to wait system apps install")
	}

	installedAppCount, err := numberOfInstalledSystemApps(ctx, cr)
	if err != nil {
		return errors.Wrap(err, "failed to get the number of installed system apps")
	}

	registeredAppCount, err := numberOfRegisteredSystemApps(ctx, cr)
	if err != nil {
		return errors.Wrap(err, "failed to get the number of registered system apps")
	}

	if installedAppCount != registeredAppCount {
		return errors.Errorf("unexpected number of installed apps: want %d; got %d",
			registeredAppCount, installedAppCount)
	}

	return nil
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

// numberOfRegisteredSystemApps returns the number of system web apps that should be installed,
// by querying System Web App Manager using the test API.
func numberOfRegisteredSystemApps(ctx context.Context, cr *chrome.Chrome) (int, error) {
	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		return -1, errors.Wrap(err, "failed to connect Test API")
	}

	result := 0
	err = tconn.Eval(
		ctx,
		`new Promise((resolve, reject) => {
			chrome.autotestPrivate.getRegisteredSystemWebApps((system_apps) => {
				if (chrome.runtime.lastError) {
					reject(new Error(chrome.runtime.lastError.message));
					return;
				}

				resolve(system_apps.length);
			});
		});`, &result)

	if err != nil {
		return -1, errors.Wrap(err, "failed to get result from Test API")
	}

	return result, nil
}

// numberOfInstalledSystemApps returns the number of system web apps that are actually
// installed, by querying the App Service.
func numberOfInstalledSystemApps(ctx context.Context, cr *chrome.Chrome) (int, error) {
	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		return -1, errors.Wrap(err, "failed to connect Test API")
	}

	result := 0
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

				resolve(system_web_apps.length);
			});
		});`, &result)

	if err != nil {
		return -1, errors.Wrap(err, "failed to get result from Test API")
	}

	return result, nil
}
