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
)

// Map of device name -> expected first API level for upreved devices.
// First API level is expected to be the same as current SDK version if the
// device name doesn't exist in this map.
var expectedFirstAPILevelMap = map[string]string{
	"caroline":         apiLevelN,
	"caroline-arcnext": apiLevelN,
	"eve":              apiLevelN,
	"eve-arcnext":      apiLevelN,
	"kevin":            apiLevelN,
	"kevin-arcnext":    apiLevelN,
	"nautilus":         apiLevelN,
	"scarlet":          apiLevelN,
	"scarlet-arcnext":  apiLevelN,
	"soraka":           apiLevelN,
}
