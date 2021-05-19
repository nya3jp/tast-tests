// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package screenshot

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

	// Whether to output the UI tree for each screenshot.
	OutputUITrees bool
}

// Suffix should return a string representation of the suffix for the test
// It will contain only non-default options.
// eg. Region: "de" would be ".de"
func (c *Config) Suffix() string {
	result := ""
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
