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
		Desc:         "Checks Android build properties such as first_api_level",
		Attr:         []string{"informational"},
		SoftwareDeps: []string{"android", "chrome_login"},
		Timeout:      4 * time.Minute,
	})
}

func BuildProperty(ctx context.Context, s *testing.State) {
	const (
		propertyBoard         = "ro.product.board"
		propertyFirstAPILevel = "ro.product.first_api_level"
		firstAPILevelForArcP  = "28"
		firstAPILevelForArcN  = "25"
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

	out1, err := a.Command(ctx, "getprop", propertyBoard).Output()
	if err != nil {
		s.Fatal("Failed to get board: ", err)
	}
	board := strings.TrimRight(string(out1), "\n")

	out2, err := a.Command(ctx, "getprop", propertyFirstAPILevel).Output()
	if err != nil {
		s.Fatal("Failed to get first_api_level: ", err)
	}
	firstAPILevel := strings.TrimRight(string(out2), "\n")

	if contains(knownArcPBoards(), board) {
		if firstAPILevel != firstAPILevelForArcP {
			s.Fatal("Expected first_api_level is [%s] but got [%s]",
				firstAPILevelForArcP, firstAPILevel)
		}
	} else if contains(knownArcNBoards(), board) {
		if firstAPILevel != firstAPILevelForArcN {
			s.Fatal("Expected first_api_level is [%s] but got [%s]",
				firstAPILevelForArcN, firstAPILevel)
		}
	} else {
		s.Fatal("Unknown board [%s]. "+
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

func knownArcPBoards() []string {
	return []string{
		"atlas",
		"caroline-arcnext",
		"eve-arcnext",
		"grunt",
		"kevin-arcnext",
		"nocturne",
		"octopus",
		"scarlet-arcnext",
	}
}

func knownArcNBoards() []string {
	return []string{
		"asuka",
		"auron_paine",
		"auron_yuna",
		"banon",
		"bob",
		"caroline",
		"caroline-ndktranslation",
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
