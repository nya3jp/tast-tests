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
		Func:         BuildProperties,
		Desc:         "Checks important properties such as first_api_level",
		Attr:         []string{"informational"},
		SoftwareDeps: []string{"android", "chrome_login"},
		Timeout:      4 * time.Minute,
	})
}

func BuildProperties(ctx context.Context, s *testing.State) {
	const (
		propertyBoard         = "ro.product.board"
		propertyFirstAPILevel = "ro.product.first_api_level"
		propertySDKVersion    = "ro.build.version.sdk"
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

	board := getProperty(ctx, s, a, propertyBoard)

	expectedFirstAPILevel, ok := expectedFirstAPILevelMap[board]
	if !ok {
		expectedFirstAPILevel = getProperty(ctx, s, a, propertySDKVersion)
	}

	firstAPILevel := getProperty(ctx, s, a, propertyFirstAPILevel)
	if firstAPILevel != expectedFirstAPILevel {
		s.Fatalf("%v property is %q; want %q", propertyFirstAPILevel,
			firstAPILevel, expectedFirstAPILevel)
	}
}

func getProperty(ctx context.Context, s *testing.State, a *arc.ARC,
	propertyName string) string {
	out, err := a.Command(ctx, "getprop", propertyName).Output()
	if err != nil {
		s.Fatalf("Failed to get %q: %v", propertyName, err)
	}
	return strings.TrimSpace(string(out))
}

const (
	apiLevelN = "25"
	apiLevelP = "28"
)

// Map of board name -> expected first API level for upreved boards.
// First API level is expected to be the same as current SDK version if the
// board name doesn't exist in this map.
var expectedFirstAPILevelMap = map[string]string{
	"atlas":    apiLevelP,
	"grunt":    apiLevelP,
	"nocturne": apiLevelP,
	"octopus":  apiLevelP,
	"rammus":   apiLevelP,

	"asuka":            apiLevelN,
	"auron_paine":      apiLevelN,
	"auron_yuna":       apiLevelN,
	"banon":            apiLevelN,
	"bob":              apiLevelN,
	"caroline":         apiLevelN,
	"caroline-arcnext": apiLevelN,
	"cave":             apiLevelN,
	"celes":            apiLevelN,
	"chell":            apiLevelN,
	"coral":            apiLevelN,
	"cyan":             apiLevelN,
	"edgar":            apiLevelN,
	"elm":              apiLevelN,
	"eve":              apiLevelN,
	"eve-arcnext":      apiLevelN,
	"fizz":             apiLevelN,
	"gandof":           apiLevelN,
	"hana":             apiLevelN,
	"kefka":            apiLevelN,
	"kevin":            apiLevelN,
	"kevin-arcnext":    apiLevelN,
	"lars":             apiLevelN,
	"lulu":             apiLevelN,
	"nami":             apiLevelN,
	"nautilus":         apiLevelN,
	"pyro":             apiLevelN,
	"reef":             apiLevelN,
	"reks":             apiLevelN,
	"relm":             apiLevelN,
	"samus":            apiLevelN,
	"sand":             apiLevelN,
	"scarlet":          apiLevelN,
	"scarlet-arcnext":  apiLevelN,
	"sentry":           apiLevelN,
	"setzer":           apiLevelN,
	"snappy":           apiLevelN,
	"soraka":           apiLevelN,
	"terra":            apiLevelN,
	"ultima":           apiLevelN,
	"veyron_fievel":    apiLevelN,
	"veyron_jerry":     apiLevelN,
	"veyron_mighty":    apiLevelN,
	"veyron_minnie":    apiLevelN,
	"veyron_tiger":     apiLevelN,
	"wizpig":           apiLevelN,
}
