// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package camera

import (
	"context"
	"path/filepath"
	"strings"
	"time"

	"chromiumos/tast/common/mtbferrors"
	"chromiumos/tast/fsutil"
	"chromiumos/tast/local/bundles/mtbf/camera/cca"
	"chromiumos/tast/local/bundles/mtbf/camera/gallery"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/media/caps"
	"chromiumos/tast/local/mtbf/debug"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         MTBF047GoToGallery,
		Desc:         "Opens CCA and Go to gallery",
		Contacts:     []string{"xliu@cienet.com"},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"chrome", caps.BuiltinOrVividCamera},
		Data:         []string{"cca_ui.js", "pixel-1280x720.jpg"},
		Pre:          chrome.LoginReuse(),
	})
}

// MTBF047GoToGallery opens cca and go to gallery
func MTBF047GoToGallery(ctx context.Context, s *testing.State) {
	ss := &debug.Screenshots{}
	// Take screenshots every two seconds.
	if err := ss.Start(ctx, 2); err != nil {
		s.Log("Failed to start screenshots: ", err)
	}
	defer ss.Stop()

	cr := s.PreValue().(*chrome.Chrome)

	// Copy pixel-1280x720.jpg to /home/chronos/user/Downloads/,
	// where MTBF047GoToGallery requires test data to exist in.
	const (
		dataDir  = "/home/chronos/user/Downloads/"
		testFile = "pixel-1280x720.jpg"
		fileName = "IMG_00000000_000000.jpg"
	)
	if err := fsutil.CopyFile(s.DataPath(testFile),
		filepath.Join(dataDir, fileName)); err != nil {
		s.Fatal(mtbferrors.New(mtbferrors.VideoCopy, err, testFile, dataDir))
	}

	app, err := cca.New(ctx, cr, []string{s.DataPath("cca_ui.js")}, s.OutDir())
	if err != nil {
		if strings.Contains(err.Error(), "Chrome probably crashed") {
			s.Fatal(mtbferrors.New(mtbferrors.CmrChromeCrashed, err))
		}
		s.Fatal(mtbferrors.New(mtbferrors.CmrOpenCCA, err))
	}
	defer app.Close(ctx)

	// Take one screenshot
	debug.TakeScreenshot(ctx)
	if err := app.WaitForVideoActive(ctx); err != nil {
		s.Fatal(mtbferrors.New(mtbferrors.CmrInact, err))
	}

	if err := app.GoToGallery(ctx); err != nil {
		s.Fatal(mtbferrors.New(mtbferrors.CmrGallery, err))
	}
	testing.Sleep(ctx, 3*time.Second)
	if err := gallery.Close(ctx, cr); err != nil {
		s.Fatal(mtbferrors.New(mtbferrors.CmrGalleryClose, err))
	}
}
