// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package screenshot

import (
	"chromiumos/tast/local/chrome/uiauto/nodewith"
)

// Config controls how the screen is rendered during screenshot tests.
type Config struct {
	// The region chrome should be started in. Should correspond to an entry in
	// /usr/share/misc/cros-regions.json.
	Region string

	// The display.Info.ID for the display you want to take screenshots on.
	// Required iff you have multiple displays.
	DisplayID string

	// Whether to skip attempting to scale the DPI so that all images are
	// as similar as possible.
	SkipDpiNormalization bool

	// The width and height requested of a window.
	// You probably don't want to set this yourself - the screen diffing
	// framework will tell you what to set this to.
	WindowWidthDP  int
	WindowHeightDP int

	// Pixels within this distance to a border (top / bottom / sides) of the window
	// will not be considered when determining difference.
	WindowBorderWidthDP int

	// Elements that will be removed from the screenshot. For example, if you have
	// some dynamic content interlaced with static content (eg. file modification
	// times in the files app).
	RemoveElements []*nodewith.Finder

	// Whether to output the UI tree for each screenshot.
	OutputUITrees bool

	// A suffix to add to the name. Might be a version number (eg. "V2"), or a
	// human-readable label.
	NameSuffix string

	// If DryRun is true, instead of running the goldctl imgtest, logs it.
	DryRun bool
}

// Suffix should return a string representation of the suffix for the test
// It will contain only non-default options.
// eg. Region: "de" would be ".de"
func (c *Config) Suffix() string {
	result := ""
	if c.NameSuffix != "" {
		result += "." + c.NameSuffix
	}
	if c.Region != "" {
		result += "." + c.Region
	}
	return result
}

// WithBase returns configs, but with any unfilled fields being filled with the value in base.
func WithBase(base Config, configs []Config) []Config {
	var results []Config
	for _, c := range configs {
		if c.Region == "" {
			c.Region = base.Region
		}
		if c.DisplayID == "" {
			c.DisplayID = base.DisplayID
		}
		if !c.SkipDpiNormalization {
			c.SkipDpiNormalization = base.SkipDpiNormalization
		}
		if !c.OutputUITrees {
			c.OutputUITrees = base.OutputUITrees
		}
		results = append(results, c)
	}
	return results
}

// ThoroughConfigs is a set of configurations that should test most use cases.
func ThoroughConfigs() []Config {
	return []Config{
		// TODO(crbug.com/1173812): Add pseudolocales once they're supported on all release images.
		// Once they're added, switch screen_diff.go to use ThoroughConfigs instead of an empty list.
		{Region: "us"},
	}
}
