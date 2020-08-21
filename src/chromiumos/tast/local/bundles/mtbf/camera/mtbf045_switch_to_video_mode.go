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
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/media/caps"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         MTBF045SwitchToVideoMode,
		Desc:         "Opens CCA and verifies mode switching",
		Contacts:     []string{"xliu@cienet.com"},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"chrome", caps.BuiltinOrVividCamera},
		Data:         []string{"cca_ui.js"},
		Pre:          chrome.LoginReuse(),
	})
}

// MTBF045SwitchToVideoMode verifies switching to video mode in the camera app
func MTBF045SwitchToVideoMode(ctx context.Context, s *testing.State) {
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
	testing.Sleep(ctx, 3*time.Second)
}
