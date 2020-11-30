// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package arc

import (
	"context"
	"fmt"
	"regexp"
	"strconv"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/coords"
	"chromiumos/tast/local/testexec"
	"chromiumos/tast/testing"
)

const (
	// DefaultDisplayID represents the display ID for the internal display.
	DefaultDisplayID = 0
)

// Display holds resources related to an ARC display.
// For the moment only the default display (internal display) is supported.
type Display struct {
	a         *ARC // Close is not called here
	DisplayID int
}

// DisplayType represents corresponding ARC display type string from dumpsys display.
type DisplayType string

// DisplayType available on R and above versions.
const (
	// Internal display.
	InternalDisplay DisplayType = "INTERNAL"
	// Virtual display.
	VirtualDisplay DisplayType = "VIRTUAL"
	// External display.
	ExternalDisplay DisplayType = "EXTERNAL"
)

// NewDisplay returns a new Display instance.
// The caller is responsible for closing a.
// Returned Display instance must be closed when the test is finished.
func NewDisplay(a *ARC, displayID int) (*Display, error) {
	return &Display{a, displayID}, nil
}

// Close closes resources related to the Display instance.
func (d *Display) Close() {
	// Blank on purpose. Function added for forward-compatibility.
}

// FirstDisplayIDByType returns first ARC display id for specific display type.
func FirstDisplayIDByType(ctx context.Context, a *ARC, displayType DisplayType) (int, error) {
	sdkVersion, err := SDKVersion()

	// Return default value of displayID for old ARC version.
	if sdkVersion == SDKP {
		switch displayType {
		case InternalDisplay:
			return 0, nil
		case ExternalDisplay:
			return 1, nil
		case VirtualDisplay:
		default:
			return -1, errors.Errorf("not supported display type %q", displayType)
		}
	}

	// Parse from dumpsys for ARC R and above.
	cmd := a.Command(ctx, "dumpsys", "display")
	output, err := cmd.Output(testexec.DumpLogOnError)
	if err != nil {
		return -1, errors.Wrap(err, "failed to execute 'dumpsys display'")
	}

	// Looking for:
	// mDisplayId=...
	// ...
	// mBaseDisplayInfo=DisplayInfo{... type EXTERNAL, ...}
	re := regexp.MustCompile(`(?m)` + // Enable multiline.
		`mDisplayId=(\d+)` + // Gather displayId number.
		`(?:\s+.*$)*?\s+` + // Skip lines and words.
		`mBaseDisplayInfo=[\W\w]+?type ` + // Locate to type string.
		`([\W\w]+?),`) // Gather type string.
	groups := re.FindAllStringSubmatch(string(output), -1)
	if len(groups) == 0 {
		testing.ContextLogf(ctx, "Failed to parse display info from dumpsys output: %q", output)
		return -1, errors.New("failed to find any display from `dumpsys display`")
	}

	for _, group := range groups {
		id, err := strconv.Atoi(group[1])
		if err != nil {
			return -1, errors.Wrapf(err, "failed to parse display id: %q", group[1])
		}
		if group[2] == string(displayType) {
			return id, nil
		}
	}
	return -1, errors.Errorf("failed to find display with type %q", displayType)
}

// CaptionHeight returns the caption height in pixels.
func (d *Display) CaptionHeight(ctx context.Context) (h int, err error) {
	cmd := d.a.Command(ctx, "dumpsys", "display")
	output, err := cmd.Output(testexec.DumpLogOnError)
	if err != nil {
		return -1, errors.Wrap(err, "failed to execute 'dumpsys display'")
	}

	// Looking for:
	// ARC Display Configuration
	//  primaryDisplayId=0
	//  layoutMode=clamshell
	//  captionHeight=72
	re := regexp.MustCompile(`(?m)` + // Enable multiline.
		`^ARC Display Configuration\n` + // Match ARC Display section.
		`(?:\s+.*$)*` + // Skip entire lines...
		`\s*captionHeight=(\w*)`) // ...until captionHeight is matched.
	groups := re.FindStringSubmatch(string(output))
	if len(groups) != 2 {
		return -1, errors.New("failed to parse 'dumpsys display'")
	}
	i, err := strconv.Atoi(groups[1])
	if err != nil {
		return -1, errors.Wrap(err, "failed to parse captionHeight value")
	}
	return i, nil
}

func scrapeDensity(displayID int, output []byte, sdkVersion int) (density float64, err error) {
	var re *regexp.Regexp
	switch sdkVersion {
	case SDKP:
		// In Android P, we are looking for:
		// Display Devices: size=1
		//  DisplayDeviceInfo
		//   mDisplayInfos=
		//    PhysicalDisplayInfo{..., density 1.5, ...}
		re = regexp.MustCompile(`(?m)` + // Enable multiline.
			`^Display Devices: size=1\n` + // Match Display Devices section.
			`(?:\s+.*$)*` + // Skip entire lines...
			`\s*PhysicalDisplayInfo{.*density (\d\.\d+)?`) // ...until density is matched.
	case SDKR:
		uniqueID, err := scrapeUniqueID(displayID, output, sdkVersion)
		if err != nil {
			return -1, err
		}
		// In Android R, we are looking for:
		// Display Devices: size=2
		//   DisplayDeviceInfo{...: uniqueId="local:1886094531531010", ...}
		//     ...
		//     mDisplayInfo=DisplayInfo{..., density=2.0, ...}
		s := fmt.Sprintf(`(?m)`+ // Enable multiline.
			`^\s+DisplayDeviceInfo{.+uniqueId="%s".+$`+ // Match Display Devices section.
			`(?:\s+.*$)+?`+ // Skip entire lines...
			`\s+mDisplayInfo=.+density=(\d\.\d+)?`, uniqueID) // ...until density is matched.
		re = regexp.MustCompile(s)
	default:
		return -1, errors.Errorf("unsupported Android version %d", sdkVersion)
	}

	groups := re.FindStringSubmatch(string(output))
	if len(groups) != 2 {
		return -1, errors.New("failed to parse 'dumpsys display'")
	}
	f, err := strconv.ParseFloat(groups[1], 64)
	if err != nil {
		return -1, errors.Wrap(err, "failed to parse Physical Display Info density value")
	}
	return f, nil
}

// PhysicalDensity returns the density value in PhysicalDisplayInfo.
func (d *Display) PhysicalDensity(ctx context.Context) (density float64, err error) {
	output, err := d.a.Command(ctx, "dumpsys", "display").Output(testexec.DumpLogOnError)
	if err != nil {
		return -1, errors.Wrap(err, "failed to execute 'dumpsys display'")
	}
	n, err := SDKVersion()
	if err != nil {
		return -1, err
	}
	return scrapeDensity(d.DisplayID, output, n)
}

// Size returns the display size. Takes into account possible orientation changes.
// For example, if the display is rotated, instead of returning {W, H}, it will return {H, W}.
func (d *Display) Size(ctx context.Context) (s coords.Size, err error) {
	cmd := d.a.Command(ctx, "dumpsys", "window", "displays")
	output, err := cmd.Output(testexec.DumpLogOnError)
	if err != nil {
		return coords.Size{}, errors.Wrap(err, "failed to execute 'dumpsys window displays'")
	}

	// Looking for:
	// Display: mDisplayId=0
	//   init=2400x1600 240dpi cur=2400x1600 app=2400x1424 rng=1600x1424-2400x2224
	re := regexp.MustCompile(`(?m)` + // Enable multiline.
		`^\s*Display: mDisplayId=0\n` + // Match displayId 0 (internal display).
		`\s*init=([0-9]+)x([0-9]+)`) // Gather 'init=' bounds.
	groups := re.FindStringSubmatch(string(output))
	if len(groups) != 3 {
		return coords.Size{}, errors.New("failed to parse dumpsys output")
	}

	width, err := strconv.Atoi(groups[1])
	if err != nil {
		return coords.Size{}, errors.Wrap(err, "could not parse bounds")
	}
	height, err := strconv.Atoi(groups[2])
	if err != nil {
		return coords.Size{}, errors.Wrap(err, "could not parse bounds")
	}

	return coords.NewSize(width, height), nil
}

// stableSize returns the display size. It is not affected by display rotations.
// It always returns the coordinates in this order: {W, H}.
func (d *Display) stableSize(ctx context.Context) (s coords.Size, err error) {
	cmd := d.a.Command(ctx, "dumpsys", "display")
	output, err := cmd.Output(testexec.DumpLogOnError)
	if err != nil {
		return coords.Size{}, errors.Wrap(err, "failed to execute 'dumpsys display'")
	}

	// Looking for:
	// DISPLAY MANAGER (dumpsys display)
	//   mOnlyCode=false
	//   [skipping some properties]
	//   mStableDisplaySize=Point(2400, 1600)
	re := regexp.MustCompile(`(?m)` + // Enable multiline.
		`^\s*DISPLAY MANAGER \(dumpsys display\)\n` + // Match DISPLAY MANAGER
		`(?:\s+.*$)*` + // Skip entire lines...
		`\s+mStableDisplaySize=\w*\((\d*),\s*(\d*)\)`) // Gather 'mStableDisplaySize=' bounds.
	groups := re.FindStringSubmatch(string(output))
	if len(groups) != 3 {
		return coords.Size{}, errors.New("failed to parse dumpsys output")
	}

	width, err := strconv.Atoi(groups[1])
	if err != nil {
		return coords.Size{}, errors.Wrap(err, "could not parse bounds")
	}
	height, err := strconv.Atoi(groups[2])
	if err != nil {
		return coords.Size{}, errors.Wrap(err, "could not parse bounds")
	}

	return coords.NewSize(width, height), nil
}

func scrapeUniqueID(displayID int, output []byte, sdkVersion int) (string, error) {
	if sdkVersion < SDKR {
		return "", errors.Errorf("unsupported Android version %d", sdkVersion)
	}

	// Looking for:
	// mDisplayId=...
	// ...
	// mBaseDisplayInfo=DisplayInfo{... , uniqueId "local:1886094531531010", ...}
	s := fmt.Sprintf(`(?m)`+ // Enable multiline.
		`mDisplayId=%d`+ // Gather displayId number.
		`(?:\s+.*$)*?\s+`+ // Skip lines and words.
		`mBaseDisplayInfo=[\W\w]+?uniqueId "(.+)"`, displayID) // Locate to uniqueId string.

	re := regexp.MustCompile(s)
	groups := re.FindStringSubmatch(string(output))
	if len(groups) != 2 {
		return "", errors.New("failed to parse 'dumpsys display'")
	}
	return string(groups[1]), nil
}
