// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package apps

import (
	"context"
	"time"

	"chromiumos/tast/local/apps"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         SystemWebAppsInstallGuest,
		LacrosStatus: testing.LacrosVariantUnneeded,
		Desc:         "Checks that system web apps are installed in guest mode",
		Contacts: []string{
			"qjw@chromium.org",
			"chrome-apps-platform-rationalization@google.com",
		},
		Timeout:      3 * time.Minute,
		SoftwareDeps: []string{"chrome"},
		Fixture:      "chromeLoggedInGuest",
		Attr:         []string{"group:mainline"},
	})
}

// SystemWebAppsInstallGuest tests that system web apps are installed on a guest profile.
func SystemWebAppsInstallGuest(ctx context.Context, s *testing.State) {
	cr := s.FixtValue().(*chrome.Chrome)

	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to connect Test API: ", err)
	}

	registeredSystemWebApps, err := apps.ListRegisteredSystemWebApps(ctx, tconn)
	if err != nil {
		s.Fatal("Failed to get registered system apps: ", err)
	}

	// Verify a set of system web apps that triggers different install code paths are installed.
	testAppInternalNames := map[string]bool{
		"OSSettings": true,
		"Media":      true,
		"Help":       true,
	}

	for _, swa := range registeredSystemWebApps {
		if testAppInternalNames[swa.InternalName] {
			app, err := apps.FindSystemWebAppByOrigin(ctx, tconn, swa.StartURL)
			if err != nil {
				s.Fatalf("Failed to match system web app by origin, app: %s, origin: %s, errror: %v", swa.InternalName, swa.StartURL, err)
			}
			if app == nil {
				s.Fatal("Failed to find system web app that should have been installed: ", swa.InternalName)
			}
		}
	}
}
