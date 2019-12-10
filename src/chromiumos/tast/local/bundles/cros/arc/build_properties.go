// Copyright 2018 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package arc

import (
	"context"
	"regexp"
	"strings"
	"time"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/arc"
	"chromiumos/tast/local/upstart"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         BuildProperties,
		Desc:         "Checks important Android properties such as first_api_level",
		Contacts:     []string{"niwa@chromium.org", "risan@chromium.org", "arc-eng@google.com"},
		SoftwareDeps: []string{"android", "chrome"},
		Timeout:      4 * time.Minute,
		Attr:         []string{"group:mainline"},
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
	if err := upstart.RestartJob(ctx, "ui"); err != nil {
		s.Fatal("Failed to start UI: ", err)
	}

	if err := arc.WaitAndroidInit(ctx); err != nil {
		s.Fatal("Failed to wait Android mini container: ", err)
	}

	getProperty := func(propertyName string) string {
		var value string
		if err := testing.Poll(ctx, func(ctx context.Context) error {
			out, err := arc.BootstrapCommand(ctx, "/system/bin/getprop", propertyName).Output()
			if err != nil {
				return err
			}
			value = strings.TrimSpace(string(out))
			if value == "" {
				return errors.New("getprop returned an empty string")
			}
			return nil
		}, &testing.PollOptions{Timeout: 10 * time.Second}); err != nil {
			s.Fatalf("Failed to get %q: %v", propertyName, err)
		}
		return value
	}

	// Read ro.product.device, drop _cheets suffix, and drop more suffices
	// following '-', like -arcnext or -kernelnext to get the canonical key
	// to map the device name to the first API level.
	//
	// The hyphen subpart conventionally denotes a variance of the same base
	// board that shares the first API level, and moreover they can be truncated
	// (to -kerneln or -ker etc) and becomes hard to match exactly.
	device := getProperty(propertyDevice)
	deviceRegexp := regexp.MustCompile(`^([^-]+).*_cheets$`)
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
)

// Map of device name -> expected first API level.
// First API level is expected to be the same as current SDK version if the
// device name doesn't exist in this map.
var expectedFirstAPILevelMap = map[string]string{
	"asuka":    apiLevelN,
	"paine":    apiLevelN,
	"yuna":     apiLevelN,
	"banon":    apiLevelN,
	"betty":    apiLevelN,
	"bob":      apiLevelN,
	"caroline": apiLevelN,
	"cave":     apiLevelN,
	"celes":    apiLevelN,
	"chell":    apiLevelN,
	"coral":    apiLevelN,
	"cyan":     apiLevelN,
	"edgar":    apiLevelN,
	"elm":      apiLevelN,
	"eve":      apiLevelN,
	"fizz":     apiLevelN,
	"gandof":   apiLevelN,
	"hana":     apiLevelN,
	"kefka":    apiLevelN,
	"kevin":    apiLevelN,
	"lars":     apiLevelN,
	"lulu":     apiLevelN,
	"nami":     apiLevelN,
	"nautilus": apiLevelN,
	"pyro":     apiLevelN,
	"reef":     apiLevelN,
	"reks":     apiLevelN,
	"relm":     apiLevelN,
	"samus":    apiLevelN,
	"sand":     apiLevelN,
	"scarlet":  apiLevelN,
	"sentry":   apiLevelN,
	"setzer":   apiLevelN,
	"snappy":   apiLevelN,
	"soraka":   apiLevelN,
	"terra":    apiLevelN,
	"ultima":   apiLevelN,
	"fievel":   apiLevelN,
	"jerry":    apiLevelN,
	"mighty":   apiLevelN,
	"minnie":   apiLevelN,
	"tiger":    apiLevelN,
	"wizpig":   apiLevelN,
}
