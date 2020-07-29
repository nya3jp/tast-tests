// Copyright 2018 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package arc

import (
	"context"
	"fmt"
	"io/ioutil"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"

	"chromiumos/tast/local/arc"
	"chromiumos/tast/local/testexec"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         BuildProperties,
		Desc:         "Checks important Android properties such as first_api_level",
		Contacts:     []string{"niwa@chromium.org", "risan@chromium.org", "arc-eng@google.com"},
		SoftwareDeps: []string{"chrome"},
		Pre:          arc.Booted(),
		// TODO(yusukes): Change the timeout back to 4 min when we revert arc.go's BootTimeout to 120s.
		Timeout: 5 * time.Minute,
		Attr:    []string{"group:mainline"},
		Params: []testing.Param{{
			ExtraSoftwareDeps: []string{"android_p"},
		}, {
			Name:              "vm",
			ExtraSoftwareDeps: []string{"android_vm"},
		}},
	})
}

func BuildProperties(ctx context.Context, s *testing.State) {
	const (
		propertyBootType      = "ro.vendor.arc_boot_type"
		propertyBoard         = "ro.product.board"
		propertyDevice        = "ro.product.device"
		propertyFirstAPILevel = "ro.product.first_api_level"
		propertyModel         = "ro.product.model"
		propertySDKVersion    = "ro.build.version.sdk"
	)

	a := s.PreValue().(arc.PreData).ARC

	getProperty := func(propertyName string) string {
		var value string
		if err := testing.Poll(ctx, func(ctx context.Context) error {
			out, err := a.Command(ctx, "getprop", propertyName).Output()
			if err != nil {
				return err
			}
			value = strings.TrimSpace(string(out))
			return nil
		}, &testing.PollOptions{Timeout: 10 * time.Second}); err != nil {
			s.Fatalf("Failed to get %q: %v", propertyName, err)
		}
		return value
	}

	getAllPropertiesMap := func() map[string]bool {
		// Returns a list of existing property names as a map.
		out, err := a.Command(ctx, "getprop").Output(testexec.DumpLogOnError)
		if err != nil {
			s.Log("Failed to read properties: ", err)
		}
		properties := make(map[string]bool)
		// Trying to match:
		//   [ro.build.version.preview_sdk]: [0]
		//   [ro.build.version.release]: [9]
		re := regexp.MustCompile(`\[([^\]]+)\]: .*`)
		for _, line := range strings.Split(string(out), "\n") {
			property := re.FindStringSubmatch(line)
			if len(property) < 2 {
				continue
			}
			properties[property[1]] = true
		}
		return properties
	}

	// On each ARC boot, ARC detects its boot type (first boot, first boot after
	// OTA, or regular boot) and sets the results as a property. Check that the
	// property is actually set.
	bootType := getProperty(propertyBootType)
	// '3' is from the ArcBootType enum in platform2/arc/setup/arc_setup.h (ARC container) and
	// device/google/bertha/arc-boot-type-detector/main.cc (ARCVM).
	if bootTypeInt, err := strconv.Atoi(bootType); err != nil || bootTypeInt <= 0 || bootTypeInt > 3 {
		s.Errorf("%v property is %q; should contain an ArcBootType enum number",
			propertyBootType, bootType)
	}

	// On unibuild boards, system.raw.img's build.prop contains some template
	// values such as ro.product.board="{metrics-tag}". This check makes sure
	// that such variables are expanded at runtime.
	board := getProperty(propertyBoard)
	if strings.Contains(board, "{") {
		s.Errorf("%v property is %q; should not contain unexpanded values",
			propertyBoard, board)
	}
	// On R, we set a placeholder board name during Android build (ag/11322572).
	// Also make sure the placeholder is not used as-is so we can detect issues
	// like b/157193348#comment5.
	if strings.HasPrefix(board, "bertha_") {
		s.Errorf("%v property is %q; should not start with bertha_",
			propertyBoard, board)
	}

	// Read ro.product.device, drop _cheets suffix, and drop more suffices
	// following '-', like -arcnext or -kernelnext to get the canonical key
	// to map the device name to the first API level.
	//
	// The hyphen subpart conventionally denotes a variance of the same base
	// board that shares the first API level, and moreover they can be truncated
	// (to -kerneln or -ker etc) and becomes hard to match exactly.
	device := getProperty(propertyDevice)
	deviceRegexp := regexp.MustCompile(`^([^-]+).*_(cheets|bertha)$`)
	match := deviceRegexp.FindStringSubmatch(device)
	if match == nil {
		s.Fatalf("%v property is %q; should have _cheets or _bertha suffix",
			propertyDevice, device)
	}
	device = match[1]

	expectedFirstAPILevel := getProperty(propertySDKVersion)
	if getProperty(propertyModel) == "rammus-arc-r" {
		// TODO(b/159985784): Remove the hack once we bring up a board truly
		// setting first_api_level=30.
		//
		// Correct value for rammus-arc-r is the same for rammus (28, obtained
		// from the map), but it is currently put under a special experiment
		// to test behaviors of devices of first API level 30. See b/159114376.
		expectedFirstAPILevel = "30"
	} else if overwrite, ok := expectedFirstAPILevelMap[device]; ok {
		expectedFirstAPILevel = strconv.Itoa(overwrite)
	}

	firstAPILevel := getProperty(propertyFirstAPILevel)
	if firstAPILevel != expectedFirstAPILevel {
		if props, err := a.Command(ctx, "getprop").Output(testexec.DumpLogOnError); err != nil {
			s.Log("Failed to read properties: ", err)
		} else if err := ioutil.WriteFile(filepath.Join(s.OutDir(), "props.txt"), props, 0644); err != nil {
			s.Log("Failed to dump properties: ", err)
		}
		s.Fatalf("Unexpected %v property (see props.txt for details): got %q; want %q", propertyFirstAPILevel,
			firstAPILevel, expectedFirstAPILevel)
	}

	// Verify that these important properties without a partition name still exist.
	propertySuffixes := []string{"fingerprint", "id", "tags", "version.incremental"}
	for _, propertySuffix := range propertySuffixes {
		property := fmt.Sprintf("ro.build.%s", propertySuffix)
		if value := getProperty(property); value == "" {
			s.Fatalf("property %v is not set", property)
		}
	}

	// Starting R, the images have ro.{system,system_ext,product,odm,vendor}.build.* properties
	// by default to allow vendors to customize the values. ARC++ doesn't need the customization
	// and uses the same value for all of them. This verifies that all properties share the same
	// value. On P, the images have only ro.{system,vendor} ones.
	partitions := []string{"system", "system_ext", "product", "odm", "vendor"}
	allProperties := getAllPropertiesMap()
	for property := range allProperties {
		if !strings.HasPrefix(property, "ro.build.") {
			continue
		}
		value := getProperty(property)
		for _, partition := range partitions {
			propertyForPartition := strings.Replace(property, "ro.build.", fmt.Sprintf("ro.%s.build.", partition), 1)
			if !allProperties[propertyForPartition] {
				// ro.build.X exists, but ro.<partition>.build.X doesn't.
				continue
			}
			if valueForPartition := getProperty(propertyForPartition); valueForPartition != value {
				s.Fatalf("Unexpected %v property: got %q; want %q", propertyForPartition, valueForPartition, value)
			}
		}
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
	"kevin64":  arc.SDKN,
	"lars":     arc.SDKN,
	"lulu":     arc.SDKN,
	"nami":     arc.SDKN,
	"nautilus": arc.SDKN,
	"novato":   arc.SDKN,
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
	"atlas":    arc.SDKP,
	"drallion": arc.SDKP,
	"grunt":    arc.SDKP,
	"hatch":    arc.SDKP,
	"jacuzzi":  arc.SDKP,
	"kalista":  arc.SDKP,
	"kukui":    arc.SDKP,
	"nocturne": arc.SDKP,
	"octopus":  arc.SDKP,
	"rammus":   arc.SDKP,
	"sarien":   arc.SDKP,
}
