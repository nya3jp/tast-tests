// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package camera

import (
	"context"

	"chromiumos/tast/local/bundles/cros/camera/cca"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/media/caps"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         CCAUIRecordVideoPerf,
		Desc:         "Opens CCA, measures the CPU/power usage and collects some performance metrics during video recording",
		Contacts:     []string{"wtlee@chromium.org", "chromeos-camera-eng@google.com"},
		Attr:         []string{"group:crosbolt", "crosbolt_perbuild"},
		SoftwareDeps: []string{"chrome", caps.BuiltinOrVividCamera},
		Data:         []string{"cca_ui.js"},
		Pre:          chrome.LoggedIn(),
	})
}

// CCAUIRecordVideoPerf launches the Chrome Camera App, waits for camera preview, fullscreens the
// application, start recording video and starts measuring system CPU usage and power consumption.
func CCAUIRecordVideoPerf(ctx context.Context, s *testing.State) {
	cr := s.PreValue().(*chrome.Chrome)

	err := cca.MeasurePerformance(ctx, cr, []string{s.DataPath("cca_ui.js")}, s.OutDir(), true)
	if err != nil {
		s.Fatal("Failed to measure performance: ", err)
	}
}
