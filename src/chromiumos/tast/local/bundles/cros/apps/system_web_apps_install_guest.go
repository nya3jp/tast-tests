// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package apps

import (
	"context"
	"strings"
	"time"

	"chromiumos/tast/local/apps"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func: SystemWebAppsInstallGuest,
		Desc: "Checks that system web apps are installed in guest mode",
		Contacts: []string{
			"benreich@chromium.org",
			"chrome-apps-platform-rationalization@google.com",
		},
		Timeout:      3 * time.Minute,
		SoftwareDeps: []string{"chrome"},
		Attr:         []string{"group:mainline"},
		Params: []testing.Param{{
			Name: "all_apps",
			Val:  []chrome.Option{chrome.EnableFeatures("EnableAllSystemWebApps")},
		}, {
			Name: "default_enabled_apps",
			Val:  []chrome.Option{},
		}},
	})
}

// SystemWebAppsInstallGuest tests that system web apps are installed on a guest profile.
func SystemWebAppsInstallGuest(ctx context.Context, s *testing.State) {
	chromeOpts := s.Param().([]chrome.Option)
	chromeOpts = append(chromeOpts, chrome.GuestLogin())

	cr, err := chrome.New(ctx, chromeOpts...)
	if err != nil {
		s.Fatal("Failed to start Chrome: ", err)
	}
	defer cr.Close(ctx)

	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to connect Test API: ", err)
	}

	installedSystemWebApps, err := apps.ListSystemWebApps(ctx, tconn)
	if err != nil {
		s.Fatal("Failed to get installed system apps: ", err)
	}

	registeredInternalNames, err := apps.ListSystemWebAppsInternalNames(ctx, tconn)
	if err != nil {
		s.Fatal("Failed to get registered system apps: ", err)
	}

	for i, internalName := range registeredInternalNames {
		// Terminal SWA is always installed but it is disabled (controlled by Crostini App Service publisher) in guest mode.
		// Exclude it from the list as it does not show in installedSystemWebApps.
		// https://source.chromium.org/chromium/chromium/src/+/HEAD:chrome/browser/apps/app_service/crostini_apps.cc;l=240
		if internalName == "Terminal" {
			registeredInternalNames = append(registeredInternalNames[:i], registeredInternalNames[i+1:]...)
			break
		}
	}

	// Only the list length is compares and not the names because the registeredInternalNames are different to
	// installedSystemWebApps, e.g. OSSettings vs. Settings.
	if len(registeredInternalNames) != len(installedSystemWebApps) {
		var installedNames []string
		for _, app := range installedSystemWebApps {
			installedNames = append(installedNames, app.Name)
		}

		s.Logf("Registered apps: %s", strings.Join(registeredInternalNames, ", "))
		s.Logf("Installed apps: %s", strings.Join(installedNames, ", "))
		s.Fatalf("Unexpected number of installed apps: want %d; got %d", len(registeredInternalNames), len(installedNames))
	}
}
