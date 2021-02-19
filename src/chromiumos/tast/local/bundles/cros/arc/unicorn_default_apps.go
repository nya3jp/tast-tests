// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package arc

import (
	"context"
	"time"

	"chromiumos/tast/local/apps"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/ash"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         UnicornDefaultApps,
		Desc:         "Verifies Default arc apps for Unicorn Account",
		Contacts:     []string{"rnanjappan@chromium.org", "cros-arc-te@google.com"},
		Attr:         []string{"group:mainline", "informational"},
		Timeout:      3 * time.Minute,
		SoftwareDeps: []string{"chrome"},
		Params: []testing.Param{{
			ExtraSoftwareDeps: []string{"android_p"},
		}, {
			Name:              "vm",
			ExtraSoftwareDeps: []string{"android_vm"},
		}},
		Vars: []string{"arc.parentUser", "arc.parentPassword", "arc.childUser", "arc.childPassword"},
	})
}

func UnicornDefaultApps(ctx context.Context, s *testing.State) {

	parentUser := s.RequiredVar("arc.parentUser")
	parentPass := s.RequiredVar("arc.parentPassword")
	childUser := s.RequiredVar("arc.childUser")
	childPass := s.RequiredVar("arc.childPassword")
	var cr *chrome.Chrome
	var err error

	cr, err = chrome.New(ctx, chrome.GAIALogin(),
		chrome.Auth(childUser, childPass, "gaia-id"),
		chrome.ParentAuth(parentUser, parentPass), chrome.ARCSupported())

	if err != nil {
		s.Fatal("Failed to start Chrome: ", err)
	}
	defer cr.Close(ctx)

	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to connect Test API: ", err)
	}

	// Lookup for ARC++ default apps
	for _, app := range []apps.App{apps.Duo, apps.PlayBooks, apps.PlayGames, apps.PlayMovies, apps.Clock, apps.Contacts} {
		if err := ash.WaitForChromeAppInstalled(ctx, tconn, app.ID, 600*time.Second); err == nil {
			s.Fatalf("App %s (%s) is installed on child account: %v", app.Name, app.ID, err)
		}
	}
}
