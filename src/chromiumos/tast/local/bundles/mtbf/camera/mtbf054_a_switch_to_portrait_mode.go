// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package camera

import (
	"context"

	"chromiumos/tast/common/mtbferrors"
	"chromiumos/tast/local/bundles/mtbf/camera/cca"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/media/caps"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         MTBF054ASwitchToPortraitMode,
		Desc:         "Opens CCA and switch to portrait mode",
		Contacts:     []string{"xliu@cienet.com"},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"chrome", caps.BuiltinOrVividCamera},
		Data:         []string{"cca_ui.js"},
		Pre:          chrome.LoginReuse(),
	})
}

// MTBF054ASwitchToPortraitMode switch to portrait mode.
func MTBF054ASwitchToPortraitMode(ctx context.Context, s *testing.State) {
	cr := s.PreValue().(*chrome.Chrome)

	app, err := cca.New(ctx, cr, []string{s.DataPath("cca_ui.js")})
	if err != nil {
		s.Fatal(mtbferrors.New(mtbferrors.CmrOpenCCA, err))
	}

	if err := app.WaitForVideoActive(ctx); err != nil {
		s.Fatal(mtbferrors.New(mtbferrors.CmrInact, err))
	}

	// Switch to portrait mode
	s.Log("supported portrait mode")
	const portraitModeSelector = "Tast.isVisible('#modes-group > .mode-item:last-child')"
	if err := app.CheckElementExist(ctx, portraitModeSelector, true); err == nil {
		if err := app.SwitchMode(ctx, cca.Portrait); err != nil {
			s.Fatal(mtbferrors.New(mtbferrors.CmrPortrait, err))
		}
	}
}
