// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package screenshot

import (
	"context"
	"encoding/json"
	"fmt"
	"image"
	"image/color"
	"image/draw"
	"image/png"
	"io/ioutil"
	"math"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"chromiumos/tast/common/testexec"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/display"
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/chrome/uiauto/nodewith"
	"chromiumos/tast/local/chrome/uiauto/role"
	"chromiumos/tast/local/coords"
	"chromiumos/tast/lsbrelease"
	"chromiumos/tast/testing"
)

// GoldServiceAccountKeyVar contains the name of the variable storing the service account key.
const GoldServiceAccountKeyVar = "goldctl.GoldServiceAccountKey"

const goldServiceAccountKeyFile = "/tmp/gold_service_account_key.json"

// TODO(crbug.com/skia/10808): Change this once we have a production instance.
const goldInstance = "cros-tast-dev"

const goldctlWorkDir = "/tmp/goldctl"
const keysFile = "keys.json"
const screenshotFile = "cropped.png"

const fontConfigDir = "/etc/fonts/conf.d"

var subPixelAAFiles = map[string]string{
	"10-no-sub-pixel.conf":   "none",
	"10-sub-pixel-bgr.conf":  "BGR",
	"10-sub-pixel-rgb.conf":  "RGB",
	"10-sub-pixel-vbgr.conf": "VGBR",
	"10-sub-pixel-vrgb.conf": "VRGB"}

type testMode string

const (
	local      testMode = "local"
	cq         testMode = "cq"
	postsubmit testMode = "postsubmit"
)

type screenshotState interface {
	Var(name string) (string, bool)
	TestName() string
	Fatal(...interface{})
	Logf(string, ...interface{})
}

// DiffTestOptions provides all of the ways which you can configure the Diff method.
type DiffTestOptions struct {
	// The time to spend looking for a node
	Timeout time.Duration

	// The minimum difference required to treat two pixels as different.
	// Specifically, this is dr + dg + db (the sum of the difference in	each channel).
	PixelDeltaThreshold int

	// Pixels within this distance to a border (e.g. the top, the bottom, the side)
	// will not be considered when determining difference.
	IgnoredBorderThickness int

	// Elements that will be removed from the screenshot. For example, if you have
	// some dynamic content interlaced with static content (eg. file modification
	// times in the files app).
	RemoveElements []*nodewith.Finder
}

// Differ is a type for running screendiffs.
type Differ interface {
	Chrome() *chrome.Chrome
	Tconn() *chrome.TestConn
	Diff(string, *nodewith.Finder) uiauto.Action
	DiffWithOptions(string, *nodewith.Finder, DiffTestOptions) uiauto.Action
	GetFailedDiffs() error
	DieOnFailedDiffs()
}

type differ struct {
	ctx         context.Context
	state       screenshotState
	config      Config
	chrome      *chrome.Chrome
	tconn       *chrome.TestConn
	dir         string
	uiScale     float64
	reset       func() error
	goldArgs    []string
	failedTests []string
	testMode    testMode
	triage      string
}

// NewDiffer creates a differ for a new instance of chrome with default configuration.
func NewDiffer(ctx context.Context, state screenshotState) (Differ, error) {
	return NewDifferFromConfig(ctx, state, Config{})
}

// NewDifferFromConfig creates a differ for a new instance of chrome with configuration specified in cfg.
func NewDifferFromConfig(ctx context.Context, state screenshotState, cfg Config) (Differ, error) {
	var d = &differ{ctx: ctx, state: state, config: cfg}
	if err := d.initialize(); err != nil {
		return nil, errors.Wrap(err, "failed to initialize screen differ")
	}
	return d, nil
}

// NewDifferFromChrome creates a differ for an existing chrome instance.
func NewDifferFromChrome(ctx context.Context, state screenshotState, cr *chrome.Chrome) (Differ, error) {
	var d = &differ{ctx: ctx, state: state, chrome: cr, config: Config{}}
	if err := d.initialize(); err != nil {
		return nil, errors.Wrap(err, "failed to initialize screen differ")
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

	release, err := lsbrelease.Load()
	if err != nil {
		return err
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
		"display_zoom_factor":      fmt.Sprintf("%.2f", info.DisplayZoomFactor),
		"device_scale_factor":      fmt.Sprintf("%.2f", displayMode.DeviceScaleFactor),
		"scale":                    fmt.Sprintf("%.2f", uiScale),
		"resolution":               fmt.Sprintf("%dx%d", displayMode.WidthInNativePixels, displayMode.HeightInNativePixels),
		"board":                    release[lsbrelease.Board],
		"sub_pixel_antialiasing":   currentSubPixelAntialiasingMethod(),
		"image_matching_algorithm": "fuzzy",
		// Fuzzy matcher is a bit weird. Instead of "no more than <max different pixels> with difference of more than <delta>",
		// it means "no more than <max different pixels> differing, and no individual pixel has more than <delta> difference."
		// If we want to accept an image with all pixels off by one, this needs to be at least the number of pixels in the image.
		"fuzzy_max_different_pixels": "999999999",
	}

	dir, ok := testing.ContextOutDir(d.ctx)
	if !ok {
		return errors.New("couldn't get output dir")
	}
	// Different configs may have different sets of keys.
	d.dir = filepath.Join(dir, "screenshots"+d.config.Suffix())

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

	baseArgs := []string{
		"--corpus", strings.Split(d.state.TestName(), ".")[0],
		"--passfail",
	}

	if strings.HasPrefix(release[lsbrelease.BuildType], "Continuous Builder") {
		d.testMode = cq
		builderMatch := regexp.MustCompile("-([0-9]+)$").FindStringSubmatch(release[lsbrelease.BuilderPath])
		if builderMatch == nil {
			return errors.Errorf("for a CQ build, the buildbucket ID should be filled in builder path (got %+v)", release[lsbrelease.BuilderPath])
		}
		d.goldArgs = append(baseArgs, []string{
			"--cis", "buildbucket",
			"--crs", "lookup",
			"--changelist", "lookup",
			"--patchset_id", "lookup",
			"--jobid", builderMatch[1]}...)
		// TODO(crbug.com/skia/10808): once gold supports filtering by job id in the URL, set that.
		d.triage = "Please look at the comment by the gold bot on the CL for a link to approve."

		// Note: This will falsely pick up local builds that have been flashed with an official build.
		// In the future, we may attempt to come up with a way to distinguish between these two.
	} else if release[lsbrelease.BuildType] == "Official Build" {
		build := release[lsbrelease.BuilderPath]
		d.testMode = postsubmit
		d.goldArgs = append(baseArgs, []string{
			"--commit_id", strings.Split(build, "/")[1],
			"--commit_metadata", fmt.Sprintf("gs://chromeos-image-archive/%s/manifest.xml", build)}...)
	} else {
		d.testMode = local
		// TODO(crbug.com/skia/11815): once gold supports a local dev mode, replace the git hash field with that.
		d.goldArgs = append(baseArgs, []string{
			"--git_hash", "f1d65adb1d7cd922f4677d0f9406a4083f5fdcbc"}...)
	}

	if err := d.authenticateGold(); err != nil {
		return errors.Wrap(err, "failed to authenticate gold")
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
		return nil, errors.New("if you want to use screenshot testing with multiple devices, you need to provide config.DisplayID")
	}
	return &infos[0], nil
}

func (d *differ) normalizeDisplayInfoAndMode() (*display.Info, *display.DisplayMode, error) {
	info, err := d.getDisplayInfo()
	if err != nil {
		return nil, nil, errors.Wrap(err, "failed to get the display info")
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
					return nil, nil, errors.Wrap(err, "failed to normalize DPI")
				}
				d.reset = func() error {
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
		fullName := d.state.TestName() + "." + name + d.config.Suffix()
		extraArgs, err := d.capture(name, finder, &options)
		if err != nil {
			return errors.Wrap(err, "failed to take screenshot")
		}
		if err := d.runGoldCommand("imgtest", append(append([]string{"add",
			"--instance", goldInstance,
			"--keys-file", filepath.Join(d.dir, keysFile),
			"--test-name", fullName,
			"--png-file", filepath.Join(d.dir, name, screenshotFile),
		}, d.goldArgs...), extraArgs...)...); err != nil {
			d.failedTests = append(d.failedTests, name+d.config.Suffix())
			// In case you don't have access to a filter by commit ID / release, output the logs directly.
			d.state.Logf("Failed test %s: %v", name, err)
		}

		return nil
	}
}

// GetFailedDiffs returns an error containing all of the diffs that failed, if any did, or nil if all passed.
func (d *differ) GetFailedDiffs() error {
	if d.reset != nil {
		if err := d.reset(); err != nil {
			return err
		}
	}

	if len(d.failedTests) > 0 {
		return errors.Errorf("failed screenshot tests: %s %s",
			strings.Join(d.failedTests, " "), d.triage)
	}
	return nil
}

// DieOnFailedDiffs calls s.Fatal() if any diffs failed.
func (d *differ) DieOnFailedDiffs() {
	if err := d.GetFailedDiffs(); err != nil {
		d.state.Fatal("screen diffs failed: ", err)
	}
}

// Chrome returns the chrome instance used in the screenshot test.
func (d *differ) Chrome() *chrome.Chrome {
	return d.chrome
}

// Tconn returns the tconn associated with this chrome.
func (d *differ) Tconn() *chrome.TestConn {
	return d.tconn
}

// DiffPerConfig takes a function that performs a set of screenshot diff tests, and a set of configurations to run it on,
// and runs that screenshot test on each configuration.
func DiffPerConfig(ctx context.Context, state screenshotState, configs []Config, fn func(Differ)) error {
	var d = &differ{ctx: ctx, state: state}
	for _, config := range configs {
		d.config = config
		// Upon resetting config, chrome needs to be re-initialized.
		d.chrome = nil
		if err := d.initialize(); err != nil {
			return err
		}
		fn(d)
	}
	return d.GetFailedDiffs()
}

func (d *differ) capture(screenshotName string, finder *nodewith.Finder, options *DiffTestOptions) ([]string, error) {
	var testArgs []string
	if options.Timeout == 0 {
		options.Timeout = time.Second * 2
	}
	if options.PixelDeltaThreshold < 3 {
		// Allow off-by-one in each channel.
		// Experimental results seem to show that several boards are off by a single color in some channels,
		// probably due to floating-point arithmetic. Since it's basically invisible to the end-user, ignore it.
		options.PixelDeltaThreshold = 3
	}

	ui := uiauto.New(d.tconn).WithTimeout(options.Timeout)
	location, err := ui.Location(d.ctx, finder)
	if err != nil {
		return testArgs, errors.Wrap(err, "failed to find node to take screenshot of")
	}

	info, err := ui.Info(d.ctx, finder)
	if err != nil {
		return testArgs, errors.Wrap(err, "unable to get info for node")
	}
	if options.IgnoredBorderThickness == 0 && info.Role == role.Window {
		// A window's corners are rounded, and unlike other elements, the background is inconsistent (since it's the wallpaper).
		options.IgnoredBorderThickness = 1
	}

	dir := filepath.Join(d.dir, screenshotName)
	if _, err := os.Stat(dir); err == nil {
		return testArgs, errors.Errorf("screenshot has already been taken for %s, please give this screenshot a unique name", screenshotName)
	}
	if err := os.Mkdir(dir, 0755); err != nil {
		return testArgs, err
	}

	if d.config.OutputUITrees {
		uiauto.LogRootDebugInfo(d.ctx, d.tconn, filepath.Join(dir, "ui_tree.txt"))
	}

	img, err := CaptureChromeImage(d.ctx, d.chrome)
	if err != nil {
		return testArgs, errors.Wrap(err, "failed to capture the chrome image")
	}

	// This screenshot isn't used anywhere, but is useful for context to devs.
	f, err := os.Create(filepath.Join(dir, "screenshot.png"))
	if err != nil {
		return testArgs, err
	}
	png.Encode(f, img)

	boundsPx := coords.ConvertBoundsFromDPToPX(*location, d.uiScale)

	// The screenshot returned is of the whole screen. Crop it to only contain the element requested by the user.
	srcOffset := image.Point{X: boundsPx.Left, Y: boundsPx.Top}
	dstSize := image.Rect(0, 0, boundsPx.Width, boundsPx.Height)
	testArgs = append(testArgs,
		"--add-test-optional-key", fmt.Sprintf("cropped_resolution:%dx%d", boundsPx.Height, boundsPx.Width),
		"--add-test-optional-key", fmt.Sprintf("top_left_pixel:left_%d_top_%d", boundsPx.Left, boundsPx.Top),
		"--add-test-optional-key", fmt.Sprintf("fuzzy_pixel_delta_threshold:%d", options.PixelDeltaThreshold),
		"--add-test-optional-key", fmt.Sprintf("fuzzy_ignored_border_thickness:%d", options.IgnoredBorderThickness))
	cropped := image.NewRGBA(dstSize)
	draw.Draw(cropped, dstSize, img, srcOffset, draw.Src)

	for _, subelement := range options.RemoveElements {
		location, err := ui.Location(d.ctx, subelement.Ancestor(finder))
		if err != nil {
			return testArgs, errors.Wrap(err, "unable to find element to remove from screenshot")
		}
		removedRect := coords.ConvertBoundsFromDPToPX(*location, d.uiScale)
		removedRect.Left -= boundsPx.Left
		removedRect.Top -= boundsPx.Top
		draw.Draw(cropped,
			image.Rect(removedRect.Left, removedRect.Top, removedRect.Left+removedRect.Width, removedRect.Top+removedRect.Height),
			&image.Uniform{color.Transparent}, image.ZP, draw.Src)
	}

	f, err = os.Create(filepath.Join(dir, screenshotFile))
	if err != nil {
		return testArgs, err
	}
	png.Encode(f, cropped)
	return testArgs, nil
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
		// Ignore the no newlines lint rule, because it makes it much easier to see what went wrong this way.
		err = errors.Errorf("while running \"goldctl %s\"\n%s", strings.Join(args, " "), out) // NOLINT
	}
	return err
}

func currentSubPixelAntialiasingMethod() string {
	for fname, method := range subPixelAAFiles {
		if _, err := os.Stat(filepath.Join(fontConfigDir, fname)); !os.IsNotExist(err) {
			return method
		}
	}
	return "unknown"
}
