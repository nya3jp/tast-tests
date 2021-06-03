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
	"math/rand"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"chromiumos/tast/common/testexec"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/action"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/ash"
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
const wholeScreenFile = "screenshot.png"
const oldScreenshotFile = "old_cropped.png"
const oldWholeScreenFile = "old_screenshot.png"

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
}

// Differ is a type for running screendiffs.
type Differ interface {
	Chrome() *chrome.Chrome
	Tconn() *chrome.TestConn
	Diff(context.Context, string, *nodewith.Finder) uiauto.Action
	DiffWindow(context.Context, string) uiauto.Action
	DiffWithOptions(context.Context, string, *nodewith.Finder, Options) uiauto.Action
	GetFailedDiffs() error
	DieOnFailedDiffs()
}

type differ struct {
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
	executionID string
	triage      string
}

// NewDiffer creates a differ for a new instance of chrome with configuration specified in cfg.
func NewDiffer(ctx context.Context, state screenshotState, cfg Config) (Differ, error) {
	var d = &differ{state: state, config: cfg}
	if err := d.initialize(ctx); err != nil {
		return nil, errors.Wrap(err, "failed to initialize screen differ")
	}
	return d, nil
}

// NewDifferFromChrome creates a differ for an existing chrome instance.
func NewDifferFromChrome(ctx context.Context, state screenshotState, cr *chrome.Chrome, cfg Config) (Differ, error) {
	var d = &differ{state: state, chrome: cr, config: cfg}
	if err := d.initialize(ctx); err != nil {
		return nil, errors.Wrap(err, "failed to initialize screen differ")
	}
	return d, nil
}

func (d *differ) initialize(ctx context.Context) error {
	if d.chrome == nil {
		cr, err := chrome.New(ctx, chrome.Region(d.config.Region))
		if err != nil {
			return err
		}
		d.chrome = cr
	}

	release, err := lsbrelease.Load()
	if err != nil {
		return err
	}

	tconn, err := d.chrome.TestAPIConn(ctx)
	if err != nil {
		return err
	}
	d.tconn = tconn

	info, displayMode, err := d.normalizeDisplayInfoAndMode(ctx)
	if err != nil {
		return err
	}
	uiScale, err := info.GetEffectiveDeviceScaleFactor()
	if err != nil {
		return err
	}
	d.uiScale = uiScale

	tabletMode, err := ash.TabletModeEnabled(ctx, tconn)
	if err != nil {
		return err
	}

	region := d.config.Region
	if region == "" {
		region = "us"
	}
	nameSuffix := d.config.NameSuffix
	if nameSuffix == "" {
		nameSuffix = "none"
	}

	cpuInfo, err := d.getCPUInfo(ctx)
	if err != nil {
		return err
	}

	modelName, ok := cpuInfo["Model name"]
	if !ok {
		modelName = "unknown"
	}

	if d.executionID == "" {
		d.executionID = fmt.Sprintf("%X", rand.Int31())
	}

	params := map[string]string{
		"board":               release[lsbrelease.Board],
		"device_scale_factor": fmt.Sprintf("%.2f", displayMode.DeviceScaleFactor),
		"display_zoom_factor": fmt.Sprintf("%.2f", info.DisplayZoomFactor),
		// Fuzzy matcher is a bit weird. Instead of "no more than <max different pixels> with difference of more than <delta>",
		// it means "no more than <max different pixels> differing, and no individual pixel has more than <delta> difference."
		// If we want to accept an image with all pixels off by one, this needs to be at least the number of pixels in the image.
		"fuzzy_max_different_pixels": "999999999",
		"image_matching_algorithm":   "fuzzy",
		"cpu_arch":                   cpuInfo["Architecture"],
		"cpu_model":                  modelName,
		"cpu_vendor":                 cpuInfo["Vendor ID"],
		"execution_id":               d.executionID,
		"name_suffix":                nameSuffix,
		"region":                     region,
		"resolution":                 fmt.Sprintf("%dx%d", displayMode.WidthInNativePixels, displayMode.HeightInNativePixels),
		"sub_pixel_antialiasing":     currentSubPixelAntialiasingMethod(),
		"scale":                      fmt.Sprintf("%.2f", uiScale),
		"tablet_mode":                fmt.Sprintf("%t", tabletMode),
		"test_group":                 d.state.TestName(),
		"version":                    release[lsbrelease.Version],
	}

	dir, ok := testing.ContextOutDir(ctx)
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

	corpus := strings.Split(d.state.TestName(), ".")[0]
	baseArgs := []string{
		"--corpus", corpus,
		"--passfail",
	}

	d.triage = fmt.Sprintf("https://%s-gold.skia.org/search?corpus=%s&left_filter=name_suffix=%s&test_group=%s&execution_id=%s&not_at_head=true", goldInstance, corpus, nameSuffix, d.state.TestName(), d.executionID)

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

	if err := d.authenticateGold(ctx); err != nil {
		return errors.Wrap(err, "failed to authenticate gold")
	}
	return nil
}

func (d *differ) getDisplayInfo(ctx context.Context) (*display.Info, error) {
	if d.config.DisplayID != "" {
		return display.FindInfo(ctx, d.tconn, func(info *display.Info) bool {
			return info.ID == d.config.DisplayID
		})
	}
	infos, err := display.GetInfo(ctx, d.tconn)
	if err != nil {
		return nil, err
	}
	// Normalizing DPI on multiple displays with different DeviceScaleFactors won't work.
	if len(infos) != 1 {
		return nil, errors.New("if you want to use screenshot testing with multiple devices, you need to provide config.DisplayID")
	}
	return &infos[0], nil
}

func (d *differ) normalizeDisplayInfoAndMode(ctx context.Context) (*display.Info, *display.DisplayMode, error) {
	info, err := d.getDisplayInfo(ctx)
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
				if err := display.SetDisplayProperties(ctx, d.tconn, info.ID, display.DisplayProperties{DisplayZoomFactor: &zoomFactor}); err != nil {
					return nil, nil, errors.Wrap(err, "failed to normalize DPI")
				}
				d.reset = func() error {
					return display.SetDisplayProperties(ctx, d.tconn, info.ID, display.DisplayProperties{DisplayZoomFactor: &oldZoomFactor})
				}
			}
		}
	}
	return info, displayMode, nil
}

// Diff takes a screenshot of a ui element within the active window and uploads the
// result to gold. If finder is nil, takes a screenshot of the whole window.
// Collect all your diff results at the end with GetFailedDiffs() or DieOnFailedDiffs()
func (d *differ) Diff(ctx context.Context, name string, finder *nodewith.Finder) uiauto.Action {
	return d.DiffWithOptions(ctx, name, finder, Options{})
}

// Diff takes a screenshot of the active window and uploads the result to gold.
// Collect all your diff results at the end with GetFailedDiffs() or DieOnFailedDiffs()
func (d *differ) DiffWindow(ctx context.Context, name string) uiauto.Action {
	return d.Diff(ctx, name, nil)
}

// DiffWithOptions takes a screenshot of a ui element within the active window and uploads
// the result to gold. If finder is nil, takes a screenshot of the whole window.
// Collect all your diff results at the end with GetFailedDiffs() or DieOnFailedDiffs()
func (d *differ) DiffWithOptions(ctx context.Context, name string, finder *nodewith.Finder, options Options) uiauto.Action {
	// Prioritise per-diff options, then test options, then global defaults.
	options.FillDefaults(d.config.DefaultOptions)
	options.FillDefaults(Options{
		Timeout: time.Second * 2,
		// A window's corners are rounded, and unlike other elements, the background is inconsistent (since it's the wallpaper).
		WindowBorderWidthDP: 1,
		// Allow off-by-one in each channel.
		// Experimental results seem to show that several boards are off by a single color in some channels,
		// probably due to floating-point arithmetic. Since it's basically invisible to the end-user, ignore it.
		PixelDeltaThreshold: 3,
		// By default, retry once to ensure the screen hasn't  changed, and fail if it has changed.
		ScreenshotRetries: 1,
		// Pick a random interval so that we don't happen to always be in sync with
		// an animation (eg. If a cursor blinks every 100ms, and your interval is 1
		// second, you're unlikely to pick up this issue during development.
		ScreenshotRetryInterval: time.Duration(rand.Intn(1000))*time.Millisecond + 500*time.Millisecond})

	return func(ctx context.Context) error {
		fullName := d.state.TestName() + "." + name + d.config.Suffix()
		extraArgs, err := d.capture(ctx, name, finder, &options)
		if err != nil {
			return errors.Wrap(err, "failed to take screenshot")
		}
		if err := d.runGoldCommand(ctx, "imgtest", append(append([]string{"add",
			"--instance", goldInstance,
			"--keys-file", filepath.Join(d.dir, keysFile),
			"--test-name", fullName,
			"--png-file", filepath.Join(d.dir, name, screenshotFile),
		}, d.goldArgs...), extraArgs...)...); err != nil {
			d.failedTests = append(d.failedTests, name+d.config.Suffix())
			// In case you don't have access to a filter by commit ID / release, output the logs directly.
			testing.ContextLogf(ctx, "Failed test %s: %v", name, err)
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
	var d = &differ{state: state}
	for _, config := range configs {
		d.config = config
		// Upon resetting config, chrome needs to be re-initialized.
		d.chrome = nil
		if err := d.initialize(ctx); err != nil {
			return err
		}
		fn(d)
	}
	return d.GetFailedDiffs()
}

func (d *differ) capture(ctx context.Context, screenshotName string, finder *nodewith.Finder, options *Options) ([]string, error) {
	var testArgs []string

	ui := uiauto.New(d.tconn).WithTimeout(options.Timeout)
	window, err := ash.GetActiveWindow(ctx, d.tconn)
	if err != nil {
		// While it is technically possible to take screenshots of things outside of windows, it's a large source of flakiness.
		// * The launcher isn't a consistent color between boards
		// * Background images are inconsistent between boards
		// * Different screen resolutions can't be normalized when taking pictures of a large portion of the screen
		return testArgs, errors.Wrap(err, "unable to find focused window - screendiff only supports taking screenshots of apps")
	}
	windowBoundsDP := window.BoundsInRoot

	// Even if the window already appears to be in normal state, it may actually be in the Default state. So always set to normal.
	windowState, err := ash.SetWindowState(ctx, d.tconn, window.ID, ash.WMEventNormal)
	if err != nil {
		return testArgs, err
	}

	// You can only set the bounds of a window in normal state.
	if windowState == ash.WindowStateNormal {
		if window.CanResize && (options.WindowWidthDP == 0 || options.WindowHeightDP == 0) {
			return testArgs, errors.Errorf("please add screenshot.Config{DefaultOptions: screenshot.Options{WindowWidthDP: %d, WindowHeightDP: %d}} to your screendiff config", windowBoundsDP.Width, windowBoundsDP.Height)
		}
		// Ensure it always goes to the top-left corner of the screen. This should prevent misalignment issues.
		requestedBounds := coords.Rect{Width: windowBoundsDP.Width, Height: windowBoundsDP.Height}
		if window.CanResize {
			requestedBounds = coords.Rect{Width: options.WindowWidthDP, Height: options.WindowHeightDP}
		}
		var displayID string
		windowBoundsDP, displayID, err = ash.SetWindowBounds(ctx, d.tconn, window.ID, requestedBounds, window.DisplayID)
		if err != nil {
			return testArgs, err
		} else if displayID != window.DisplayID {
			return testArgs, errors.New("Unable to move window to correct display")
		}
	}
	if err := ash.WaitWindowFinishAnimating(ctx, d.tconn, window.ID); err != nil {
		return testArgs, errors.Wrap(err, "Unable to wait for the window to finish animating")
	}

	isAtTopLeft := windowBoundsDP.Top == 0 && windowBoundsDP.Left == 0
	windowBoundsDP = windowBoundsDP.WithInset(options.WindowBorderWidthDP, options.WindowBorderWidthDP)

	// .First() ensures it selects the outermost window element.
	// Using the .Attribute name instead of Name ensures that in other locales,
	// it won't attempt to translate (since it gets the name from the system,
	// it's already translated).
	rootElement := nodewith.Role(role.Window).Attribute("name", window.Title).First()
	if finder == nil {
		finder = rootElement
	} else {
		finder = finder.Ancestor(rootElement)
	}

	dir := filepath.Join(d.dir, screenshotName)
	if _, err := os.Stat(dir); err == nil {
		return testArgs, errors.Errorf("screenshot has already been taken for %s, please give this screenshot a unique name", screenshotName)
	}
	if err := os.Mkdir(dir, 0755); err != nil {
		return testArgs, err
	}

	if d.config.OutputUITrees {
		uiauto.LogRootDebugInfo(ctx, d.tconn, filepath.Join(dir, "ui_tree.txt"))
	}

	location, err := ui.Location(ctx, finder)
	if err != nil {
		return testArgs, errors.Wrap(err, "failed to find node to take screenshot of")
	}

	boundsPx := coords.ConvertBoundsFromDPToPX(location.Intersection(windowBoundsDP), d.uiScale)
	windowBoundsPX := coords.ConvertBoundsFromDPToPX(windowBoundsDP, d.uiScale)

	testArgs = append(testArgs,
		"--add-test-optional-key", fmt.Sprintf("cropped_resolution:%dx%d", boundsPx.Height, boundsPx.Width),
		"--add-test-optional-key", fmt.Sprintf("fuzzy_pixel_delta_threshold:%d", options.PixelDeltaThreshold),
		"--add-test-optional-key", fmt.Sprintf("is_at_top_left:%t", isAtTopLeft),
		"--add-test-optional-key", fmt.Sprintf("screenshot_name:%s", screenshotName),
		"--add-test-optional-key", fmt.Sprintf("window_size:%dx%d", windowBoundsPX.Width, windowBoundsPX.Height),
		"--add-test-optional-key", fmt.Sprintf("window_state:%s", windowState),
	)

	srcOffset := image.Point{X: boundsPx.Left, Y: boundsPx.Top}
	dstSize := image.Rect(0, 0, boundsPx.Width, boundsPx.Height)
	// rectangles removed from the cropped image.
	var removedRects []image.Rectangle
	for _, subelement := range options.RemoveElements {
		nodes, err := ui.NodesInfo(ctx, subelement.Ancestor(rootElement))
		if err != nil {
			return testArgs, err
		}
		for _, node := range nodes {
			removedRect := coords.ConvertBoundsFromDPToPX(node.Location, d.uiScale)
			removedRect.Left -= boundsPx.Left
			removedRect.Top -= boundsPx.Top
			removedRects = append(removedRects, image.Rect(removedRect.Left, removedRect.Top, removedRect.Right(), removedRect.Bottom()))
		}
	}

	takeScreenshot := func() (*image.RGBA, error) {
		img, err := CaptureChromeImage(ctx, d.chrome)
		if err != nil {
			return nil, errors.Wrap(err, "failed to capture the chrome image")
		}

		// This screenshot isn't used anywhere, but is useful for context to devs.
		f, err := os.Create(filepath.Join(dir, wholeScreenFile))
		if err != nil {
			return nil, err
		}
		png.Encode(f, img)

		// The screenshot returned is of the whole screen. Crop it to only contain the element requested by the user.
		cropped := image.NewRGBA(dstSize)
		draw.Draw(cropped, dstSize, img, srcOffset, draw.Src)

		for _, rect := range removedRects {
			draw.Draw(cropped, rect, &image.Uniform{color.Transparent}, image.ZP, draw.Src)
		}

		f, err = os.Create(filepath.Join(dir, screenshotFile))
		if err != nil {
			return nil, err
		}
		png.Encode(f, cropped)
		return cropped, nil
	}

	screenshot, err := takeScreenshot()
	if err != nil {
		return testArgs, err
	}
	var lastScreenshot *image.RGBA
	if options.ScreenshotRetries > 1 {
		if err := testing.Sleep(ctx, options.ScreenshotRetryInterval); err != nil {
			return testArgs, err
		}
	}
	if err := action.Retry(options.ScreenshotRetries, func(ctx context.Context) error {
		testing.ContextLogf(ctx, "Taking screenshot again after %q", options.ScreenshotRetryInterval)
		if err := os.Rename(filepath.Join(dir, screenshotFile), filepath.Join(dir, oldScreenshotFile)); err != nil {
			return err
		}
		if err := os.Rename(filepath.Join(dir, wholeScreenFile), filepath.Join(dir, oldWholeScreenFile)); err != nil {
			return err
		}
		lastScreenshot = screenshot

		screenshot, err = takeScreenshot()
		if err != nil {
			return err
		}
		for y := screenshot.Bounds().Min.Y; y < screenshot.Bounds().Max.Y; y++ {
			for x := screenshot.Bounds().Min.X; x < screenshot.Bounds().Max.X; x++ {
				if screenshot.RGBAAt(x, y) != lastScreenshot.RGBAAt(x, y) {
					return errors.Errorf("Screen has changed since the last screenshot. Images %s and %s differ at (%d, %d)", oldScreenshotFile, screenshotFile, x, y)
				}
			}
		}
		return nil
	}, options.ScreenshotRetryInterval)(ctx); err != nil {
		return testArgs, err
		// Cleanup the old screenshot files, since they're the same images as the new ones.
	} else if err := os.Remove(filepath.Join(dir, oldScreenshotFile)); err != nil {
		return testArgs, err
	} else if err := os.Remove(filepath.Join(dir, oldWholeScreenFile)); err != nil {
		return testArgs, err
	}

	return testArgs, nil
}

func (d *differ) authenticateGold(ctx context.Context) error {
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

	return d.runGoldCommand(ctx, "auth", "--service-account", goldServiceAccountKeyFile)
}

func (d *differ) runGoldCommand(ctx context.Context, subcommand string, args ...string) error {
	args = append([](string){subcommand, "--work-dir", goldctlWorkDir}, args...)
	testing.ContextLogf(ctx, `Running command "goldctl %v"`, args)
	if d.config.DryRun {
		return nil
	}
	cmd := testexec.CommandContext(ctx, "goldctl", args...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		// Ignore the no newlines lint rule, because it makes it much easier to see what went wrong this way.
		err = errors.Errorf("while running \"goldctl %s\"\n%s", strings.Join(args, " "), out) // NOLINT
	}
	return err
}

func (d *differ) getCPUInfo(ctx context.Context) (map[string]string, error) {
	cmd := testexec.CommandContext(ctx, "lscpu")
	out, err := cmd.Output()
	if err != nil {
		return nil, errors.Wrap(err, "lscpu failed")
	}
	result := map[string]string{}
	// Each line is of the form "Key: value"
	lineMatcher := regexp.MustCompile(`([^:]*):\s*(.*)`)
	for _, line := range strings.Split(strings.TrimSpace(string(out)), "\n") {
		match := lineMatcher.FindStringSubmatch(line)
		// Gold params cannot have spaces in them. This will look like:

		result[match[1]] = strings.ReplaceAll(match[2], " ", "_")
	}

	return result, nil
}

func currentSubPixelAntialiasingMethod() string {
	for fname, method := range subPixelAAFiles {
		if _, err := os.Stat(filepath.Join(fontConfigDir, fname)); !os.IsNotExist(err) {
			return method
		}
	}
	return "unknown"
}
