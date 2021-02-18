// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package arc

import (
	"context"
	"time"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/apps"
	"chromiumos/tast/local/arc/optin"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/ash"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:     GuestPlayStore,
		Desc:     "Check PlayStore is Off in Guest mode",
		Contacts: []string{"rnanjappan@chromium.org", "cros-arc-te@google.com"},
		Params: []testing.Param{{
			ExtraSoftwareDeps: []string{"android_p", "chrome"},
		}, {
			Name:              "vm",
			ExtraSoftwareDeps: []string{"android_vm", "chrome"},
		}},
		Timeout: 3 * time.Minute,
		Attr:    []string{"group:mainline", "informational"},
		Fixture: "chromeLoggedInGuest",
	})
}

func GuestPlayStore(ctx context.Context, s *testing.State) {
	cr := s.FixtValue().(*chrome.Chrome)

	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to connect Test API: ", err)
	}

	s.Log("Verify Play Store is Off")
	if err := testing.Poll(ctx, func(ctx context.Context) error {
		playStoreState, err := optin.GetPlayStoreState(ctx, tconn)
		if err != nil {
			return errors.Wrap(err, "failed to get some playstore state")
		}
		if playStoreState["enabled"] == true {
			return errors.New("Playstore is On in Guest Login")
		}
		return nil
	}, &testing.PollOptions{Timeout: 30 * time.Second}); err != nil {
		s.Fatal("Failed to verify Play Store is off: ", err)
	}

	s.Log("Verify None of Default ARC++ Apps is Installed")
	for _, app := range []apps.App{apps.PlayStore, apps.Duo, apps.PlayBooks, apps.PlayGames, apps.PlayMovies, apps.Clock, apps.Contacts} {
		if err := ash.WaitForChromeAppInstalled(ctx, tconn, app.ID, 10*time.Second); err == nil {
			s.Fatalf("Failed to wait for %s (%s) to be installed: %v", app.Name, app.ID, err)
		}
	}
}
