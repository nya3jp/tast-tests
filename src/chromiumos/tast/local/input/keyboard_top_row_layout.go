// Copyright 2019 The Chromium OS Authors. All rights reserved.
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
	"chromiumos/tast/testing"
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

var (
	// "mapping 1" and "mapping 2" taken from:
	// https://cs.chromium.org/chromium/src/ui/chromeos/events/event_rewriter_chromeos.cc?l=1143&rcl=3028a8be77afd57282d664b6bb07f6d4d01edc55
	mapping1 = TopRowLayout{
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
	mapping2 = TopRowLayout{
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
	mappingWilco = TopRowLayout{
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
	mappingCustom = TopRowLayout{
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
)

// customMapping includes top row layouts for different custom function_row_physmap definitions.
var customMapping = map[string]TopRowLayout{
	// Delbin platform.
	"EA E7 91 92 93 94 95 A0 AE B0": TopRowLayout{
		BrowserBack:    "documents",
		BrowserRefresh: "send",
		ZoomToggle:     "sendfile",
		SelectTask:     "deletefile",
		Screenshot:     "xfer",
		BrightnessDown: "prog1",
		BrightnessUp:   "prog2",
		VolumeMute:     "closecd",
		VolumeDown:     "exit",
		VolumeUp:       "edit",
	},
}

// KeyboardTopRowLayout returns the layout of the top row (function keys) for a given keyboard.
// This is because not all Chromebook keyboards have the same functionality associated to the functions keys.
// As an example, the Toggle Zoom key could be mapped to F3 or F4 depending on the Chromebook model.
func KeyboardTopRowLayout(ctx context.Context, ew *KeyboardEventWriter) (*TopRowLayout, error) {
	props, err := udevProperties(ctx, ew.Device())
	if err != nil {
		return nil, err
	}
	attrs, err := udevAttributes(ctx, ew.Device())
	if err != nil {
		return nil, err
	}

	if physmap, ok := attrs["function_row_physmap"]; ok {
		if val, ok := customMapping[strings.ToUpper(strings.TrimSpace(physmap))]; ok {
			return &val, nil
		}
		testing.ContextLogf(ctx, "Keyboard top row layout for physmap %q is not defined; using default one", physmap)
		return &mappingCustom, nil
	}
	// Logic taken from here:
	// https://source.chromium.org/chromium/chromium/src/+/HEAD:ui/chromeos/events/event_rewriter_chromeos.h;l=56;drc=3e2b7d89ce6261e00e6e723c13c52d0d41bcc69e
	if val, ok := props["CROS_KEYBOARD_TOP_ROW_LAYOUT"]; ok {
		switch val {
		case "1":
			return &mapping1, nil
		case "2":
			return &mapping2, nil
		case "3", "4":
			return &mappingWilco, nil
		default:
			return nil, errors.Errorf("unexpected CROS_KEYBOARD_ROW_LAYOUT: got %s, want [1-4]", val)
		}
	}
	// If keyboard cannot be identified, return mappings1 as defined here:
	// https://source.chromium.org/chromium/chromium/src/+/HEAD:ui/chromeos/events/event_rewriter_chromeos.h;l=172;drc=c537d05a0cc7b74258fe1474260094923b1e4f68
	return &mapping1, nil
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
