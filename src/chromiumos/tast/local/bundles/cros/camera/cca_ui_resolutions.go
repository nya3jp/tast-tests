// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package camera

import (
	"context"
	"fmt"
	"image/jpeg"
	"math"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/abema/go-mp4"
	"github.com/pixelbender/go-matroska/matroska"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/bundles/cros/camera/cca"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/media/caps"
	"chromiumos/tast/testing"
)

// resolutionType is different capture resolution type.
type resolutionType string

const (
	photoResolution resolutionType = "photo"
	videoResolution                = "video"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         CCAUIResolutions,
		Desc:         "Opens CCA and verifies video recording related use cases",
		Contacts:     []string{"inker@chromium.org", "chromeos-camera-eng@google.com"},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"chrome", "arc_camera3", caps.BuiltinOrVividCamera},
		Data:         []string{"cca_ui.js"},
		Pre:          chrome.LoggedIn(),
	})
}

func CCAUIResolutions(ctx context.Context, s *testing.State) {
	cr := s.PreValue().(*chrome.Chrome)

	if err := cca.ClearSavedDir(ctx, cr); err != nil {
		s.Fatal("Failed to clear saved directory: ", err)
	}

	app, err := cca.New(ctx, cr, []string{s.DataPath("cca_ui.js")}, s.OutDir())
	if err != nil {
		s.Fatal("Failed to open CCA: ", err)
	}
	defer app.Close(ctx)
	defer (func() {
		if err := app.CheckJSError(ctx, s.OutDir()); err != nil {
			s.Error("Failed with javascript errors: ", err)
		}
	})()

	restartApp := func() {
		s.Log("Restarts CCA")
		if err := app.Restart(ctx); err != nil {
			s.Fatal("Failed to restart CCA: ", err)
		}
	}

	if noMenu, err := app.GetState(ctx, "no-resolution-settings"); err != nil {
		s.Fatal(`Failed to get "no-resolution-settings" state: `, err)
	} else if noMenu {
		s.Fatal("Resolution settings menu is not available on device")
	}

	saveDir, err := app.SavedDir(ctx)
	if err != nil {
		s.Fatal("Failed to get save dir: ", err)
	}

	if err := testPhotoResolution(ctx, app, saveDir); err != nil {
		s.Error("Failed in testPhotoResolution(): ", err)
		restartApp()
	}
	if err := testVideoResolution(ctx, app, saveDir); err != nil {
		s.Error("Failed in testVideoResolution(): ", err)
		restartApp()
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

func testPhotoResolution(ctx context.Context, app *cca.App, saveDir string) error {
	return app.RunThroughCameras(ctx, func(facing cca.Facing) error {
		if err := app.SwitchMode(ctx, cca.Photo); err != nil {
			return errors.Wrap(err, "failed to switch to photo mode")
		}
		rs, err := app.GetPhotoResolutions(ctx)
		if err != nil {
			return errors.Wrap(err, "failed to get photo resolution")
		}
		for i, r := range rs {
			// CCA UI will filter out photo resolutions of megapixels < 0.1 i.e. megapixels 0.0
			if r.Width*r.Height < 100000 {
				continue
			}
			testing.ContextLogf(ctx, "Switch to photo %dx%d resolution", r.Width, r.Height)
			if err := switchResolution(ctx, app, photoResolution, facing, i); err != nil {
				return errors.Wrapf(err, "failed to switch to photo resolution %dx%d", r.Width, r.Height)
			}

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
			f, err := os.Open(filepath.Join(saveDir, info[0].Name()))
			if err != nil {
				return errors.Wrap(err, "failed to open captured file")
			}
			c, err := jpeg.DecodeConfig(f)
			if err != nil {
				return errors.Wrap(err, "failed to decode captured file")
			}
			if c.Width != or.Width || c.Height != or.Height {
				return errors.Wrapf(err, "incorrect captured resolution get %dx%d; want %dx%d", c.Width, c.Height, or.Width, or.Height)
			}
		}
		return nil
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
				return &cca.Resolution{Width: intW, Height: intH}, nil
			}
		}
	}
	return nil, errors.Errorf("no video track found in the file %v", path)
}

func testVideoResolution(ctx context.Context, app *cca.App, saveDir string) error {
	return app.RunThroughCameras(ctx, func(facing cca.Facing) error {
		if err := app.SwitchMode(ctx, cca.Video); err != nil {
			return errors.Wrap(err, "failed to switch to video mode")
		}
		rs, err := app.GetVideoResolutions(ctx)
		if err != nil {
			return errors.Wrap(err, "failed to get video resolution")
		}
		for i, r := range rs {
			testing.ContextLogf(ctx, "Switch to %dx%d video resolution", r.Width, r.Height)
			if err := switchResolution(ctx, app, videoResolution, facing, i); err != nil {
				return errors.Wrapf(err, "failed to switch to video resolution %dx%d", r.Width, r.Height)
			}

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
			vr, err := videoTrackResolution(filepath.Join(saveDir, info.Name()))
			if err != nil {
				return err
			}
			if vr.Width != or.Width || vr.Height != or.Height {
				return errors.Wrapf(err, "incorrect captured resolution get %dx%d; want %dx%d", vr.Width, vr.Height, r.Width, r.Height)
			}
		}
		return nil
	})
}

// switchResolution toggles the i'th resolution of specified capture resolution of specified camera facing.
func switchResolution(ctx context.Context, app *cca.App, rt resolutionType, facing cca.Facing, index int) error {
	testing.ContextLogf(ctx, "Switch to %v th %v facing %v resolution", index, facing, rt)

	openSetting := func(name, selector string) error {
		testing.ContextLogf(ctx, "Open %q view", name)
		if err := app.ClickWithSelector(ctx, selector); err != nil {
			return err
		}
		if active, err := app.GetState(ctx, name); err != nil {
			return errors.Wrap(err, "failed to get view open state")
		} else if active != true {
			return errors.Errorf("view %q is not openned", name)
		}
		return nil
	}

	closeSetting := func(name string) {
		testing.ContextLogf(ctx, "Close %q view", name)
		back := (map[string]cca.UIComponent{
			"view-settings":                  cca.SettingsBackButton,
			"view-resolution-settings":       cca.ResolutionSettingBackButton,
			"view-photo-resolution-settings": cca.PhotoResolutionSettingBackButton,
			"view-video-resolution-settings": cca.VideoResolutionSettingBackButton,
		})[name]
		app.Click(ctx, back)
	}

	if err := openSetting("view-settings", "#open-settings"); err != nil {
		return err
	}
	defer closeSetting("view-settings")

	if err := openSetting("view-resolution-settings", "#settings-resolution"); err != nil {
		return err
	}
	defer closeSetting("view-resolution-settings")

	fname, ok := (map[cca.Facing]string{
		cca.FacingBack:     "back",
		cca.FacingFront:    "front",
		cca.FacingExternal: "external",
	})[facing]
	if !ok {
		return errors.Errorf("cannot switch resolution of unsuppport facing %v", facing)
	}

	view := fmt.Sprintf("view-%s-resolution-settings", rt)

	settingSelector := fmt.Sprintf("#settings-%s-%sres", fname, rt)
	if facing == cca.FacingExternal {
		id, err := app.GetDeviceID(ctx)
		if err != nil {
			return errors.Wrap(err, "failed to get device id of external camera")
		}
		settingSelector = fmt.Sprintf("button[aria-describedby='%s-%sres-desc']", id, rt)
	}
	if err := openSetting(view, settingSelector); err != nil {
		return err
	}
	defer closeSetting(view)

	optionUI := cca.PhotoResolutionOption
	if rt == videoResolution {
		optionUI = cca.VideoResolutionOption
	}

	if checked, err := app.IsCheckedWithIndex(ctx, optionUI, index); err != nil {
		return err
	} else if checked {
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
	return nil
}
