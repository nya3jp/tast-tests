// Copyright 2019 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package input

import (
	"bufio"
	"bytes"
	"context"
	"regexp"
	"strings"

	"chromiumos/tast/common/testexec"
	"chromiumos/tast/errors"
)

// TopRowLayout represents the top row layout of the Chromebook keyboard.
// Each entry represents the key it is mapped to. An empty string means that is is not mapped.
type TopRowLayout struct {
	BrowserBack    string
	BrowserForward string
	BrowserRefresh string
	ZoomToggle     string
	SelectTask     string
	Screenshot     string
	BrightnessDown string
	BrightnessUp   string
	MediaPlayPause string
	VolumeMute     string
	VolumeDown     string
	VolumeUp       string
	MediaLaunchApp string
}

// TopRowLayoutType represents the top row keyboard type.
type TopRowLayoutType int

// Following constants represent different top row keyboard layouts taken from here:
// https://source.chromium.org/chromium/chromium/src/+/main:ui/chromeos/events/event_rewriter_chromeos.h;l=54;drc=f54415214d7d3ed03d2e9a1382d633abb3a44f47
const (
	// LayoutUnknown must be 0 so KeyboardEventWriter is initialized correctly
	LayoutUnknown TopRowLayoutType = iota
	Layout1
	Layout2
	LayoutWilco
	LayoutDrallion
	LayoutCustom
)

// Scan codes taken from chromeos keyboard spec:
// https://chromeos.google.com/partner/dlm/docs/hardware-specs/keyboardspec.html
var topRowScanCodeMap = map[EventCode]int32{
	KEY_BACK:                  0xea,
	KEY_REFRESH:               0xe7,
	KEY_FULL_SCREEN:           0x91,
	KEY_SCALE:                 0x92,
	KEY_SYSRQ:                 0x93,
	KEY_BRIGHTNESSDOWN:        0x94,
	KEY_BRIGHTNESSUP:          0x95,
	KEY_PRIVACY_SCREEN_TOGGLE: 0x96,
	KEY_MUTE:                  0xa0,
	KEY_VOLUMEDOWN:            0xae,
	KEY_VOLUMEUP:              0xb0,
	KEY_KBDILLUMDOWN:          0x97,
	KEY_KBDILLUMUP:            0x98,
	KEY_NEXTSONG:              0x99,
	KEY_PREVIOUSSONG:          0x90,
	KEY_PLAYPAUSE:             0x9a,
	KEY_FORWARD:               0xe9,
	KEY_SLEEP:                 0xaf,
}

// KeyboardTopRowLayout returns the layout of the top row (function keys) for a given keyboard.
// This is because not all Chromebook keyboards have the same functionality associated to the functions keys.
// As an example, the Toggle Zoom key could be mapped to F3 or F4 depending on the Chromebook model.
func KeyboardTopRowLayout(ctx context.Context, ew *KeyboardEventWriter) (*TopRowLayout, error) {
	// "mapping 1" and "mapping 2" taken from:
	// https://cs.chromium.org/chromium/src/ui/chromeos/events/event_rewriter_chromeos.cc?l=1143&rcl=3028a8be77afd57282d664b6bb07f6d4d01edc55
	mapping1 := TopRowLayout{
		BrowserBack:    "F1",
		BrowserForward: "F2",
		BrowserRefresh: "F3",
		ZoomToggle:     "F4",
		SelectTask:     "F5",
		BrightnessDown: "F6",
		BrightnessUp:   "F7",
		VolumeMute:     "F8",
		VolumeDown:     "F9",
		VolumeUp:       "F10",
	}

	// BROWSER_FORWARD removed, MEDIA_PLAY_PAUSE added.
	mapping2 := TopRowLayout{
		BrowserBack:    "F1",
		BrowserRefresh: "F2",
		ZoomToggle:     "F3",
		SelectTask:     "F4",
		BrightnessDown: "F5",
		BrightnessUp:   "F6",
		MediaPlayPause: "F7",
		VolumeMute:     "F8",
		VolumeDown:     "F9",
		VolumeUp:       "F10",
	}

	// Wilco mappings taken from:
	// https://source.chromium.org/chromium/chromium/src/+/HEAD:ui/chromeos/events/event_rewriter_chromeos.cc;drc=3e2b7d89ce6261e00e6e723c13c52d0d41bcc69e;l=1599
	// MEDIA_PLAY_PAUSE removed, MEDIA_LAUNCH_APP2 added.
	mappingWilcoAndDrallion := TopRowLayout{
		BrowserBack:    "search+F1",
		BrowserRefresh: "search+F2",
		ZoomToggle:     "search+F3",
		SelectTask:     "search+F4",
		BrightnessDown: "search+F5",
		BrightnessUp:   "search+F6",
		VolumeMute:     "search+F7",
		VolumeDown:     "search+F8",
		VolumeUp:       "search+F9",
		MediaLaunchApp: "search+F12",
	}

	// This is the minimal set of required top row keys for custom top row
	// layouts.
	mappingCustom := TopRowLayout{
		BrowserBack:    "back",
		BrowserRefresh: "refresh",
		ZoomToggle:     "fullscreen",
		SelectTask:     "scale",
		Screenshot:     "sysrq",
		BrightnessDown: "brightnessdown",
		BrightnessUp:   "brightnessup",
		VolumeMute:     "mute",
		VolumeDown:     "volumedown",
		VolumeUp:       "volumeup",
	}

	layout, err := GetTopRowLayoutType(ctx, ew)
	if err != nil {
		return nil, err
	}

	switch layout {
	case Layout1:
		return &mapping1, nil
	case Layout2:
		return &mapping2, nil
	case LayoutWilco:
		return &mappingWilcoAndDrallion, nil
	case LayoutDrallion:
		return &mappingWilcoAndDrallion, nil
	case LayoutCustom:
		return &mappingCustom, nil
	// If for some reason our layout does not match any of our known layouts, return mapping for Layout1.
	default:
		return &mapping1, nil
	}
}

// GetTopRowLayoutType returns the TopRowLayoutType for a given KeyboardEventWriter.
// If the type had been discovered before, this function uses the cached value within the KeyboardEventWriter.
func GetTopRowLayoutType(ctx context.Context, ew *KeyboardEventWriter) (TopRowLayoutType, error) {
	getTopRowLayoutTypeHelper := func(ctx context.Context, ew *KeyboardEventWriter) (TopRowLayoutType, error) {
		props, err := udevProperties(ctx, ew.Device())
		if err != nil {
			return Layout1, err
		}
		attrs, err := udevAttributes(ctx, ew.Device())
		if err != nil {
			return Layout1, err
		}

		// Logic taken from here:
		// https://source.chromium.org/chromium/chromium/src/+/HEAD:ui/chromeos/events/event_rewriter_chromeos.h;l=56;drc=3e2b7d89ce6261e00e6e723c13c52d0d41bcc69e
		if _, ok := attrs["function_row_physmap"]; ok {
			return LayoutCustom, nil
		}
		if val, ok := props["CROS_KEYBOARD_TOP_ROW_LAYOUT"]; ok {
			switch val {
			case "1":
				return Layout1, nil
			case "2":
				return Layout2, nil
			case "3":
				return LayoutWilco, nil
			case "4":
				return LayoutDrallion, nil
			default:
				return Layout1, errors.Errorf("unexpected CROS_KEYBOARD_ROW_LAYOUT: got %s, want [1-4]", val)
			}
		}
		// If keyboard cannot be identified, return Layout1 as defined here:
		// https://source.chromium.org/chromium/chromium/src/+/HEAD:ui/chromeos/events/event_rewriter_chromeos.h;l=172;drc=c537d05a0cc7b74258fe1474260094923b1e4f68
		return Layout1, nil
	}

	if ew.topRowLayoutType != LayoutUnknown {
		return ew.topRowLayoutType, nil
	}

	layout, err := getTopRowLayoutTypeHelper(ctx, ew)
	ew.topRowLayoutType = layout
	return layout, err
}

// udevAttributes returns the attributes associated to a certain Linux udev device.
func udevAttributes(ctx context.Context, devicePath string) (map[string]string, error) {
	cmd := testexec.CommandContext(ctx, "udevadm", "info", "--attribute-walk", "property", "--name", devicePath)
	out, err := cmd.Output()
	if err != nil {
		cmd.DumpLog(ctx)
		return nil, errors.Errorf("failed running %q", strings.Join(cmd.Args, " "))
	}
	return parseUdev(out, `^ATTRS{([a-z0-9_]+)}=="(.*)"$`)
}

// udevProperties returns the properties associated to a certain Linux udev device.
func udevProperties(ctx context.Context, devicePath string) (map[string]string, error) {
	cmd := testexec.CommandContext(ctx, "udevadm", "info", "--query", "property", "--name", devicePath)
	out, err := cmd.Output()
	if err != nil {
		cmd.DumpLog(ctx)
		return nil, errors.Errorf("failed running %q", strings.Join(cmd.Args, " "))
	}
	return parseUdev(out, `^([A-Z0-9_]+)\s*=\s*(.*)$`)
}

// parseUdev parses the raw output from udevadm and converts it into a key-value map.
func parseUdev(r []byte, matchPattern string) (map[string]string, error) {
	// lineRe matches a key-value line.
	var lineRe = regexp.MustCompile(matchPattern)

	kvs := make(map[string]string)
	sc := bufio.NewScanner(bytes.NewReader(r))
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line == "" {
			continue
		}
		m := lineRe.FindStringSubmatch(line)
		if m == nil {
			continue
		}
		kvs[m[1]] = m[2]
	}
	if err := sc.Err(); err != nil {
		return nil, err
	}
	return kvs, nil
}
