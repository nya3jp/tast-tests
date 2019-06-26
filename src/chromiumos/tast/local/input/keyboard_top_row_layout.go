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

	"chromiumos/tast/errors"
	"chromiumos/tast/local/testexec"
)

// TopRowLayout represents the top row layout of the Chromebook keyboard.
// Each entry represents the key it is mapped to. An empty string means that is is not mapped.
type TopRowLayout struct {
	BrowserBack    string
	BrowserForward string
	BrowserRefresh string
	ZoomToggle     string
	SelectTask     string
	BrightnessDown string
	BrightnessUp   string
	MediaPlayPause string
	VolumeMute     string
	VolumeDown     string
	VolumeUp       string
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

	props, err := uDevProperties(ctx, ew.Device())
	if err != nil {
		return nil, err
	}

	// mapping2 is only returned when CROS_KEYBOARD_TOP_ROW_LAYOUT=2. Any other condition returns mapping1. See:
	// https://cs.chromium.org/chromium/src/ui/chromeos/events/event_rewriter_chromeos.cc?l=1211&rcl=3028a8be77afd57282d664b6bb07f6d4d01edc55
	if val, ok := props["CROS_KEYBOARD_TOP_ROW_LAYOUT"]; ok {
		if val == "2" {
			return &mapping2, nil
		}
	}
	return &mapping1, nil
}

// uDevProperties returns the properties associated to a certain Linux udev device.
func uDevProperties(ctx context.Context, devicePath string) (map[string]string, error) {
	cmd := testexec.CommandContext(ctx, "udevadm", "info", "--query", "property", "--name", devicePath)
	out, err := cmd.Output()
	if err != nil {
		cmd.DumpLog(ctx)
		return nil, errors.Errorf("failed running %q", strings.Join(cmd.Args, " "))
	}
	return parseUdev(out)
}

// parseUdev parses the raw output from udevadm and converts it into a key-value map.
func parseUdev(r []byte) (map[string]string, error) {
	// lineRe matches a key-value line.
	var lineRe = regexp.MustCompile(`^([A-Z0-9_]+)\s*=\s*(.*)$`)

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
