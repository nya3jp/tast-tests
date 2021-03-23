// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package arc

import (
	"context"
	"fmt"
	"math"
	"regexp"
	"strconv"
	"strings"

	"chromiumos/tast/common/testexec"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/coords"
	"chromiumos/tast/testing"
)

const (
	// DefaultDisplayID represents the display ID for the internal display.
	DefaultDisplayID = 0
	// CaptionHeightR represents the caption height in ChromeDP which is defined in ArcSystemUIConstants.
	// TODO: Replace hard code caption height by getting from ash.
	CaptionHeightR = 32
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

	version, err := SDKVersion()
	if err != nil {
		return -1, err
	}
	switch version {
	case SDKP:
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
	case SDKR:
		uniqueID, err := scrapeUniqueID(output, d.DisplayID)
		if err != nil {
			return -1, errors.Wrap(err, "failed to parse display unique id")
		}
		waylandCmd := d.a.Command(ctx, "dumpsys", "Wayland")
		waylandOutput, err := waylandCmd.Output(testexec.DumpLogOnError)
		if err != nil {
			return -1, errors.Wrap(err, "failed to execute 'dumpsys Wayland'")
		}
		scaleFactor, err := scrapeScaleFactor(waylandOutput, uniqueID)
		return int(math.Round(CaptionHeightR * scaleFactor)), nil
	default:
		return -1, errors.Errorf("unsupported Android version %d", version)
	}
}

// scrapeScaleFactor returns the scale factor from `dumpsys Wayland`.
// Only works for ARC R and later version.
func scrapeScaleFactor(output []byte, uniqueDisplayID string) (scaleFactor float64, err error) {
	uniqueDisplayIDWithoutPrefix := strings.TrimPrefix(uniqueDisplayID, "local:")
	// Looking for:
	// Display Layout
	//  ...
	//  Display ... (SF display 21536137753913600, default scale 2.666, zoom factor 1) ...
	s := fmt.Sprintf(`(?m)`+ // Enable multiline.
		`Display Layout`+ // Match Display Layout section.
		`(?:\s+.*$)*`+ // Skip entries lines...
		`\s*.+SF display %s`+ // ...until matched corresponding display unique id.
		`, default scale (\d+\.?\d*)`+ // Gather default scale.
		`, zoom factor (\d+\.?\d*)`, uniqueDisplayIDWithoutPrefix) // Gather zoom factor.
	re := regexp.MustCompile(s)
	groups := re.FindStringSubmatch(string(output))
	if len(groups) != 3 {
		return 0, errors.Errorf("failed to parse 'dumpsys Wayland' %v", groups)
	}
	defaultScale, err := strconv.ParseFloat(groups[1], 64)
	if err != nil {
		return 0, errors.Wrap(err, "failed to parse default scale value")
	}
	zoomFactor, err := strconv.ParseFloat(groups[2], 64)
	if err != nil {
		return 0, errors.Wrap(err, "failed to parse zoom factor value")
	}
	return defaultScale * zoomFactor, nil
}

func scrapeDensity(output []byte, displayID, sdkVersion int) (density float64, err error) {
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
		uniqueID, err := scrapeUniqueID(output, displayID)
		if err != nil {
			return -1, err
		}
		// In Android R, we are looking for:
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
	return scrapeDensity(output, d.DisplayID, n)
}

// Size returns the display size. Takes into account possible orientation changes.
// For example, if the display is rotated, instead of returning {W, H}, it will return {H, W}.
func (d *Display) Size(ctx context.Context) (s coords.Size, err error) {
	var cmd *testexec.Cmd
	sdkVersion, err := SDKVersion()
	if err != nil {
		return coords.Size{}, err
	}

	if sdkVersion <= SDKP {
		cmd = d.a.Command(ctx, "dumpsys", "window", "displays")
	} else {
		cmd = d.a.Command(ctx, "dumpsys", "display")
	}

	output, err := cmd.Output(testexec.DumpLogOnError)
	if err != nil {
		return coords.Size{}, errors.Wrap(err, "failed to execute 'dumpsys window displays'")
	}

	return scrapeDisplaySize(output, false, d.DisplayID, sdkVersion)
}

// stableSize returns the display size. It is not affected by display rotations.
// It always returns the coordinates in this order: {W, H}.
func (d *Display) stableSize(ctx context.Context) (s coords.Size, err error) {
	cmd := d.a.Command(ctx, "dumpsys", "display")
	output, err := cmd.Output(testexec.DumpLogOnError)
	if err != nil {
		return coords.Size{}, errors.Wrap(err, "failed to execute 'dumpsys display'")
	}

	n, err := SDKVersion()
	if err != nil {
		return coords.Size{}, err
	}

	return scrapeDisplaySize(output, true, d.DisplayID, n)
}

// scrapeDisplaySize returns the display size parsed from dumpsys.
// If isStableSize is true, it will return the stable display size. The stable size will not be
// affected by orientation changes, otherwise the size will be the logical size.
func scrapeDisplaySize(output []byte, isStableSize bool, displayID, sdkVersion int) (s coords.Size, err error) {
	var re *regexp.Regexp
	switch sdkVersion {
	case SDKP:
		if isStableSize {
			// For ARC P, from `dumpsys display` looking for:
			// DISPLAY MANAGER (dumpsys display)
			//   mOnlyCode=false
			//   [skipping some properties]
			//   mStableDisplaySize=Point(2400, 1600)
			re = regexp.MustCompile(`(?m)` + // Enable multiline.
				`^\s*DISPLAY MANAGER \(dumpsys display\)\n` + // Match DISPLAY MANAGER
				`(?:\s+.*$)*` + // Skip entire lines...
				`\s+mStableDisplaySize=\w*\((\d*),\s*(\d*)\)`) // Gather 'mStableDisplaySize=' bounds.
		} else {
			// For ARC P, from `dumpsys window display` looking for:
			// Display: mDisplayId=0
			//   init=2400x1600 240dpi cur=2400x1600 app=2400x1424 rng=1600x1424-2400x2224
			re = regexp.MustCompile(`(?m)` + // Enable multiline.
				`^\s*Display: mDisplayId=0\n` + // Match displayId 0 (internal display).
				`\s*init=([0-9]+)x([0-9]+)`) // Gather 'init=' bounds.
		}
	case SDKR:
		// For ARC R, dump output from `dumpsys display`
		if isStableSize {
			// Looking for:
			//   mDisplayId=...
			//   ...
			//   mBaseDisplayInfo=DisplayInfo{... , real 3840 x 2160, ...}
			s := fmt.Sprintf(`(?m)`+ // Enable multiline.
				`mDisplayId=%d`+ // Gather displayId number.
				`(?:\s+.*$)*?\s+`+ // Skip lines and words.
				`mBaseDisplayInfo=.+?real ([0-9]+) x ([0-9]+)`, displayID) // Locate to base real size string.
			re = regexp.MustCompile(s)
		} else {
			// Looking for:
			//   mDisplayId=...
			//   ...
			//   mOverrideDisplayInfo=DisplayInfo{... , real 2160 x 3840, ...}
			s := fmt.Sprintf(`(?m)`+ // Enable multiline.
				`mDisplayId=%d`+ // Gather displayId number.
				`(?:\s+.*$)*?\s+`+ // Skip lines and words.
				`mOverrideDisplayInfo=.+?real ([0-9]+) x ([0-9]+)`, displayID) // Locate to override real size string.
			re = regexp.MustCompile(s)
		}
	default:
		return coords.Size{}, errors.Errorf("unsupported Android version %d", sdkVersion)
	}

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

// scrapeUniqueID returns unique id by display id.
// Only works for ARC R and later version.
func scrapeUniqueID(output []byte, displayID int) (string, error) {
	// Looking for:
	//   mDisplayId=...
	//   ...
	//   mBaseDisplayInfo=DisplayInfo{... , uniqueId "local:1886094531531010", ...}
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
