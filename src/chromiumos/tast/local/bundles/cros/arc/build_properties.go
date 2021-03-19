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
		Fixture:      "arcBooted",
		Timeout:      4 * time.Minute,
		Attr:         []string{"group:mainline"},
		// Val is the property representing ARC boot type, which is different for container/VM.
		Params: []testing.Param{{
			ExtraSoftwareDeps: []string{"android_p"},
			Val:               "ro.vendor.arc_boot_type",
		}, {
			Name:              "vm",
			ExtraSoftwareDeps: []string{"android_vm"},
			Val:               "vendor.arc.boot_type",
		}},
	})
}

// createPropertiesMatcher returns a function that takes 3 strings and checks if Android properties are set up correctly.
// The returned function first gets a value of the referenceProperty, then for each partition, gets a value of the variant
// of the referenceProperty. For example, when referenceProperty is "ro.build.X", prefixToReplace is "ro.build.",
// replacementFormat is "ro.%s.build.", and partitions is ["P1", "P2"], the returned function checks if the value of
// "ro.build.X" is equal to "ro.P1.build.X" and "ro.P2.build.X".
func createPropertiesMatcher(s *testing.State, allProperties map[string]bool, partitions []string, getProperty func(string) string) func(string, string, string) {
	return func(referenceProperty, prefixToReplace, replacementFormat string) {
		referenceValue := getProperty(referenceProperty)
		for _, partition := range partitions {
			// Generate the property name to check. For example, when referenceProperty is ro.build.X, prefixToReplace is ro.build.,
			// and replacementFormat is ro.%s.build., propertyForPartition will be ro.<partition>.build.X.
			propertyForPartition := strings.Replace(referenceProperty, prefixToReplace, fmt.Sprintf(replacementFormat, partition), 1)
			if !allProperties[propertyForPartition] {
				// The partition doesn't have the referenceProperty equivalent.
				continue
			}
			if valueForPartition := getProperty(propertyForPartition); valueForPartition != referenceValue {
				s.Errorf("Unexpected %v property: got %q; want %q (%v)", propertyForPartition, valueForPartition, referenceValue, referenceProperty)
			}
		}
	}
}

func BuildProperties(ctx context.Context, s *testing.State) {
	const (
		propertyBoard         = "ro.product.board"
		propertyDevice        = "ro.product.device"
		propertyFirstAPILevel = "ro.product.first_api_level"
		propertyModel         = "ro.product.model"
		propertySDKVersion    = "ro.build.version.sdk"
	)

	a := s.FixtValue().(*arc.PreData).ARC

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
	propertyBootType := s.Param().(string)
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
	deviceRegexp := regexp.MustCompile(`^([^-]+).*_cheets$`)
	match := deviceRegexp.FindStringSubmatch(device)
	if match == nil {
		s.Errorf("%v property is %q; should have _cheets suffix",
			propertyDevice, device)
	}
	device = match[1]

	expectedFirstAPILevel := getProperty(propertySDKVersion)
	if device == "rammus" && strings.HasSuffix(board, "-arc-r") {
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
		s.Errorf("Unexpected %v property (see props.txt for details): got %q; want %q", propertyFirstAPILevel,
			firstAPILevel, expectedFirstAPILevel)
	}

	// Verify that these important properties without a partition name still exist.
	propertySuffixes := []string{"fingerprint", "id", "tags", "version.incremental"}
	for _, propertySuffix := range propertySuffixes {
		property := fmt.Sprintf("ro.build.%s", propertySuffix)
		value := getProperty(property)
		if value == "" {
			s.Errorf("property %v is not set", property)
		}
		// Check that ro.build.fingerprint doesn't contain '_bertha' even when the device uses ARCVM (b/152775858)
		if propertySuffix == "fingerprint" && !strings.Contains(value, "_cheets") {
			s.Errorf("%v property is %q; should contain _cheets", property, value)
		}
	}

	partitions := []string{"system", "system_ext", "product", "odm", "vendor", "bootimage"}
	allProperties := getAllPropertiesMap()
	propertiesMatcher := createPropertiesMatcher(s, allProperties, partitions, getProperty)

	// Starting R, the images have ro.[property.]{system,system_ext,product,odm,vendor,bootimage}.*
	// properties by default to allow vendors to customize the values. ARC doesn't need the
	// customization and uses the same value for all of them. This verifies that all properties
	// share the same value. On P, the images have only ro.{system,vendor,bootimage} ones.
	for property := range allProperties {
		if prefix := "ro.build."; strings.HasPrefix(property, prefix) {
			// Verify that ro.build.X has the same value as ro.<partition>.build.X.
			propertiesMatcher(property, prefix, "ro.%s.build.")
		} else if prefix := "ro.product."; strings.HasPrefix(property, prefix) {
			// Verify that ro.product.X has the same value as ro.<partition>.product.X.
			propertiesMatcher(property, prefix, "ro.%s.product.")
			// Verify that ro.product.X has the same value as ro.product.<partition>.X.
			propertiesMatcher(property, prefix, "ro.product.%s.")
		}
	}
}

// Map of device name -> expected first API level.
// First API level is expected to be the same as current SDK version if the
// device name doesn't exist in this map.
var expectedFirstAPILevelMap = map[string]int{
	// N boards (sorted alphabetically)
	"asuka":    arc.SDKN,
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
	"fievel":   arc.SDKN,
	"fizz":     arc.SDKN,
	"gandof":   arc.SDKN,
	"hana":     arc.SDKN,
	"jerry":    arc.SDKN,
	"kefka":    arc.SDKN,
	"kevin":    arc.SDKN,
	"kevin64":  arc.SDKN,
	"lars":     arc.SDKN,
	"lulu":     arc.SDKN,
	"mighty":   arc.SDKN,
	"minnie":   arc.SDKN,
	"nami":     arc.SDKN,
	"nautilus": arc.SDKN,
	"novato":   arc.SDKN,
	"paine":    arc.SDKN,
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
	"tiger":    arc.SDKN,
	"ultima":   arc.SDKN,
	"wizpig":   arc.SDKN,
	"yuna":     arc.SDKN,
	// P boards (sorted alphabetically)
	"asurada":   arc.SDKP,
	"atlas":     arc.SDKP,
	"dedede":    arc.SDKP,
	"drallion":  arc.SDKP,
	"grunt":     arc.SDKP,
	"hatch":     arc.SDKP,
	"jacuzzi":   arc.SDKP,
	"kalista":   arc.SDKP,
	"kukui":     arc.SDKP,
	"nocturne":  arc.SDKP,
	"octopus":   arc.SDKP,
	"puff":      arc.SDKP,
	"rammus":    arc.SDKP,
	"sarien":    arc.SDKP,
	"strongbad": arc.SDKP,
	"trogdor":   arc.SDKP,
	"volteer":   arc.SDKP,
	"zork":      arc.SDKP,
}
