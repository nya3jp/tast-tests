// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package screenshot

import (
	"time"

	"chromiumos/tast/local/chrome/uiauto/nodewith"
)

// Config controls how the screen is rendered during screenshot tests.
type Config struct {
	// The set of default options to use for diff tests.
	DefaultOptions Options

	// The region chrome should be started in. Should correspond to an entry in
	// /usr/share/misc/cros-regions.json.
	Region string

	// The display.Info.ID for the display you want to take screenshots on.
	// Required iff you have multiple displays.
	DisplayID string

	// Whether to skip attempting to scale the DPI so that all images are
	// as similar as possible.
	SkipDpiNormalization bool

	// If DryRun is true, instead of running the goldctl imgtest, logs it.
	DryRun bool

	// Whether to output the UI tree for each screenshot.
	OutputUITrees bool

	// A suffix to add to the name. Might be a version number (eg. "V2"), or a
	// human-readable label.
	NameSuffix string
}

// Options provides all of the ways which you can configure the Diff method.
type Options struct {
	// The time to spend looking for a node
	Timeout time.Duration

	// The minimum difference required to treat two pixels as different.
	// Specifically, this is dr + dg + db (the sum of the difference in	each channel).
	PixelDeltaThreshold int

	// The width and height requested of a window, in DP.
	// You probably don't want to set this yourself - the screen diffing
	// framework will tell you what to set this to.
	WindowWidthDP  int
	WindowHeightDP int
	// Whether to skip window resizing and moving respectively.
	// If SkipWindowResize is true, WindowHeightDP and WindowWidthDP won't be required.
	SkipWindowResize bool
	SkipWindowMove   bool

	// Density independent pixels within this distance to a border (top / bottom / sides)
	// of the window will not be considered when determining difference.
	WindowBorderWidthDP int

	// Elements that will be removed from the screenshot. For example, if you have
	// some dynamic content interlaced with static content (eg. file modification
	// times in the files app).
	RemoveElements []*nodewith.Finder

	// The number of times and interval between retries when taking screenshots.
	// We retry for two reasons:
	// 1) Check that any animations have completed (eg. attempting to move the
	//    window can be slow, and the ui tree updates before the window has
	//    finished moving).
	// 2) Try and pick up on any ongoing animations during execution rather
	//    than in gold.
	Retries       int
	RetryInterval time.Duration
}

// FillDefaults fills any unfilled fields in o with values from d.
func (o *Options) FillDefaults(d Options) {
	if o.Timeout == 0 {
		o.Timeout = d.Timeout
	}
	if o.PixelDeltaThreshold == 0 {
		o.PixelDeltaThreshold = d.PixelDeltaThreshold
	}
	if o.WindowWidthDP == 0 {
		o.WindowWidthDP = d.WindowWidthDP
	}
	if o.PixelDeltaThreshold == 0 {
		o.WindowHeightDP = d.WindowHeightDP
	}
	if o.WindowBorderWidthDP == 0 {
		o.WindowBorderWidthDP = d.WindowBorderWidthDP
	}
	o.RemoveElements = append(o.RemoveElements, d.RemoveElements...)
	if o.Retries == 0 {
		o.Retries = d.Retries
	}
	if o.RetryInterval == 0 {
		o.RetryInterval = d.RetryInterval
	}
	if !o.SkipWindowResize {
		o.SkipWindowResize = d.SkipWindowResize
	}
	if !o.SkipWindowMove {
		o.SkipWindowMove = d.SkipWindowMove
	}
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
		c.DefaultOptions.FillDefaults(base.DefaultOptions)
		if !c.DryRun {
			c.DryRun = base.DryRun
		}
		if !c.OutputUITrees {
			c.OutputUITrees = base.OutputUITrees
		}
		if c.NameSuffix == "" {
			c.NameSuffix = base.NameSuffix
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

// Option is a modifier to apply to Options.
type Option = func(*Options)

// Timeout controls the screenshot test option Timeout.
func Timeout(timeout time.Duration) Option {
	return func(o *Options) { o.Timeout = timeout }
}

// PixelDeltaThreshold controls the screenshot test option PixelDeltaThreshold.
func PixelDeltaThreshold(pixelDeltaThreshold int) Option {
	return func(o *Options) { o.PixelDeltaThreshold = pixelDeltaThreshold }
}

// WindowWidthDP controls the screenshot test option WindowWidthDP.
func WindowWidthDP(windowWidthDP int) Option {
	return func(o *Options) { o.WindowWidthDP = windowWidthDP }
}

// WindowHeightDP controls the screenshot test option WindowHeightDP.
func WindowHeightDP(windowHeightDP int) Option {
	return func(o *Options) { o.WindowHeightDP = windowHeightDP }
}

// SkipWindowResize controls the screenshot test option SkipWindowResize.
func SkipWindowResize(skipWindowResize bool) Option {
	return func(o *Options) { o.SkipWindowResize = skipWindowResize }
}

// SkipWindowMove controls the screenshot test option SkipWindowMove.
func SkipWindowMove(skipWindowMove bool) Option {
	return func(o *Options) { o.SkipWindowMove = skipWindowMove }
}

// WindowBorderWidthDP controls the screenshot test option WindowBorderWidthDP.
func WindowBorderWidthDP(windowBorderWidthDP int) Option {
	return func(o *Options) { o.WindowBorderWidthDP = windowBorderWidthDP }
}

// RemoveElements controls the screenshot test option RemoveElements.
func RemoveElements(removeElements []*nodewith.Finder) Option {
	return func(o *Options) { o.RemoveElements = removeElements }
}

// Retries controls the screenshot test option Retries.
func Retries(retries int) Option {
	return func(o *Options) { o.Retries = retries }
}

// RetryInterval controls the screenshot test option RetryInterval.
func RetryInterval(retryInterval time.Duration) Option {
	return func(o *Options) { o.RetryInterval = retryInterval }
}
