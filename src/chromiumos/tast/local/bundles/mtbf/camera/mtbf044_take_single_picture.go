// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package camera

import (
	"context"
	"strings"
	"time"

	"chromiumos/tast/common/mtbferrors"
	"chromiumos/tast/local/bundles/mtbf/camera/cca"
	"chromiumos/tast/local/bundles/mtbf/camera/common"
	"chromiumos/tast/local/bundles/mtbf/camera/gallery"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/media/caps"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         MTBF044TakeSinglePicture,
		Desc:         "Opens CCA, verifies photo taking and go to gallery",
		Contacts:     []string{"xliu@cienet.com"},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"chrome", caps.BuiltinOrVividCamera},
		Data:         []string{"cca_ui.js"},
		Pre:          chrome.LoginReuse(),
	})
}

func MTBF044TakeSinglePicture(ctx context.Context, s *testing.State) {
	cr := s.PreValue().(*chrome.Chrome)

	app, err := cca.New(ctx, cr, []string{s.DataPath("cca_ui.js")}, s.OutDir())
	if err != nil {
		if strings.Contains(err.Error(), "Chrome probably crashed") {
			s.Fatal(mtbferrors.New(mtbferrors.CmrChromeCrashed, err))
		}
		s.Fatal(mtbferrors.New(mtbferrors.CmrOpenCCA, err))
	}
	defer app.Close(ctx)
	defer common.RemoveJPGFiles(s)

	if err := app.WaitForVideoActive(ctx); err != nil {
		s.Fatal(mtbferrors.New(mtbferrors.CmrInact, err))
	}

	if _, err := app.TakeSinglePhoto(ctx, cca.TimerOff); err != nil {
		s.Fatal(mtbferrors.New(mtbferrors.CmrTakePhoto, err))
	}

	if err := app.GoToGallery(ctx); err != nil {
		s.Fatal(mtbferrors.New(mtbferrors.CmrGallery, err))
	}
	testing.Sleep(ctx, 3*time.Second)
	if err := gallery.Close(ctx, cr); err != nil {
		s.Fatal(mtbferrors.New(mtbferrors.CmrGalleryClose, err))
	}
}
