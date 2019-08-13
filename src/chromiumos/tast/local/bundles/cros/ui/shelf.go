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

	// Apps we will be using in the test
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

	cr := s.PreValue().(*chrome.Chrome)
	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to create Test API connection: ", err)
	}

	// At login, we should have just Chrome in the Shelf
	l, err := ash.GetShelfItems(ctx, tconn)
	if err != nil {
		s.Fatal("Failed to get shelf items: ", err)
	}
	if len(l) != 1 {
		s.Fatal("Unexpected apps in the shelf. Expected only Chrome: ", l)
	}

	s.Log("Launching some apps")
	for i := 1; i < len(apps); i++ {
		launchQuery := fmt.Sprintf("tast.promisify(chrome.autotestPrivate.launchApp)(%q)", apps[i].ID)
		if err := tconn.EvalPromise(ctx, launchQuery, nil); err != nil {
			s.Fatal(apps[i].ID, " failed to launch: ", err)
		}
		err = ash.WaitForAppToAppear(ctx, tconn, apps[i].ID)
		if err != nil {
			s.Fatal(apps[i].ID, " did not appear in shelf after launch: ", err)
		}
	}

	// Get the list of apps in the shelf via API and UI
	l, err = ash.GetShelfItems(ctx, tconn)
	if err != nil {
		s.Fatal("Failed to get shelf items: ", err)
	}
	var icons []string
	findQuery := fmt.Sprintf("tast.promisify(chrome.automation.getDesktop)().then(root => root.findAll({attributes: {role: %q}}).map(node => node.name))", "button")
	if err := tconn.EvalPromise(ctx, findQuery, &icons); err != nil {
		s.Fatal("Failed to grab buttons on screen: ", err)
	}

	s.Log("Checking that all expected apps are in the shelf ")
	for i := 0; i < len(l); i++ {
		if apps[i].ID != l[i].AppID {
			s.Fatal("App IDs did not match. Expected: ", apps[i].ID, " Actual: ", l[i].AppID)
		}
		if apps[i].name != l[i].Title {
			s.Fatal("App names did not match. Expected: ", apps[i].name, " Actual: ", l[i].Title)
		}
		// Check that the icons are also present in the UI
		found := false
		for j := 0; j < len(icons); j++ {
			if icons[j] == apps[i].name {
				found = true
				break
			}
		}
		if !found {
			s.Fatal("There was no icon for ", apps[i].name, " in the shelf")
		}
	}
}
