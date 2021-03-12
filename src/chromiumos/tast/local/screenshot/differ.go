// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package screenshot

import (
	"context"
	"encoding/json"
	"fmt"
	"image"
	"image/draw"
	"image/png"
	"io/ioutil"
	"math"
	"os"
	"path/filepath"
	"time"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/display"
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/chrome/uiauto/nodewith"
	"chromiumos/tast/local/coords"
	"chromiumos/tast/testing"
)

const keysFile = "keys.json"
const screenshotFile = "screenshot.png"

type screenshotState interface {
	Var(name string) (string, bool)
	TestName() string
	Fatal(...interface{})
}

// DiffTestOptions provides all of the ways which you can configure the Diff method.
type DiffTestOptions struct {
	// The time to spend looking for a node
	Timeout time.Duration
}

// Differ is a type for running screendiffs.
type Differ interface {
	Diff(string, *nodewith.Finder) uiauto.Action
	DiffWithOptions(string, *nodewith.Finder, DiffTestOptions) uiauto.Action
	GetFailedDiffs() error
	DieOnFailedDiffs()
}

type differ struct {
	ctx                context.Context
	state              screenshotState
	config             Config
	chrome             *chrome.Chrome
	tconn              *chrome.TestConn
	dir                string
	uiScale            float64
	resetNormalization func() error
}

// NewDiffer creates a differ for a new instance of chrome with default configuration.
func NewDiffer(ctx context.Context, state screenshotState) (Differ, *chrome.Chrome, error) {
	return NewDifferFromConfig(ctx, state, Config{})
}

// NewDifferFromConfig creates a differ for a new instance of chrome with configuration specified in cfg.
func NewDifferFromConfig(ctx context.Context, state screenshotState, cfg Config) (Differ, *chrome.Chrome, error) {
	var d = &differ{ctx: ctx, state: state, config: cfg}
	if err := d.initialize(); err != nil {
		return nil, nil, err
	}
	return d, d.chrome, nil
}

// NewDifferFromChrome creates a differ for an existing chrome instance.
func NewDifferFromChrome(ctx context.Context, state screenshotState, cr *chrome.Chrome) (Differ, error) {
	var d = &differ{ctx: ctx, state: state, chrome: cr, config: Config{}}
	if err := d.initialize(); err != nil {
		return nil, err
	}
	return d, nil
}

func (d *differ) initialize() error {
	if d.chrome == nil {
		cr, err := chrome.New(d.ctx, chrome.Region(d.config.Region))
		if err != nil {
			return err
		}
		d.chrome = cr
	}

	tconn, err := d.chrome.TestAPIConn(d.ctx)
	if err != nil {
		return err
	}
	d.tconn = tconn

	info, displayMode, err := d.normalizeDisplayInfoAndMode()
	if err != nil {
		return err
	}
	uiScale, err := info.GetEffectiveDeviceScaleFactor()
	if err != nil {
		return err
	}
	d.uiScale = uiScale
	params := map[string]string{
		"displayZoomFactor": fmt.Sprintf("%.2f", info.DisplayZoomFactor),
		"deviceScaleFactor": fmt.Sprintf("%.2f", displayMode.DeviceScaleFactor),
		"scale":             fmt.Sprintf("%.2f", uiScale),
		"resolution":        fmt.Sprintf("%dx%d", displayMode.WidthInNativePixels, displayMode.HeightInNativePixels),
	}

	dir, ok := testing.ContextOutDir(d.ctx)
	if !ok {
		return errors.New("couldn't get output dir")
	}
	d.dir = filepath.Join(dir, "screenshots")

	jsonString, err := json.Marshal(params)
	if err != nil {
		return err
	}

	if err := os.MkdirAll(d.dir, 0755); err != nil {
		return err
	}
	if err := ioutil.WriteFile(filepath.Join(d.dir, keysFile), jsonString, 0644); err != nil {
		return err
	}

	return nil
}

func (d *differ) getDisplayInfo() (*display.Info, error) {
	if d.config.DisplayID != "" {
		return display.FindInfo(d.ctx, d.tconn, func(info *display.Info) bool {
			return info.ID == d.config.DisplayID
		})
	}
	infos, err := display.GetInfo(d.ctx, d.tconn)
	if err != nil {
		return nil, err
	}
	// Normalizing DPI on multiple displays with different DeviceScaleFactors won't work.
	if len(infos) != 1 {
		return nil, errors.New("If you want to use screenshot testing with multiple devices, you need to provide config.DisplayID")
	}
	return &infos[0], err
}

func (d *differ) normalizeDisplayInfoAndMode() (*display.Info, *display.DisplayMode, error) {
	info, err := d.getDisplayInfo()
	if err != nil {
		return nil, nil, err
	}

	displayMode, err := info.GetSelectedMode()
	if err != nil {
		return nil, nil, err
	}

	// Pick the resolution that results in a DP/PX scale factor of 1, if available.
	// This should reduce the number of screenshots that have to be approved.
	if !d.config.SkipDpiNormalization {
		for _, zoomFactor := range info.AvailableDisplayZoomFactors {
			if math.Abs(zoomFactor*displayMode.DeviceScaleFactor-1) < 0.01 && zoomFactor != info.DisplayZoomFactor {
				oldZoomFactor := info.DisplayZoomFactor
				info.DisplayZoomFactor = zoomFactor
				if err := display.SetDisplayProperties(d.ctx, d.tconn, info.ID, display.DisplayProperties{DisplayZoomFactor: &zoomFactor}); err != nil {
					return nil, nil, err
				}
				d.resetNormalization = func() error {
					return display.SetDisplayProperties(d.ctx, d.tconn, info.ID, display.DisplayProperties{DisplayZoomFactor: &oldZoomFactor})
				}
			}
		}
	}
	return info, displayMode, nil
}

// Diff takes a screenshot of a ui element and uploads the result to gold.
// Collect all your diff results at the end with GetFailedDiffs() or DieOnFailedDiffs()
func (d *differ) Diff(name string, finder *nodewith.Finder) uiauto.Action {
	return d.DiffWithOptions(name, finder, DiffTestOptions{})
}

// DiffWithOptions takes a screenshot of a ui element and uploads the result to gold.
// Collect all your diff results at the end with GetFailedDiffs() or DieOnFailedDiffs()
func (d *differ) DiffWithOptions(name string, finder *nodewith.Finder, options DiffTestOptions) uiauto.Action {
	return func(_ context.Context) error {
		// Since elements of config are parameters that affect rendering, different configs should always have distinct diffs.
		name = d.state.TestName() + "." + name
		if configString := d.config.ToString(); len(configString) > 0 {
			name += "." + configString
		}
		err := d.capture(name, finder, &options)
		if err != nil {
			return err
		}
		return nil
	}
}

// GetFailedDiffs returns an error containing all of the diffs that failed, if any did, or nil if all passed.
func (d *differ) GetFailedDiffs() error {
	// TODO(crbug/1137989): Once we start using gold, this will become relevant.
	if d.resetNormalization != nil {
		return d.resetNormalization()
	}
	return nil
}

// DieOnFailedDiffs calls s.Fatal() if any diffs failed.
func (d *differ) DieOnFailedDiffs() {
	if err := d.GetFailedDiffs(); err != nil {
		d.state.Fatal("diffs failed: ", err)
	}
}

// DiffPerConfig takes a function that performs a set of screenshot diff tests, and a set of configurations to run it on,
// and runs that screenshot test on each configuration.
func DiffPerConfig(ctx context.Context, state screenshotState, configs []Config, fn func(Differ, *chrome.Chrome)) error {
	var d = &differ{ctx: ctx, state: state}
	for _, config := range configs {
		d.config = config
		// Upon resetting config, chrome needs to be re-initialized.
		d.chrome = nil
		if err := d.initialize(); err != nil {
			return err
		}
		fn(d, d.chrome)
	}
	return d.GetFailedDiffs()
}

// DiffPerConfigOrDie takes a function that performs a set of screenshot diff tests, and a set of configurations to run it on,
// and runs that screenshot test on each configuration, calling fatal if any diff tests failed.
func DiffPerConfigOrDie(ctx context.Context, state screenshotState, configs []Config, fn func(Differ, *chrome.Chrome)) {
	if err := DiffPerConfig(ctx, state, configs, fn); err != nil {
		state.Fatal("diffs failed: ", err)
	}
}

func (d *differ) capture(screenshotName string, finder *nodewith.Finder, options *DiffTestOptions) error {
	if options.Timeout == 0 {
		options.Timeout = time.Second * 2
	}
	location, err := uiauto.New(d.tconn).WithTimeout(options.Timeout).Location(d.ctx, finder)
	if err != nil {
		return err
	}

	dir := filepath.Join(d.dir, screenshotName)
	if _, err := os.Stat(dir); err == nil {
		return errors.Errorf("screenshot has already been taken for %s", screenshotName)
	}
	if err := os.Mkdir(dir, 0755); err != nil {
		return err
	}

	img, err := CaptureChromeImage(d.ctx, d.chrome)
	if err != nil {
		return err
	}

	boundsPx := coords.ConvertBoundsFromDPToPX(*location, d.uiScale)

	// The screenshot returned is of the whole screen. Crop it to only contain the element requested by the user.
	srcOffset := image.Point{X: boundsPx.Left, Y: boundsPx.Top}
	dstSize := image.Rect(0, 0, boundsPx.Width, boundsPx.Height)
	cropped := image.NewRGBA(dstSize)
	draw.Draw(cropped, dstSize, img, srcOffset, draw.Src)

	f, err := os.Create(filepath.Join(dir, screenshotFile))
	if err != nil {
		return err
	}
	png.Encode(f, cropped)
	return nil
}
