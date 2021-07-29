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
	"strings"
	"time"

	"github.com/abema/go-mp4"
	"github.com/pixelbender/go-matroska/matroska"
	"github.com/rwcarlsen/goexif/exif"

	"chromiumos/tast/common/media/caps"
	"chromiumos/tast/ctxutil"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/camera/cca"
	"chromiumos/tast/local/camera/testutil"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         CCAUIResolutions,
		Desc:         "Opens CCA and verifies video recording related use cases",
		Contacts:     []string{"inker@chromium.org", "chromeos-camera-eng@google.com"},
		Attr:         []string{"group:mainline", "informational", "group:camera-libcamera"},
		SoftwareDeps: []string{"camera_app", "chrome", "arc_camera3", caps.BuiltinOrVividCamera},
		Data:         []string{"cca_ui.js"},
		Pre:          chrome.LoggedIn(),
		// Default timeout (i.e. 2 minutes) is not enough for some devices to
		// exercise all resolutions on all cameras.
		Timeout: 5 * time.Minute,
	})
}

func CCAUIResolutions(ctx context.Context, s *testing.State) {
	cr := s.PreValue().(*chrome.Chrome)
	tb, err := testutil.NewTestBridge(ctx, cr, testutil.UseRealCamera)
	if err != nil {
		s.Fatal("Failed to construct test bridge: ", err)
	}
	defer tb.TearDown(ctx)

	if err := cca.ClearSavedDirs(ctx, cr); err != nil {
		s.Fatal("Failed to clear saved directory: ", err)
	}

	app, err := cca.New(ctx, cr, []string{s.DataPath("cca_ui.js")}, s.OutDir(), tb)
	if err != nil {
		s.Fatal("Failed to open CCA: ", err)
	}
	defer func(ctx context.Context) {
		if err := app.Close(ctx); err != nil {
			s.Error("Failed to close app: ", err)
		}
	}(ctx)

	if noMenu, err := app.GetState(ctx, "no-resolution-settings"); err != nil {
		s.Fatal(`Failed to get "no-resolution-settings" state: `, err)
	} else if noMenu {
		s.Fatal("Resolution settings menu is not available on device")
	}

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
			shortCtx, cancel := ctxutil.Shorten(ctx, 10*time.Second)
			defer cancel()

			if err := cca.ClearSavedDirs(ctx, cr); err != nil {
				s.Fatal("Failed to clear saved directory: ", err)
			}

			if err := tst.testFunc(shortCtx, app); err != nil {
				s.Fatalf("Failed to run subtest: %v: %v", tst.name, err)
			}

			// Restart app using non-shorten context.
			if err := app.Restart(ctx, tb); err != nil {
				s.Fatal("Failed to restart CCA: ", err)
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

func imageResolution(path string) (*cca.Resolution, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, errors.Wrap(err, "failed to open captured file for decoding jpeg")
	}
	defer f.Close()
	c, err := jpeg.DecodeConfig(f)
	if err != nil {
		return nil, errors.Wrap(err, "failed to decode captured file")
	}

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
	return &cca.Resolution{Width: c.Width, Height: c.Height}, nil
}

func testPhotoResolution(ctx context.Context, app *cca.App) error {
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

			info, err := app.TakeSinglePhoto(ctx, cca.TimerOff)
			if err != nil {
				return errors.Wrap(err, "failed to take photo")
			}
			path, err := app.FilePathInSavedDirs(ctx, info[0].Name())
			if err != nil {
				return errors.Wrap(err, "failed to get file path")
			}
			ir, err := imageResolution(path)
			if err != nil {
				return errors.Wrap(err, "failed to get image resolution")
			}
			if ir.Width != or.Width || ir.Height != or.Height {
				return errors.Wrapf(err, "incorrect captured resolution get %dx%d; want %dx%d", ir.Width, ir.Height, or.Width, or.Height)
			}
			return nil
		})
	})
}

// videoTrackResolution returns the resolution from video file under specified path.
func videoTrackResolution(path string) (*cca.Resolution, error) {
	if strings.HasSuffix(path, ".mkv") {
		doc, err := matroska.Decode(path)
		if err != nil {
			return nil, errors.Wrapf(err, "failed to decode video file %v", path)
		}
		for _, track := range doc.Segment.Tracks {
			for _, ent := range track.Entries {
				if ent.Type == matroska.TrackTypeVideo {
					return &cca.Resolution{Width: ent.Video.Width, Height: ent.Video.Height}, nil
				}
			}
		}
	} else if strings.HasSuffix(path, ".mp4") {
		file, err := os.Open(path)
		defer file.Close()
		if err != nil {
			return nil, errors.Wrapf(err, "failed to open video file %v", path)
		}
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
				rotate90 := [9]int32{0, 65536, 0, -2147418112, 0, 0, 0, 0, 1073741824}
				rotate180 := [9]int32{-2147418112, 0, 0, 0, -2147418112, 0, 0, 0, 1073741824}
				rotate270 := [9]int32{0, -2147418112, 0, 65536, 0, 0, 0, 0, 1073741824}
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
	}
	return nil, errors.Errorf("no video track found in the file %v", path)
}

func testVideoResolution(ctx context.Context, app *cca.App) error {
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

			info, err := app.RecordVideo(ctx, cca.TimerOff, time.Second)
			if err != nil {
				return errors.Wrap(err, "failed to record video")
			}
			path, err := app.FilePathInSavedDirs(ctx, info.Name())
			if err != nil {
				return errors.Wrap(err, "failed to get file path")
			}
			vr, err := videoTrackResolution(path)
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
			value, err := app.AttributeWithIndex(ctx, optionUI, index, "data-width")
			if err != nil {
				return err
			}
			width, err := strconv.Atoi(value)
			if err != nil {
				return errors.Wrapf(err, "failed to convert width value %v to int", width)
			}
			value, err = app.AttributeWithIndex(ctx, optionUI, index, "data-height")
			if err != nil {
				return err
			}
			height, err := strconv.Atoi(value)
			if err != nil {
				return errors.Wrapf(err, "failed to convert height value %v to int", height)
			}

			testing.ContextLogf(ctx, "Switch to %v facing %v resolution %dx%d", facing, rt, width, height)
			checked, err := app.IsCheckedWithIndex(ctx, optionUI, index)
			if err != nil {
				return err
			}
			if checked {
				testing.ContextLogf(ctx, "%d th resolution option is already checked", index)
			} else {
				if err := app.TriggerConfiguration(ctx, func() error {
					testing.ContextLogf(ctx, "Checking with %d th resolution option", index)
					if err := app.ClickWithIndex(ctx, optionUI, index); err != nil {
						return errors.Wrap(err, "failed to click on resolution item")
					}
					return nil
				}); err != nil {
					return errors.Wrap(err, "camera configuration failed after switching resolution")
				}
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
