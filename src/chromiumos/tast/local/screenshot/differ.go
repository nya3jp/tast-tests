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
	"strings"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/ash"
	"chromiumos/tast/local/chrome/display"
	"chromiumos/tast/local/chrome/ui"
	"chromiumos/tast/local/coords"
	"chromiumos/tast/local/testexec"
	"chromiumos/tast/testing"
)

// ScreenDiffFixtureName is the name of the fixture returned by ScreenDiffFixture()
const ScreenDiffFixtureName = "screenDiff"

// GoldServiceAccountKeyVar contains the name of the variable storing the service account key.
const GoldServiceAccountKeyVar = "goldServiceAccountKey"

const goldServiceAccountKeyFile = "/tmp/gold_service_account_key.json"

// TODO(crbug.com/skia/10808): Change this once we have a production instance.
const goldInstance = "cros-tast-dev"

// TODO(crbug.com/skia/10808): Remove this once we have a production instance that relies on a unique identifier that doesn't use commit IDs (see the getChromeosVersion function).
const commitID = "631c50247514d41ac2266f5c993705606cff3d0c"

const goldctlWorkDir = "/tmp/goldctl"
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
	Node *ui.Node

	// Whether to hide any notifications which might be overlaid on top of the element.
	HideNotifications bool
}

// Differ is a type for running screendiffs.
type Differ interface {
	Diff(string, ui.FindParams) error
	DiffNode(string, *ui.Node) error
	DiffWithOptions(string, DiffTestOptions) error
	GetFailedDiffs() error
	DieOnFailedDiffs()
}

type differ struct {
	ctx    context.Context
	state  screenshotState
	config Config
	chrome *chrome.Chrome
	errors []string
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
	return d.authenticateGold()
}

// Diff takes a screenshot of a ui element and uploads the result to gold.
// Collect all your diff results at the end with GetFailedDiffs() or DieOnFailedDiffs()
func (d *differ) Diff(name string, params ui.FindParams) error {
	tconn, err := d.chrome.TestAPIConn(d.ctx)
	if err != nil {
		return err
	}
	node, err := ui.FindSingleton(d.ctx, tconn, params)
	if err != nil {
		return err
	}
	defer node.Release(d.ctx)
	return d.DiffNode(name, node)
}

// Diff takes a screenshot of a ui element and uploads the result to gold.
// Collect all your diff results at the end with GetFailedDiffs() or DieOnFailedDiffs()
func (d *differ) DiffNode(name string, node *ui.Node) error {
	return d.DiffWithOptions(name, DiffTestOptions{Node: node})
}

// DiffWithOptions takes a screenshot of a ui element and uploads the result to gold.
// Collect all your diff results at the end with GetFailedDiffs() or DieOnFailedDiffs()
func (d *differ) DiffWithOptions(name string, options DiffTestOptions) error {
	if err := d.initialize(); err != nil {
		return err
	}
	// Since elements of config are parameters that affect rendering, different configs should always have distinct diffs.
	name = d.state.TestName() + "." + name
	if configString := d.config.ToString(); len(configString) > 0 {
		name += "." + configString
	}
	dir, err := d.capture(name, options)
	if err != nil {
		return err
	}
	if err := d.runGoldCommand("imgtest", "init", "--instance", goldInstance, "--keys-file", filepath.Join(dir, keysFile), "--commit", commitID, "--passfail"); err != nil {
		return err
	}
	if err := d.runGoldCommand("imgtest", "add", "--test-name", name, "--png-file", filepath.Join(dir, screenshotFile)); err != nil {
		d.errors = append(d.errors, err.Error())
		// Not strictly required, but for longer tests, it's probably useful to let users see these diffs as early as possible.
		d.state.Log(err)
	}
	return nil
}

// GetFailedDiffs returns an error containing all of the diffs that failed, if any did, or nil if all passed.
func (d *differ) GetFailedDiffs() error {
	if len(d.errors) > 0 {
		// Each of these errors is the stdout from a failing diff.
		// Each failed diff will look something like this:
		// while running "goldctl imgtest --test-name blah --png-file blah.png"
		// Given image with hash <hash> for test blah
		// Expectation for test: <hash> (positive)
		// Untriaged or negative image: https://cros-tast-gold.skia.org/detail?test=blah&digest=<hash>
		// Test: blah FAIL
		err := errors.New("\n\n" + strings.Join(d.errors, "\n\n"))
		return err
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

func (d *differ) authenticateGold() error {
	// If this file exists, then we've already authenticated, so there's no need to do it again.
	if file, _ := os.Stat(filepath.Join(goldctlWorkDir, "auth_opt.json")); file != nil {
		return nil
	}
	key, ok := d.state.Var(GoldServiceAccountKeyVar)
	if !ok {
		return errors.New("couldn't get the gold service account key. Please ensure you have access to tast-tests-private")
	}
	if err := ioutil.WriteFile(goldServiceAccountKeyFile, []byte(key), 0644); err != nil {
		return err
	}

	return d.runGoldCommand("auth", "--service-account", goldServiceAccountKeyFile)
}

func (d *differ) runGoldCommand(subcommand string, args ...string) error {
	args = append([](string){subcommand, "--work-dir", goldctlWorkDir}, args...)
	cmd := testexec.CommandContext(d.ctx, "goldctl", args...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		// The suggested method of writing to a file to avoid newlines is irrelevant in this case,
		// since the stdout/err needs to be part of the error, which has newlines anyway.
		err = errors.Errorf("while running \"goldctl %s\"\n%s", strings.Join(args, " "), out) // NOLINT
	}
	return err
}

func (d *differ) capture(screenshotName string, options DiffTestOptions) (string, error) {
	ctx := d.ctx
	tconn, err := d.chrome.TestAPIConn(ctx)
	if err != nil {
		return "", err
	}

	boundsDp := options.Node.Location

	info, err := display.FindInfo(ctx, tconn, func(info *display.Info) bool {
		return info.Bounds.Contains(boundsDp)
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
	boundsPx := coords.ConvertBoundsFromDPToPX(boundsDp, scale)

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
