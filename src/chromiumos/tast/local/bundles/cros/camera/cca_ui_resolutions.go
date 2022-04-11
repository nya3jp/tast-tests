// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package camera

import (
	"context"
	"image/jpeg"
	"math"
	"os"
	"strconv"
	"time"

	"github.com/abema/go-mp4"
	"github.com/rwcarlsen/goexif/exif"

	"chromiumos/tast/common/media/caps"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/camera/cca"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         CCAUIResolutions,
		LacrosStatus: testing.LacrosVariantUnneeded,
		Desc:         "Opens CCA and verifies video recording related use cases",
		Contacts:     []string{"inker@chromium.org", "chromeos-camera-eng@google.com"},
		Attr:         []string{"group:mainline", "informational", "group:camera-libcamera"},
		SoftwareDeps: []string{"camera_app", "chrome", "arc_camera3", caps.BuiltinOrVividCamera},
		Fixture:      "ccaTestBridgeReady",
		// Default timeout (i.e. 2 minutes) is not enough for some devices to
		// exercise all resolutions on all cameras.
		Timeout: 5 * time.Minute,
	})
}

func CCAUIResolutions(ctx context.Context, s *testing.State) {
	runTestWithApp := s.FixtValue().(cca.FixtureData).RunTestWithApp
	subTestTimeout := 2 * time.Minute
	for _, tst := range []struct {
		name     string
		testFunc func(context.Context, *cca.App) error
	}{{
		"testPhotoResolution",
		testPhotoResolution,
	}, {
		"testVideoResolution",
		testVideoResolution,
	}} {
		subTestCtx, cancel := context.WithTimeout(ctx, subTestTimeout)
		s.Run(subTestCtx, tst.name, func(ctx context.Context, s *testing.State) {
			if err := runTestWithApp(ctx, func(ctx context.Context, app *cca.App) error {
				if noMenu, err := app.State(ctx, "no-resolution-settings"); err != nil {
					return errors.Wrap(err, `failed to get "no-resolution-settings" state`)
				} else if noMenu {
					return errors.New("resolution settings menu is not available on device")
				}
				return tst.testFunc(ctx, app)
			}, cca.TestWithAppParams{}); err != nil {
				s.Errorf("Failed to pass %v subtest: %v", tst.name, err)
			}
		})
		cancel()
	}
}

// getOrientedResolution gets resolution with respect to screen orientation.
func getOrientedResolution(ctx context.Context, app *cca.App, r cca.Resolution) (cca.Resolution, error) {
	orientation, err := app.GetScreenOrientation(ctx)
	if err != nil {
		return r, err
	}
	isLandscape := (orientation == cca.LandscapePrimary || orientation == cca.LandscapeSecondary)
	if isLandscape != (r.Width > r.Height) {
		r.Width, r.Height = r.Height, r.Width
	}
	testing.ContextLogf(ctx, "Screen orientation %v, resolution after orientation %dx%d", orientation, r.Width, r.Height)
	return r, nil
}

func imageResolution(path string, handleOrientation bool) (*cca.Resolution, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, errors.Wrap(err, "failed to open captured file for decoding jpeg")
	}
	defer f.Close()
	c, err := jpeg.DecodeConfig(f)
	if err != nil {
		return nil, errors.Wrap(err, "failed to decode captured file")
	}

	if handleOrientation {
		// Read display rotation number from exif.
		f2, err := os.Open(path)
		if err != nil {
			return nil, errors.Wrap(err, "failed to open captured file for reading exif")
		}
		defer f2.Close()
		x, err := exif.Decode(f2)
		if err != nil {
			return nil, errors.Wrap(err, "failed to decode exif")
		}
		tag, err := x.Get(exif.Orientation)
		if err != nil {
			return nil, errors.Wrap(err, "failed to get orientation from exif")
		}
		o, err := tag.Int(0)
		if err != nil {
			return nil, errors.Wrap(err, "failed to get integer value of orientation tag")
		}
		if 5 <= o && o <= 8 {
			return &cca.Resolution{Width: c.Height, Height: c.Width}, nil
		}
	}
	return &cca.Resolution{Width: c.Width, Height: c.Height}, nil
}

func testPhotoResolution(ctx context.Context, app *cca.App) error {
	// TODO(b/215484798): Remove the logic for old UI once the new UI applied.
	useNewUI, err := app.Exist(ctx, cca.PhotoResolutionSettingButton)
	if err != nil {
		return errors.Wrap(err, "failed to check existence of the photo resolution settings button")
	}
	if useNewUI {
		return testPhotoResolutionAndAspectRatio(ctx, app)
	}

	// The test logic for legacy photo resolutions UI.
	return app.RunThroughCameras(ctx, func(facing cca.Facing) error {
		if err := app.SwitchMode(ctx, cca.Photo); err != nil {
			return errors.Wrap(err, "failed to switch to photo mode")
		}
		return iterateResolutions(ctx, app, cca.PhotoResolution, facing, func(r cca.Resolution) error {
			or, err := getOrientedResolution(ctx, app, r)
			if err != nil {
				return err
			}

			pr, err := app.GetPreviewResolution(ctx)
			if err != nil {
				return err
			}

			// aspectRatioTolerance is the small aspect ratio
			// comparison tolerance for judging the treeya's
			// special resolution 848:480(1.766...) which should be
			// counted as 16:9(1.77...).
			const aspectRatioTolerance = 0.02
			if math.Abs(pr.AspectRatio()-or.AspectRatio()) > aspectRatioTolerance {
				return errors.Wrapf(err, "inconsistent preview aspect ratio get %d:%d; want %d:%d", pr.Width, pr.Height, or.Width, or.Height)
			}

			ir, err := takePhotoAndGetResolution(ctx, app, true)
			if err != nil {
				return err
			}
			if ir.Width != or.Width || ir.Height != or.Height {
				return errors.Wrapf(err, "incorrect captured resolution get %dx%d; want %dx%d", ir.Width, ir.Height, or.Width, or.Height)
			}
			return nil
		})
	})
}

// takePhotoAndGetResolution takes a photo and extract the resolution of the taken photo
func takePhotoAndGetResolution(ctx context.Context, app *cca.App, handleOrientation bool) (*cca.Resolution, error) {
	info, err := app.TakeSinglePhoto(ctx, cca.TimerOff)
	if err != nil {
		return nil, errors.Wrap(err, "failed to take photo")
	}
	path, err := app.FilePathInSavedDir(ctx, info[0].Name())
	if err != nil {
		return nil, errors.Wrap(err, "failed to get file path")
	}
	ir, err := imageResolution(path, handleOrientation)
	if err != nil {
		return nil, errors.Wrap(err, "failed to get image resolution")
	}
	return ir, nil
}

// recordVideoAndGetResolution records a video and extract the resolution of the recoreded video
func recordVideoAndGetResolution(ctx context.Context, app *cca.App) (*cca.Resolution, error) {
	info, err := app.RecordVideo(ctx, cca.TimerOff, time.Second)
	if err != nil {
		return nil, errors.Wrap(err, "failed to record video")
	}
	path, err := app.FilePathInSavedDir(ctx, info.Name())
	if err != nil {
		return nil, errors.Wrap(err, "failed to get file path")
	}
	vr, err := videoTrackResolution(path)
	if err != nil {
		return nil, errors.Wrap(err, "failed to extract resolution from the video")
	}
	return vr, nil
}

// videoTrackResolution returns the resolution from video file under specified path.
func videoTrackResolution(path string) (*cca.Resolution, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to open video file %v", path)
	}
	defer file.Close()

	boxes, err := mp4.ExtractBoxWithPayload(
		file, nil, mp4.BoxPath{mp4.BoxTypeMoov(), mp4.BoxTypeTrak(), mp4.BoxTypeTkhd()})
	if err != nil {
		return nil, errors.Wrapf(err, "failed to find track boxes in video file %v", path)
	}
	for _, b := range boxes {
		thkd := b.Payload.(*mp4.Tkhd)
		if thkd.Width > 0 && thkd.Height > 0 {
			// Ignore the 16 low bits of fractional part.
			intW := int(thkd.Width >> 16)
			intH := int(thkd.Height >> 16)

			// All possible rotation matrices(values in matrices are 32-bit fixed-point) in mp4 thkd produced from CCA.
			rotate0 := [9]int32{65536, 0, 0, 0, 65536, 0, 0, 0, 1073741824}
			rotate90 := [9]int32{0, 65536, 0, -65536, 0, 0, 0, 0, 1073741824}
			rotate180 := [9]int32{-65536, 0, 0, 0, -65536, 0, 0, 0, 1073741824}
			rotate270 := [9]int32{0, -65536, 0, 65536, 0, 0, 0, 0, 1073741824}
			switch thkd.Matrix {
			case rotate0:
			case rotate180:
			case rotate90:
				fallthrough
			case rotate270:
				intW, intH = intH, intW
			default:
				return nil, errors.Errorf("unknown mp4 thkd matrix %v", thkd.Matrix)
			}
			return &cca.Resolution{Width: intW, Height: intH}, nil
		}
	}
	return nil, errors.Errorf("no video track found in the file %v", path)
}

func testVideoResolution(ctx context.Context, app *cca.App) error {
	// TODO(b/215484798): Remove the logic for old UI once the new UI applied.
	useNewUI, err := app.Exist(ctx, cca.VideoResolutionSettingButton)
	if err != nil {
		return errors.Wrap(err, "failed to check existence of the video resolution settings button")
	}
	if useNewUI {
		return testVideoResolutionAndFPS(ctx, app)
	}

	// The test logic for legacy video resolutions UI.
	return app.RunThroughCameras(ctx, func(facing cca.Facing) error {
		if err := app.SwitchMode(ctx, cca.Video); err != nil {
			return errors.Wrap(err, "failed to switch to video mode")
		}
		return iterateResolutions(ctx, app, cca.VideoResolution, facing, func(r cca.Resolution) error {
			or, err := getOrientedResolution(ctx, app, r)
			if err != nil {
				return err
			}

			pr, err := app.GetPreviewResolution(ctx)
			if err != nil {
				return err
			}
			if pr.Width*or.Height != pr.Height*or.Width {
				return errors.Wrapf(err, "inconsistent preview aspect ratio get %d:%d; want %d:%d", pr.Width, pr.Height, or.Width, or.Height)
			}

			vr, err := recordVideoAndGetResolution(ctx, app)
			if err != nil {
				return err
			}
			if vr.Width != or.Width || vr.Height != or.Height {
				return errors.Wrapf(err, "incorrect captured resolution get %dx%d; want %dx%d", vr.Width, vr.Height, or.Width, or.Height)
			}
			return nil
		})
	})
}

// withInnerResolutionSetting opens inner |rt| type resolution menu for |facing| camera, calls |onOpened()| and closes the menu.
func withInnerResolutionSetting(ctx context.Context, app *cca.App, rt cca.ResolutionType, facing cca.Facing, onOpened func() error) error {
	if err := cca.MainMenu.Open(ctx, app); err != nil {
		return err
	}
	defer cca.MainMenu.Close(ctx, app)

	if err := cca.ResolutionMenu.Open(ctx, app); err != nil {
		return err
	}
	defer cca.ResolutionMenu.Close(ctx, app)

	innerMenu, err := app.InnerResolutionSetting(ctx, facing, rt)
	if err != nil {
		return err
	}
	if err := innerMenu.Open(ctx, app); err != nil {
		return err
	}
	defer innerMenu.Close(ctx, app)

	return onOpened()
}

// iterateResolutions toggles through all |rt| resolutions in camera |facing| setting menu and calls |onSwitched| with the toggled resolution.
func iterateResolutions(ctx context.Context, app *cca.App, rt cca.ResolutionType, facing cca.Facing, onSwitched func(r cca.Resolution) error) error {
	optionUI := cca.PhotoResolutionOption
	if rt == cca.VideoResolution {
		optionUI = cca.VideoResolutionOption
	}

	var numOptions int
	if err := withInnerResolutionSetting(ctx, app, rt, facing, func() error {
		count, err := app.CountUI(ctx, optionUI)
		if err != nil {
			return err
		}
		numOptions = count
		return nil
	}); err != nil {
		return err
	}

	toggleOption := func(index int) (cca.Resolution, error) {
		var r cca.Resolution
		err := withInnerResolutionSetting(ctx, app, rt, facing, func() error {
			width, err := attributeValueOfOption(ctx, app, optionUI, index, "data-width")
			if err != nil {
				return err
			}
			height, err := attributeValueOfOption(ctx, app, optionUI, index, "data-height")
			if err != nil {
				return err
			}
			if err := clickOptionAndWaitConfiguration(ctx, app, optionUI, index); err != nil {
				return errors.Wrap(err, "failed to click option and wait configration done")
			}
			r.Width = width
			r.Height = height
			return nil
		})
		return r, err
	}

	for index := 0; index < numOptions; index++ {
		r, err := toggleOption(index)
		if err != nil {
			return err
		}
		if err := onSwitched(r); err != nil {
			return err
		}
	}

	return nil
}

func clickOptionAndWaitConfiguration(ctx context.Context, app *cca.App, optionUI cca.UIComponent, index int) error {
	testing.ContextLogf(ctx, "Switch to #%v of %v", index, optionUI.Name)
	checked, err := app.IsCheckedWithIndex(ctx, optionUI, index)
	if err != nil {
		return err
	}
	if checked {
		testing.ContextLogf(ctx, "#%d resolution option is already checked", index)
	} else {
		if err := app.TriggerConfiguration(ctx, func() error {
			testing.ContextLogf(ctx, "Checking with #%d resolution option", index)
			if err := app.ClickWithIndex(ctx, optionUI, index); err != nil {
				return errors.Wrap(err, "failed to click on resolution item")
			}
			return nil
		}); err != nil {
			return errors.Wrap(err, "camera configuration failed after switching resolution")
		}
	}

	return nil
}

// attributeValueOfOption returns the attribute value of |index| th of
// |optionsUI| given by the name of the attribute. The value will be in integer.
func attributeValueOfOption(ctx context.Context, app *cca.App, optionsUI cca.UIComponent, index int, attribute string) (int, error) {
	stringValue, err := app.AttributeWithIndex(ctx, optionsUI, index, attribute)
	if err != nil {
		return -1, err
	}
	intValue, err := strconv.Atoi(stringValue)
	if err != nil {
		return -1, errors.Wrapf(err, "failed to convert the value (%v) of the attribute (%v) to int", stringValue, attribute)
	}
	return intValue, nil
}

// getOrientedAspectRatio gets aspect ratio with respect to screen orientation.
func getOrientedAspectRatio(ctx context.Context, app *cca.App, aspectRatio float64) (float64, error) {
	orientation, err := app.GetScreenOrientation(ctx)
	if err != nil {
		return 0, err
	}
	isLandscape := (orientation == cca.LandscapePrimary || orientation == cca.LandscapeSecondary)
	if (isLandscape && aspectRatio < 1) || (!isLandscape && aspectRatio > 1) {
		aspectRatio = 1 / aspectRatio
	}
	return aspectRatio, nil
}

// testPhotoResolutionAndAspectRatio tests the mutual behavior of photo resolutions and aspect ratio.
func testPhotoResolutionAndAspectRatio(ctx context.Context, app *cca.App) error {
	return app.RunThroughCameras(ctx, func(facing cca.Facing) error {
		if facing == cca.FacingExternal {
			return nil
		}

		if err := app.SwitchMode(ctx, cca.Photo); err != nil {
			return errors.Wrap(err, "failed to switch to photo mode")
		}

		if err := cca.MainMenu.Open(ctx, app); err != nil {
			return err
		}
		defer cca.MainMenu.Close(ctx, app)

		if err := cca.PhotoAspectRatioMenu.Open(ctx, app); err != nil {
			return err
		}
		defer cca.PhotoAspectRatioMenu.Close(ctx, app)

		aspectRatioOptions := cca.FrontAspectRatioOptions
		if facing == cca.FacingBack {
			aspectRatioOptions = cca.BackAspectRatioOptions
		}
		numOptions, err := app.CountUI(ctx, aspectRatioOptions)
		if err != nil {
			return errors.Wrap(err, "failed to count the aspect ratio options")
		} else if numOptions < 2 {
			// Ensure that at least two options are provided since "square" will
			// always be an option.
			return errors.Wrapf(err, "unexpected amount of options: %v", numOptions)
		}

		// Checks that when changing the aspect ratio preference of current
		// running camera, the camera stream will be reconfigured.
		for index := 0; index < numOptions; index++ {
			if err := clickOptionAndWaitConfiguration(ctx, app, aspectRatioOptions, index); err != nil {
				return errors.Wrap(err, "failed to click the aspect ratio option and wait for the configration done")
			}

			value, err := app.AttributeWithIndex(ctx, aspectRatioOptions, index, "data-aspect-ratio")
			if err != nil {
				return err
			}
			aspectRatio, err := strconv.ParseFloat(value, 64)
			if err != nil {
				return errors.Wrapf(err, "failed to convert aspect ratio value %v to float", aspectRatio)
			}
			orientedAspectRatio, err := getOrientedAspectRatio(ctx, app, aspectRatio)
			if err != nil {
				return errors.Wrap(err, "failed to get oriented aspect ratio value")
			}

			// For each aspect ratio, tests through all possible photo resolution options in the photos resolution settings.
			if err := clickThroughAllPhotoResolutionOptions(ctx, app, facing, orientedAspectRatio); err != nil {
				return errors.Wrap(err, "failed to check photo resolution options")
			}
		}
		return nil
	})
}

// checkResolutionAspectRatio checks the aspect ratio of |resolution| is close to the given |aspectRatio|.
func checkResolutionAspectRatio(ctx context.Context, resolution *cca.Resolution, aspectRatio float64) error {
	// aspectRatioTolerance is the small aspect ratio comparison tolerance for
	// judging the treeya's special resolution 848:480(1.766...) which should be
	// counted as 16:9(1.77...).
	const aspectRatioTolerance = 0.02
	if math.Abs(resolution.AspectRatio()-aspectRatio) > aspectRatioTolerance {
		return errors.Errorf("inconsistent aspect ratio. Resolution: %d:%d; Want aspect ratio: %v", resolution.Width, resolution.Height, aspectRatio)
	}
	return nil
}

// clickThroughAllPhotoResolutionOptions tries out all the photo resolution options under current aspect ratio and ensures the configuration works successfully.
func clickThroughAllPhotoResolutionOptions(ctx context.Context, app *cca.App, facing cca.Facing, aspectRatio float64) error {
	if err := cca.PhotoAspectRatioMenu.Close(ctx, app); err != nil {
		return errors.Wrap(err, "failed to close the aspect ratio settings page")
	}
	defer cca.PhotoAspectRatioMenu.Open(ctx, app)

	if err := cca.PhotoResolutionMenu.Open(ctx, app); err != nil {
		return err
	}
	defer cca.PhotoResolutionMenu.Close(ctx, app)

	photoResolotionOptions := cca.FrontPhotoResolutionOptions
	if facing == cca.FacingBack {
		photoResolotionOptions = cca.BackPhotoResolutionOptions
	}

	numOptions, err := app.CountUI(ctx, photoResolotionOptions)
	if err != nil {
		return errors.Wrap(err, "failed to count the aspect ratio options")
	}

	for index := 0; index < numOptions; index++ {
		if err := clickOptionAndWaitConfiguration(ctx, app, photoResolotionOptions, index); err != nil {
			return errors.Wrap(err, "failed to click the aspect ratio option and wait for the configration done")
		}

		// Ensure preview viewport has correct aspect ratio.
		pr, err := app.GetPreviewViewportSize(ctx)
		if err != nil {
			return err
		}
		if err := checkResolutionAspectRatio(ctx, &pr, aspectRatio); err != nil {
			return err
		}

		// Ensure captured photo has correct aspect ratio.
		ir, err := takePhotoAndGetResolution(ctx, app, aspectRatio != float64(1))
		if err != nil {
			return err
		}
		if err := checkResolutionAspectRatio(ctx, ir, aspectRatio); err != nil {
			return err
		}
	}
	return nil
}

// testVideoResolutionAndFPS tests the behavior of selecting video resolution and the FPS buttons.
func testVideoResolutionAndFPS(ctx context.Context, app *cca.App) error {
	// The FPS buttons are currently available on external cameras only. Add
	// corresponding tests once they are supported on built-in cameras.
	return app.RunThroughCameras(ctx, func(facing cca.Facing) error {
		if facing == cca.FacingExternal {
			return nil
		}

		if err := app.SwitchMode(ctx, cca.Video); err != nil {
			return errors.Wrap(err, "failed to switch to video mode")
		}

		if err := cca.MainMenu.Open(ctx, app); err != nil {
			return err
		}
		defer cca.MainMenu.Close(ctx, app)

		if err := cca.VideoResolutionMenu.Open(ctx, app); err != nil {
			return err
		}
		defer cca.VideoResolutionMenu.Close(ctx, app)

		videoResolutionOptions := cca.FrontVideoResolutionOptions
		if facing == cca.FacingBack {
			videoResolutionOptions = cca.BackVideoResolutionOptions
		}
		numOptions, err := app.CountUI(ctx, videoResolutionOptions)
		if err != nil {
			return errors.Wrap(err, "failed to count the video resolution options")
		}

		for index := 0; index < numOptions; index++ {
			if err := clickOptionAndWaitConfiguration(ctx, app, videoResolutionOptions, index); err != nil {
				return errors.Wrap(err, "failed to click the aspect ratio option and wait for the configration done")
			}

			width, err := attributeValueOfOption(ctx, app, videoResolutionOptions, index, "data-width")
			if err != nil {
				return err
			}
			height, err := attributeValueOfOption(ctx, app, videoResolutionOptions, index, "data-height")
			if err != nil {
				return err
			}

			shouldCheckResolution := (width == 3840 && height == 2160) ||
				(width == 2560 && height == 1440) ||
				(width == 1920 && height == 1080) ||
				(width == 1280 && height == 720)
			if !shouldCheckResolution {
				continue
			}

			r := cca.Resolution{width, height}
			or, err := getOrientedResolution(ctx, app, r)
			if err != nil {
				return errors.Wrap(err, "failed to get oriented resolution")
			}
			// Ensure preview viewport has correct aspect ratio as the target video resolution.
			pr, err := app.GetPreviewViewportSize(ctx)
			if err != nil {
				return err
			}
			if err := checkResolutionAspectRatio(ctx, &pr, or.AspectRatio()); err != nil {
				return err
			}

			// Ensure captured video has correct resolution.
			vr, err := recordVideoAndGetResolution(ctx, app)
			if err != nil {
				return err
			}
			if vr.Width != or.Width || vr.Height != or.Height {
				return errors.Wrapf(err, "incorrect captured resolution get %dx%d; want %dx%d", vr.Width, vr.Height, or.Width, or.Height)
			}
		}
		return nil
	})
}
