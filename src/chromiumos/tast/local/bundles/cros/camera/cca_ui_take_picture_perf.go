// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package camera

import (
	"context"

	"chromiumos/tast/local/bundles/cros/camera/cca"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/media/caps"
	"chromiumos/tast/local/media/cpu"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         CCAUITakePicturePerf,
		Desc:         "Opens CCA and collects some performance metrics during photo taking",
		Contacts:     []string{"wtlee@chromium.org", "chromeos-camera-eng@google.com"},
		Attr:         []string{"group:crosbolt", "crosbolt_perbuild"},
		SoftwareDeps: []string{"chrome", caps.BuiltinOrVividCamera},
		Data:         []string{"cca_ui.js"},
		Pre:          chrome.LoggedIn(),
	})
}

// CCAUITakePicturePerf launches the Chrome Camera App, waits for camera preview, taking a picture
// and collects the performance metrics.
func CCAUITakePicturePerf(ctx context.Context, s *testing.State) {
	cr := s.PreValue().(*chrome.Chrome)

	// Waits until CPU is idle so that the result won't affect by previous tests or initialization.
	if err := cpu.WaitUntilIdle(ctx); err != nil {
		s.Fatal("Failed to idle: ", err)
	}

	app, err := cca.New(ctx, cr, []string{s.DataPath("cca_ui.js")})
	if err != nil {
		s.Fatal("Failed to open CCA: ", err)
	}
	defer app.Close(ctx)

	if err := app.WaitForVideoActive(ctx); err != nil {
		s.Fatal("Failed to wait for video active")
	}

	app.TakeSinglePhoto(ctx, cca.TimerOff)

	if err := app.CollectPerfEvents(ctx, s.OutDir()); err != nil {
		s.Fatal("Failed to collect perf events: ", err)
	}
}
