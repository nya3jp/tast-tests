// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package ui

import (
	"context"
	"time"

	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/upstart"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func: SystemWebAppsReinstall,
		Desc: "Checks that all default enabled system apps can be reinstalled",
		Contacts: []string{
			"qjw@chromium.org",                   // Test author
			"system-web-apps-discuss@google.com", // Backup mailing list
		},
		Timeout:      5 * time.Minute,
		SoftwareDeps: []string{"chrome"},
		Attr:         []string{"group:mainline", "informational"},
	})
}

func waitForSystemWebAppsInstall(ctx context.Context, s *testing.State, cr *chrome.Chrome) {
	testConn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to connect Test API: ", err)
		return
	}

	defer testConn.Close()

	err = testConn.Eval(
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
		nil /* out */)

	if err != nil {
		s.Fatal("Failed to wait for system apps install: ", err)
		return
	}

	s.Log("System Web Apps installed")
}

// getNumberOfRegisteredSystemApps returns the number of system web apps that should be installed,
// by querying System Web App Manager using the test API.
func getNumberOfRegisteredSystemApps(ctx context.Context, s *testing.State, cr *chrome.Chrome) int {
	testConn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to connect Test API: ", err)
		return -1
	}

	defer testConn.Close()

	result := 0
	err = testConn.Eval(
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
		s.Fatal("Failed to get the number of registered system web apps: ", err)
		return -1
	}

	return result
}

// getNumberOfInstalledSystemApps returns the number of system web apps that are actually
// installed, by querying the App Service.
func getNumberOfInstalledSystemApps(ctx context.Context, s *testing.State, cr *chrome.Chrome) int {
	testConn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to connect Test API: ", err)
		return -1
	}

	defer testConn.Close()

	result := 0
	err = testConn.Eval(
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
		s.Fatal("Failed to get the number of registered system web apps: ", err)
		return -1
	}

	return result
}

func getInstalledApps(ctx context.Context, s *testing.State, cr *chrome.Chrome) int {
	testConn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to connect Test API: ", err)
		return -1
	}

	defer testConn.Close()

	result := 0
	err = testConn.Eval(
		ctx,
		`new Promise((resolve, reject) => {
			chrome.autotestPrivate.getAllInstalledApps((apps) => {
				if (chrome.runtime.lastError) {
					reject(new Error(chrome.runtime.lastError.message));
					return;
				}

				resolve(apps.length);
			});
		});`, &result)

	if err != nil {
		s.Fatal("Failed to get the number of registered system web apps: ", err)
		return -1
	}

	return result
}

// SystemWebAppsReinstall tests that system web apps can be reinstalled (i.e. doesn't crash Chrome).
func SystemWebAppsReinstall(ctx context.Context, s *testing.State) {
	// First login. This happens on a clean state.
	cr, err := chrome.New(ctx, chrome.ExtraArgs("--enable-features=EnableAllSystemWebApps"))
	if err != nil {
		s.Fatal("Failed to start Chrome: ", err)
	}

	waitForSystemWebAppsInstall(ctx, s, cr)
	if getNumberOfInstalledSystemApps(ctx, s, cr) != getNumberOfRegisteredSystemApps(ctx, s, cr) {
		s.Fatal("Number of installed system apps doesn't match the expected number")
	}

	// Emulate logout. chrome.Chrome.Close() does not log out. So, here, manually restart "ui" job
	// for the emulation.
	if err := upstart.RestartJob(ctx, "ui"); err != nil {
		s.Fatal("Failed to log out: ", err)
	}

	// Second login, this reuses the previous state, so System Apps are already installed.
	cr2, err := chrome.New(ctx, chrome.ExtraArgs("--enable-features=EnableAllSystemWebApps,AlwaysReinstallSystemWebApps"), chrome.KeepState())
	if err != nil {
		s.Fatal("Failed to start Chrome: ", err)
	}

	waitForSystemWebAppsInstall(ctx, s, cr2)
	if getNumberOfInstalledSystemApps(ctx, s, cr2) != getNumberOfRegisteredSystemApps(ctx, s, cr2) {
		s.Fatal("Number of installed system apps doesn't match the expected number")
	}

	cr2.Close(ctx)
}
