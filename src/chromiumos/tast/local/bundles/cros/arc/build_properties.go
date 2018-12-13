// Copyright 2018 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package arc

import (
	"context"
	"regexp"
	"strings"
	"time"

	"chromiumos/tast/local/arc"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         BuildProperties,
		Desc:         "Checks important Android properties such as first_api_level",
		Attr:         []string{"informational"},
		SoftwareDeps: []string{"android", "chrome_login"},
		Timeout:      4 * time.Minute,
	})
}

func BuildProperties(ctx context.Context, s *testing.State) {
	const (
		propertyDevice        = "ro.product.device"
		propertyFirstAPILevel = "ro.product.first_api_level"
		propertySDKVersion    = "ro.build.version.sdk"
	)

	// TODO(niwa): Mount the Android image and get properties from build.prop
	// instead of booting ARC once b/121170041 is resolved.
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

	getProperty := func(propertyName string) string {
		out, err := a.Command(ctx, "getprop", propertyName).Output()
		if err != nil {
			s.Fatalf("Failed to get %q: %v", propertyName, err)
		}
		return strings.TrimSpace(string(out))
	}

	device := getProperty(propertyDevice)
	deviceRegexp := regexp.MustCompile(`^(.+)_cheets$`)
	match := deviceRegexp.FindStringSubmatch(device)
	if match == nil {
		s.Fatalf("%v property is %q; should have _cheets suffix",
			propertyDevice, device)
	}
	device = match[1]

	expectedFirstAPILevel, ok := expectedFirstAPILevelMap[device]
	if !ok {
		expectedFirstAPILevel = getProperty(propertySDKVersion)
	}

	firstAPILevel := getProperty(propertyFirstAPILevel)
	if firstAPILevel != expectedFirstAPILevel {
		s.Fatalf("%v property is %q; want %q", propertyFirstAPILevel,
			firstAPILevel, expectedFirstAPILevel)
	}
}

const (
	apiLevelN = "25"
	apiLevelP = "28"
)

// Map of device name -> expected first API level for upreved devices.
// First API level is expected to be the same as current SDK version if the
// device name doesn't exist in this map.
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
	"quawks":           apiLevelN,
	"reef":             apiLevelN,
	"reks":             apiLevelN,
	"relm":             apiLevelN,
	"samus":            apiLevelN,
	"sand":             apiLevelN,
	"scarlet":          apiLevelN,
	"sentry":           apiLevelN,
	"setzer":           apiLevelN,
	"snappy":           apiLevelN,
	"soraka":           apiLevelN,
	"squawks":          apiLevelN,
	"terra":            apiLevelN,
	"ultima":           apiLevelN,
	"veyron_fievel":    apiLevelN,
	"veyron_jaq":       apiLevelN,
	"veyron_jerry":     apiLevelN,
	"veyron_mighty":    apiLevelN,
	"veyron_minnie":    apiLevelN,
	"veyron_speedy":    apiLevelN,
	"veyron_tiger":     apiLevelN,
	"wizpig":           apiLevelN,
}
