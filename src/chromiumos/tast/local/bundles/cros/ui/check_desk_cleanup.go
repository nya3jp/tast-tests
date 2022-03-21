// Copyright 2022 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package ui

import (
	"context"

	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/ash"
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         CheckDeskCleanup,
		LacrosStatus: testing.LacrosVariantUnneeded,
		Desc:         "Useful for checking if desks created by a test are removed by Tast framework when the test finishes",
		Contacts:     []string{"amusbach@chromium.org", "xiyuan@chromium.org", "chromeos-wmp@google.com"},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"chrome"},
		Fixture:      "chromeLoggedIn",
		Params:       []testing.Param{{Name: "a"}, {Name: "b"}, {Name: "c"}},
	})
}

func CheckDeskCleanup(ctx context.Context, s *testing.State) {
	tconn, err := s.FixtValue().(*chrome.Chrome).TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to connect to test API: ", err)
	}

	if err := ash.SetOverviewModeAndWait(ctx, tconn, true); err != nil {
		s.Fatal("Failed to ensure overview mode: ", err)
	}

	deskMiniViewsInfo, err := ash.FindDeskMiniViews(ctx, uiauto.New(tconn))
	if err != nil {
		s.Fatal("Failed to get desk mini-views info: ", err)
	}

	s.Log("Number of desk mini-views found before creating a new one: ", len(deskMiniViewsInfo))

	if err := ash.CreateNewDesk(ctx, tconn); err != nil {
		s.Fatal("Failed to create new desk: ", err)
	}

	if err := ash.CreateNewDesk(ctx, tconn); err != nil {
		s.Fatal("Failed to create new desk: ", err)
	}
}
