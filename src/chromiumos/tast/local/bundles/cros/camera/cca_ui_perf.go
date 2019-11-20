// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package camera

import (
	"context"
	"time"

	"chromiumos/tast/local/bundles/cros/camera/cca"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/media/caps"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         CCAUIPerf,
		Desc:         "Opens CCA and measures the UI performance including CPU usage",
		Contacts:     []string{"shik@chromium.org", "kelsey.deuth@intel.com", "chromeos-camera-eng@google.com"},
		Attr:         []string{"group:crosbolt", "crosbolt_perbuild"},
		SoftwareDeps: []string{"chrome", caps.BuiltinOrVividCamera},
		Data:         []string{"cca_ui.js"},
		Timeout:      3 * time.Minute,
	})
}

// CCAUIPerf measure cold/warm start time of CCA and also measure its
// performance through some UI operations.
func CCAUIPerf(ctx context.Context, s *testing.State) {
	cr, err := chrome.New(ctx)
	if err != nil {
		s.Fatal("Failed to connect to Chrome: ", err)
	}
	defer cr.Close(ctx)

	if err := cca.MeasurePerformance(ctx, cr, []string{s.DataPath("cca_ui.js")}, cca.MeasurementOptions{
		IsColdStart:              true,
		OutputDir:                s.OutDir(),
		ShouldMeasureUIBehaviors: true,
	}); err != nil {
		s.Fatal("Failed to measure performance: ", err)
	}

	// It is used to measure the warm start time of CCA.
	if err := cca.MeasurePerformance(ctx, cr, []string{s.DataPath("cca_ui.js")}, cca.MeasurementOptions{
		IsColdStart:              false,
		OutputDir:                s.OutDir(),
		ShouldMeasureUIBehaviors: false,
	}); err != nil {
		s.Fatal("Failed to measure warm start time: ", err)
	}
}
