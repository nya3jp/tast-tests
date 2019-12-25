// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package camera

import (
	"context"
	"fmt"
	"image/jpeg"
	"os"
	"path/filepath"
	"time"

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
		Attr:         []string{"informational"},
		SoftwareDeps: []string{"chrome", "arc_camera3", caps.BuiltinOrVividCamera},
		Data:         []string{"cca_ui.js"},
		Pre:          chrome.LoggedIn(),
	})
}

func CCAUIResolutions(ctx context.Context, s *testing.State) {
	cr := s.PreValue().(*chrome.Chrome)

	app, err := cca.New(ctx, cr, []string{s.DataPath("cca_ui.js")}, s.OutDir())
	if err != nil {
		s.Fatal("Failed to open CCA: ", err)
	}
	defer app.Close(ctx)
	restartApp := func() {
		if err := app.Restart(ctx); err != nil {
			s.Fatal("Failed to restart CCA: ", err)
		}
		if err := app.WaitForVideoActive(ctx); err != nil {
			s.Fatal("Preview is inactive after restart App: ", err)
		}
	}
	if err := app.WaitForVideoActive(ctx); err != nil {
		s.Fatal("Preview is inactive after launching App: ", err)
	}
	if noMenu, err := app.GetState(ctx, "no-resolution-settings"); err != nil {
		s.Fatal(`Failed to get "no-resolution-settings" state: `, err)
	} else if noMenu {
		s.Fatal("Resolution settings menu is not available on device")
	}

	saveDir, err := cca.GetSavedDir(ctx, cr)
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
			if pr.Width*or.Height != pr.Height*or.Width {
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

// getVideoTrack get video track from video file under specified path.
func getVideoTrack(path string) (*matroska.VideoTrack, error) {
	doc, err := matroska.Decode(path)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to decode video file of path %v", path)
	}
	for _, track := range doc.Segment.Tracks {
		for _, ent := range track.Entries {
			if ent.Type == matroska.TrackTypeVideo {
				return ent.Video, nil
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
			track, err := getVideoTrack(filepath.Join(saveDir, info.Name()))
			if err != nil {
				return err
			}
			if track.Width != or.Width || track.Height != or.Height {
				return errors.Wrapf(err, "incorrect captured resolution get %dx%d; want %dx%d", track.Width, track.Height, r.Width, r.Height)
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
		back := fmt.Sprintf("#%s .menu-header button", name)
		app.ClickWithSelector(ctx, back)
	}

	if err := openSetting("settings", "#open-settings"); err != nil {
		return err
	}
	defer closeSetting("settings")

	if err := openSetting("resolutionsettings", "#settings-resolution"); err != nil {
		return err
	}
	defer closeSetting("resolutionsettings")

	fname, ok := (map[cca.Facing]string{
		cca.FacingBack:     "back",
		cca.FacingFront:    "front",
		cca.FacingExternal: "external",
	})[facing]
	if !ok {
		return errors.Errorf("cannot switch resolution of unsuppport facing %v", facing)
	}

	view := fmt.Sprintf("%sresolutionsettings", rt)

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

	if err := app.ClickWithSelectorIndex(ctx, fmt.Sprintf("#%s input", view), index); err != nil {
		return errors.Wrap(err, "failed to click on resolution item")
	}
	if err := app.WaitForVideoActive(ctx); err != nil {
		return errors.Wrap(err, "preview is inactive after switching resolution")
	}
	return nil
}
