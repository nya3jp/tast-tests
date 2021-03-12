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
	"os"
	"path/filepath"
	"time"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/ash"
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
	Log(...interface{})
	Fatal(...interface{})
}

// DiffTestOptions provides all of the ways which you can configure the Diff method.
type DiffTestOptions struct {
	// The node that we're attempting to take a screenshot of
	Params *nodewith.Finder

	// Whether to hide any notifications which might be overlaid on top of the element.
	HideNotifications bool
}

// Differ is a type for running screendiffs.
type Differ interface {
	Diff(string, *nodewith.Finder) uiauto.Action
	DiffWithOptions(string, DiffTestOptions) uiauto.Action
	GetFailedDiffs() error
	DieOnFailedDiffs()
}

type differ struct {
	ctx    context.Context
	state  screenshotState
	config Config
	chrome *chrome.Chrome
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
	d.config.FillDefaults()
	if d.chrome == nil {
		cr, err := chrome.New(d.ctx, chrome.Region(d.config.Region))
		if err != nil {
			return err
		}
		d.chrome = cr
	}
	return nil
}

// Diff takes a screenshot of a ui element and uploads the result to gold.
// Collect all your diff results at the end with GetFailedDiffs() or DieOnFailedDiffs()
func (d *differ) Diff(name string, params *nodewith.Finder) uiauto.Action {
	return d.DiffWithOptions(name, DiffTestOptions{Params: params})
}

// DiffWithOptions takes a screenshot of a ui element and uploads the result to gold.
// Collect all your diff results at the end with GetFailedDiffs() or DieOnFailedDiffs()
func (d *differ) DiffWithOptions(name string, options DiffTestOptions) uiauto.Action {
	return func(_ context.Context) error {
		if err := d.initialize(); err != nil {
			return err
		}
		// Since elements of config are parameters that affect rendering, different configs should always have distinct diffs.
		name = d.state.TestName() + "." + name
		if configString := d.config.ToString(); len(configString) > 0 {
			name += "." + configString
		}
		_, err := d.capture(name, options)
		if err != nil {
			return err
		}
		return nil
	}
}

// GetFailedDiffs returns an error containing all of the diffs that failed, if any did, or nil if all passed.
func (d *differ) GetFailedDiffs() error {
	// TODO(crbug/1137989): Once we start using gold, this will become relevant.
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

func (d *differ) capture(screenshotName string, options DiffTestOptions) (string, error) {
	ctx := d.ctx
	tconn, err := d.chrome.TestAPIConn(ctx)
	if err != nil {
		return "", err
	}

	node, err := uiauto.New(tconn).WithTimeout(time.Second*2).Info(d.ctx, options.Params)
	if err != nil {
		return "", err
	}

	info, err := display.FindInfo(ctx, tconn, func(info *display.Info) bool {
		return info.Bounds.Contains(node.Location)
	})
	if err != nil {
		return "", err
	}

	displayMode, err := info.GetSelectedMode()
	if err != nil {
		return "", err
	}

	scale, err := info.GetEffectiveDeviceScaleFactor()
	if err != nil {
		return "", err
	}
	boundsPx := coords.ConvertBoundsFromDPToPX(node.Location, scale)

	params := map[string]string{
		"resolution": fmt.Sprintf("%dx%d", displayMode.WidthInNativePixels, displayMode.HeightInNativePixels),
		"scale":      fmt.Sprintf("%.2f", scale),
	}

	dir, ok := testing.ContextOutDir(ctx)
	if !ok {
		return "", errors.New("couldn't get output dir")
	}

	dir = filepath.Join(dir, "screenshots", screenshotName)
	if _, err := os.Stat(dir); err == nil {
		return "", errors.Errorf("screenshot has already been taken for screenshot %s and params %+v", screenshotName, params)
	}
	if err := os.MkdirAll(dir, 0755); err != nil {
		return "", err
	}

	jsonString, err := json.Marshal(params)
	if err != nil {
		return "", err
	}
	if err := ioutil.WriteFile(filepath.Join(dir, keysFile), jsonString, 0644); err != nil {
		return "", err
	}

	if options.HideNotifications {
		ash.HideVisibleNotificationsAndWait(ctx, tconn)
	}

	img, err := CaptureChromeImage(d.ctx, d.chrome)
	if err != nil {
		return "", err
	}

	// The screenshot returned is of the whole screen. Crop it to only contain the element requested by the user.
	srcOffset := image.Point{X: boundsPx.Left, Y: boundsPx.Top}
	dstSize := image.Rect(0, 0, boundsPx.Width, boundsPx.Height)
	cropped := image.NewRGBA(dstSize)
	draw.Draw(cropped, dstSize, img, srcOffset, draw.Src)

	f, err := os.Create(filepath.Join(dir, screenshotFile))
	if err != nil {
		return "", err
	}
	png.Encode(f, cropped)
	return dir, nil
}
