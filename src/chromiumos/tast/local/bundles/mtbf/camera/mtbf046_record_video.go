// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package camera

import (
	"context"
	"strings"
	"time"

	"chromiumos/tast/common/mtbferrors"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/bundles/mtbf/camera/cca"
	"chromiumos/tast/local/bundles/mtbf/camera/common"
	"chromiumos/tast/local/bundles/mtbf/camera/gallery"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/media/caps"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         MTBF046RecordVideo,
		Desc:         "Opens CCA, verifies video recording and go to gallery",
		Contacts:     []string{"xliu@cienet.com"},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"chrome", caps.BuiltinOrVividCamera},
		Data:         []string{"cca_ui.js"},
		Pre:          chrome.LoginReuse(),
	})
}

// MTBF046RecordVideo verifies video recording and go to gallery
func MTBF046RecordVideo(ctx context.Context, s *testing.State) {
	cr := s.PreValue().(*chrome.Chrome)

	app, err := cca.New(ctx, cr, []string{s.DataPath("cca_ui.js")}, s.OutDir())
	if err != nil {
		if strings.Contains(err.Error(), "Chrome probably crashed") {
			s.Fatal(mtbferrors.New(mtbferrors.CmrChromeCrashed, err))
		}
		s.Fatal(mtbferrors.New(mtbferrors.CmrOpenCCA, err))
	}
	defer app.Close(ctx)
	defer app.RemoveCacheData(ctx, []string{"toggleTimer"})
	defer common.RemoveMKVFiles(s)

	if err := app.WaitForVideoActive(ctx); err != nil {
		s.Fatal(mtbferrors.New(mtbferrors.CmrInact, err))
	}

	testing.ContextLog(ctx, "Switch to video mode")
	if err := app.SwitchMode(ctx, cca.Video); err != nil {
		s.Fatal(mtbferrors.New(mtbferrors.CmrVideoMode, err))
	}
	if err := app.WaitForVideoActive(ctx); err != nil {
		s.Fatal(mtbferrors.New(mtbferrors.CmrInactVd, err))
	}

	dir, err := cca.GetSavedDir(ctx, cr)
	if err != nil {
		s.Fatal("Failed to get CCA default saved path: ", err)
	}

	if err := app.RunThroughCameras(ctx, func(facing cca.Facing) error {
		testing.ContextLog(ctx, "Start test video recording")
		if err := testVideoRecording(ctx, app, dir); err != nil {
			s.Fatal(mtbferrors.New(mtbferrors.CmrVideoRecord, err))
		}
		testing.Sleep(ctx, 2*time.Second)
		testing.ContextLog(ctx, "Start test go to Gallery")
		if err := app.GoToGallery(ctx); err != nil {
			s.Fatal(mtbferrors.New(mtbferrors.CmrGallery, err))
		}
		testing.Sleep(ctx, 3*time.Second)

		testing.ContextLog(ctx, "Start test play video from Gallery")
		if err := gallery.PlayVideo(ctx, cr); err != nil {
			s.Fatal(mtbferrors.New(mtbferrors.CmrGalleryPlay, err))
		}
		testing.Sleep(ctx, 5*time.Second)
		if err := gallery.Close(ctx, cr); err != nil {
			s.Fatal(mtbferrors.New(mtbferrors.CmrGalleryClose, err))
		}
		return nil
	}); err != nil {
		s.Fatal(mtbferrors.New(mtbferrors.CmrTestsAll, err))
	}
}

// testVideoRecording starts and stops video recording
func testVideoRecording(ctx context.Context, app *cca.App, dir string) error {
	testing.ContextLog(ctx, "Click on start shutter")
	if err := app.ClickShutter(ctx); err != nil {
		return err
	}
	if err := testing.Sleep(ctx, 5*time.Second); err != nil {
		return err
	}
	start := time.Now()
	testing.ContextLog(ctx, "Click on stop shutter")
	if err := app.ClickShutter(ctx); err != nil {
		return err
	}
	if err := app.WaitForState(ctx, "taking", false); err != nil {
		return errors.Wrap(err, "shutter is not ended")
	}

	if _, err := app.WaitForFileSaved(ctx, dir, cca.VideoPattern, start); err != nil {
		return errors.Wrap(err, "cannot find result video")
	}
	return nil
}
