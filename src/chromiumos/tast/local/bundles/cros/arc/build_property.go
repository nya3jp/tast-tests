// Copyright 2018 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package arc

import (
	"context"
	"strings"
	"time"

	"chromiumos/tast/local/arc"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         BuildProperty,
		Desc:         "Checks important properties such as first_api_level",
		Attr:         []string{"informational"},
		SoftwareDeps: []string{"android", "chrome_login"},
		Timeout:      4 * time.Minute,
	})
}

func BuildProperty(ctx context.Context, s *testing.State) {
	const (
		propertyBoard         = "ro.product.board"
		propertyFirstAPILevel = "ro.product.first_api_level"
		apiLevelP             = "28"
		apiLevelN             = "25"
	)

	cr, err := chrome.New(ctx, chrome.ARCEnabled())
	if err != nil {
		s.Fatal("Failed to connect to Chrome: ", err)
	}
	defer cr.Close(ctx)

	a, err := arc.New(ctx, s.OutDir())
	if err != nil {
		s.Fatal("Failed to start ARC: ", err)
	}
	defer a.Close()

	out, err := a.Command(ctx, "getprop", propertyBoard).Output()
	if err != nil {
		s.Fatal("Failed to get board: ", err)
	}
	board := strings.TrimSpace(string(out))

	out, err = a.Command(ctx, "getprop", propertyFirstAPILevel).Output()
	if err != nil {
		s.Fatal("Failed to get first_api_level: ", err)
	}
	firstAPILevel := strings.TrimSpace(string(out))

	if contains(boardsLaunchedInP(), board) {
		if firstAPILevel != apiLevelP {
			s.Fatalf("Expected first_api_level is %q but got %q",
				apiLevelP, firstAPILevel)
		}
	} else if contains(boardsLaunchedInN(), board) {
		if firstAPILevel != apiLevelN {
			s.Fatalf("Expected first_api_level is %q but got %q",
				apiLevelN, firstAPILevel)
		}
	} else {
		s.Fatalf("Unknown board %q "+
			"Please update the known board list in build_property.go", board)
	}
}

func contains(strs []string, x string) bool {
	for _, str := range strs {
		if x == str {
			return true
		}
	}
	return false
}

func boardsLaunchedInP() []string {
	return []string{
		"atlas",
		"grunt",
		"nocturne",
		"octopus",
		"rammus",
	}
}

func boardsLaunchedInN() []string {
	return []string{
		"asuka",
		"auron_paine",
		"auron_yuna",
		"banon",
		"bob",
		"caroline",
		"cave",
		"celes",
		"chell",
		"coral",
		"cyan",
		"edgar",
		"elm",
		"eve",
		"fizz",
		"gandof",
		"hana",
		"kefka",
		"kevin",
		"lars",
		"lulu",
		"nami",
		"nautilus",
		"pyro",
		"reef",
		"reks",
		"relm",
		"samus",
		"sand",
		"scarlet",
		"sentry",
		"setzer",
		"snappy",
		"soraka",
		"terra",
		"ultima",
		"veyron_fievel",
		"veyron_jerry",
		"veyron_mighty",
		"veyron_minnie",
		"veyron_tiger",
		"wizpig",
	}
}
