// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package ui

import (
	"context"
	"fmt"

	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/ash"
	"chromiumos/tast/testing"
)

type app struct {
	ID   string
	name string
}

func init() {
	testing.AddTest(&testing.Test{
		Func: Shelf,
		Desc: "Checks that launched apps appear in the shelf",
		Contacts: []string{
			"dhaddock@chromium.org",
		},
		Attr:         []string{"informational"},
		SoftwareDeps: []string{"chrome"},
		Pre:          chrome.LoggedIn(),
	})
}

// Shelf tests the ChromeOS shelf.
func Shelf(ctx context.Context, s *testing.State) {
	cr := s.PreValue().(*chrome.Chrome)

	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to create Test API connection: ", err)
	}

	l, err := ash.GetShelfItems(ctx, tconn)
	if err != nil {
		s.Fatal("Failed to get shelf items: ", err)
	}
	if len(l) != 1 {
		s.Fatal("Unexpected apps in the shelf. Expected only Chrome: ", l)
	}

	var apps = []app{
		{
			ID:   "mgndgikekgjfcpckkfioiadnlibdjbkf",
			name: "Google Chrome",
		},
		{
			ID:   "hhaomjibdihmijegdhdafkllkbggdgoj",
			name: "Files",
		},
		{
			ID:   "obklkkbkpaoaejdabbfldmcfplpdgolj",
			name: "Wallpaper Picker",
		},
	}
	s.Log("Launching some apps")
	for i := 1; i < len(apps); i++ {
		launchQuery := fmt.Sprintf("tast.promisify(chrome.autotestPrivate.launchApp)(%q)", apps[i].ID)
		if err := tconn.EvalPromise(ctx, launchQuery, nil); err != nil {
			s.Fatal(apps[i].ID, " failed to launch: ", err)
		}
		err = ash.WaitForAppShown(ctx, tconn, apps[i].ID)
		if err != nil {
			s.Fatal(apps[i].ID, " did not appear in shelf after launch: ", err)
		}
	}

	s.Log("Checking that the launched apps appear in the shelf")
	l, err = ash.GetShelfItems(ctx, tconn)
	if err != nil {
		s.Fatal("Failed to get shelf items: ", err)
	}
	if len(l) != 3 {
		s.Fatal("Unexpected shelf items. Expected: 3 Actual: ", len(l))
	}
	for i := 0; i < len(l); i++ {
		if apps[i].ID != l[i].AppID {
			s.Fatal("App IDs did not match. Expected: ", apps[i].ID, " Actual: ", l[i].AppID)
		}
		if apps[i].name != l[i].Title {
			s.Fatal("App names did not match. Expected: ", apps[i].name, " Actual: ", l[i].Title)
		}
	}
}
