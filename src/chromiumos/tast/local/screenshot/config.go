// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package screenshot

// Config controls how the screen is rendered during screenshot tests.
type Config struct {
	Region string
	// The display.Info.ID for the display you want to take screenshots on.
	// Required iff you have multiple displays.
	DisplayID string
	// Whether to skip attempting to scale the DPI so that all images are
	// as similar as possible.
	SkipDpiNormalization bool
}

// ToString should return a string representation of the configuration.
// It will contain only non-default options.
func (c *Config) ToString() string {
	result := ""
	if c.Region != "" {
		result += c.Region
	}
	return result
}

// ThoroughConfigs is a set of configurations that should test most use cases.
func ThoroughConfigs() []Config {
	return []Config{
		{Region: "us"},
		{Region: "il"}, // Israel for Hebrew (RTL)
		{Region: "de"}, // German for long strings
		{Region: "jp"}, // Japanese because you should always test at least one of chinese/korean/japanese.
	}
}
