// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package arc

import (
	"context"
	"regexp"
	"strconv"

	"chromiumos/tast/errors"
)

const (
	// ClamshellMode when the display is in clamshell mode.
	ClamshellMode = 0
	// TabletMode when the display is in tablet mode.
	TabletMode = 1

	// DefaultDisplayID represents the display ID for the internal display.
	DefaultDisplayID = 0
)

// Display holds resources related to an ARC display.
type Display struct {
	ctx       context.Context
	a         *ARC
	displayID int
}

// NewDisplay returns a new Display instance.
// Returned Display instance must be closed when the test is finished.
func NewDisplay(ctx context.Context, a *ARC, displayID int) (*Display, error) {
	if displayID != DefaultDisplayID {
		return nil, errors.New("only displayID 0 is supported at the moment")
	}
	return &Display{ctx: ctx, a: a, displayID: displayID}, nil
}

// Close closes resources related to the Display instance.
func (disp *Display) Close() {
}

// Mode returns the display mode, either TabletMode or ClamshellMode.
func (disp *Display) Mode() (int, error) {
	cmd := disp.a.Command(disp.ctx, "dumpsys", "display")
	output, err := cmd.Output()
	if err != nil {
		return -1, errors.Wrap(err, "failed to execute 'dumpsys display'")
	}

	// Looking for:
	// ARC Display Configuration
	//  primaryDisplayId=0
	//  layoutMode=clamshell
	//  captionHeight=72
	re := regexp.MustCompile(`(?m)` + // Enable multiline.
		`^ARC Display Configuration$` + // Match ARC Display section.
		`(?:\s*.*$)*` + // Skip entire lines...
		`\s*layoutMode=(\w*)`) // ...until layoutMode is matched.
	groups := re.FindStringSubmatch(string(output))
	if len(groups) != 2 {
		return -1, errors.New("failed to parse 'dumpsys display'")
	}
	if groups[1] == "tablet" {
		return TabletMode, nil
	}
	return ClamshellMode, nil
}

// CaptionHeight returns the caption height in pixels.
func (disp *Display) CaptionHeight() (int, error) {
	cmd := disp.a.Command(disp.ctx, "dumpsys", "display")
	output, err := cmd.Output()
	if err != nil {
		return -1, errors.Wrap(err, "failed to execute 'dumpsys display'")
	}

	// Looking for:
	// ARC Display Configuration
	//  primaryDisplayId=0
	//  layoutMode=clamshell
	//  captionHeight=72
	re := regexp.MustCompile(`(?m)` + // Enable multiline.
		`^ARC Display Configuration$` + // Match ARC Display section.
		`(?:\s*.*$)*` + // Skip entire lines...
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

// Bounds returns the display bounds. Takes into account possible orientation changes.
func (disp *Display) Bounds() (int, int, error) {
	cmd := disp.a.Command(disp.ctx, "dumpsys", "window", "displays")
	output, err := cmd.Output()
	if err != nil {
		return 0, 0, errors.Wrap(err, "failed to launch dumpsys")
	}

	// Looking for:
	// Display: mDisplayId=0
	//   init=2400x1600 240dpi cur=2400x1600 app=2400x1424 rng=1600x1424-2400x2224
	re := regexp.MustCompile(`(?m)` + // Enable multiline.
		`^\s*Display: mDisplayId=0$` + // Match displayId 0 (internal display).
		`\s*init=([0-9]+)x([0-9]+)`) // Gather 'init=' bounds.
	groups := re.FindStringSubmatch(string(output))
	if len(groups) != 3 {
		return 0, 0, errors.New("failed to parse dumpsys output")
	}

	width, err := strconv.Atoi(groups[1])
	if err != nil {
		return 0, 0, errors.Wrap(err, "could not parse bounds")
	}
	height, err := strconv.Atoi(groups[2])
	if err != nil {
		return 0, 0, errors.Wrap(err, "could not parse bounds")
	}

	// testing.ContextLogf(disp.ctx, "display bounds: %d, %d", width, height)
	return width, height, nil
}

// stableBounds returns the display bounds, without including possible orientation changes.
func (disp *Display) stableBounds() (int, int, error) {
	cmd := disp.a.Command(disp.ctx, "dumpsys", "display")
	output, err := cmd.Output()
	if err != nil {
		return 0, 0, errors.Wrap(err, "failed to run dumpsys")
	}

	// Looking for:
	// DISPLAY MANAGER (dumpsys display)
	//   mOnlyCode=false
	//   [skipping some properties]
	//   mStableDisplaySize=Point(2400, 1600)
	re := regexp.MustCompile(`(?m)` + // Enable multiline.
		`^\s*DISPLAY MANAGER \(dumpsys display\)$` + // Match DISPLAY MANAGER
		`(?:\s*.*$)*` + // Skip entire lines...
		`\s*mStableDisplaySize=\w*\((\d*),\s*(\d*)\)`) // Gather 'mStableDisplaySize=' bounds.
	groups := re.FindStringSubmatch(string(output))
	if len(groups) != 3 {
		return 0, 0, errors.New("failed to parse dumpsys output")
	}

	width, err := strconv.Atoi(groups[1])
	if err != nil {
		return 0, 0, errors.Wrap(err, "could not parse bounds")
	}
	height, err := strconv.Atoi(groups[2])
	if err != nil {
		return 0, 0, errors.Wrap(err, "could not parse bounds")
	}

	// testing.ContextLogf(disp.ctx, "display stable bounds: %d, %d", width, height)
	return width, height, nil
}
