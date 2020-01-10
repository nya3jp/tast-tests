// Copyright 2018 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package arc

import (
	"context"
	"io/ioutil"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/arc"
	"chromiumos/tast/local/testexec"
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

	expectedFirstAPILevel := getProperty(propertySDKVersion)
	if overwrite, ok := expectedFirstAPILevelMap[device]; ok {
		expectedFirstAPILevel = strconv.Itoa(overwrite)
	}

	firstAPILevel := getProperty(propertyFirstAPILevel)
	if firstAPILevel != expectedFirstAPILevel {
		if props, err := arc.BootstrapCommand(ctx, "/system/bin/getprop").Output(testexec.DumpLogOnError); err != nil {
			s.Log("Failed to read properties: ", err)
		} else if err := ioutil.WriteFile(filepath.Join(s.OutDir(), "props.txt"), props, 0644); err != nil {
			s.Log("Failed to dump properties: ", err)
		}
		s.Fatalf("Unexpected %v property (see props.txt for details): got %q; want %q", propertyFirstAPILevel,
			firstAPILevel, expectedFirstAPILevel)
	}
}

// Map of device name -> expected first API level.
// First API level is expected to be the same as current SDK version if the
// device name doesn't exist in this map.
var expectedFirstAPILevelMap = map[string]int{
	"asuka":    arc.SDKN,
	"paine":    arc.SDKN,
	"yuna":     arc.SDKN,
	"banon":    arc.SDKN,
	"betty":    arc.SDKN,
	"bob":      arc.SDKN,
	"caroline": arc.SDKN,
	"cave":     arc.SDKN,
	"celes":    arc.SDKN,
	"chell":    arc.SDKN,
	"coral":    arc.SDKN,
	"cyan":     arc.SDKN,
	"edgar":    arc.SDKN,
	"elm":      arc.SDKN,
	"eve":      arc.SDKN,
	"fizz":     arc.SDKN,
	"gandof":   arc.SDKN,
	"hana":     arc.SDKN,
	"kefka":    arc.SDKN,
	"kevin":    arc.SDKN,
	"lars":     arc.SDKN,
	"lulu":     arc.SDKN,
	"nami":     arc.SDKN,
	"nautilus": arc.SDKN,
	"pyro":     arc.SDKN,
	"reef":     arc.SDKN,
	"reks":     arc.SDKN,
	"relm":     arc.SDKN,
	"samus":    arc.SDKN,
	"sand":     arc.SDKN,
	"scarlet":  arc.SDKN,
	"sentry":   arc.SDKN,
	"setzer":   arc.SDKN,
	"snappy":   arc.SDKN,
	"soraka":   arc.SDKN,
	"terra":    arc.SDKN,
	"ultima":   arc.SDKN,
	"fievel":   arc.SDKN,
	"jerry":    arc.SDKN,
	"mighty":   arc.SDKN,
	"minnie":   arc.SDKN,
	"tiger":    arc.SDKN,
	"wizpig":   arc.SDKN,
}
