// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package camera

import (
	"context"
	"time"

	"chromiumos/tast/common/media/caps"
	"chromiumos/tast/common/perf"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/camera/cca"
	"chromiumos/tast/local/camera/testutil"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         CCAUIPerf,
		Desc:         "Opens CCA and measures the UI performance including CPU and power usage",
		Contacts:     []string{"wtlee@chromium.org", "inker@chromium.org", "chromeos-camera-eng@google.com"},
		Attr:         []string{"group:crosbolt", "crosbolt_perbuild"},
		SoftwareDeps: []string{"camera_app", "chrome", caps.BuiltinOrVividCamera},
		Data:         []string{"cca_ui.js"},
		Timeout:      5 * time.Minute,
		Pre:          chrome.LoggedIn(),
	})
}

// CCAUIPerf measure cold/warm start time of CCA and also measure its
// performance through some UI operations.
func CCAUIPerf(ctx context.Context, s *testing.State) {
	cr := s.PreValue().(*chrome.Chrome)
	tb, err := testutil.NewTestBridge(ctx, cr, testutil.UseRealCamera)
	if err != nil {
		s.Fatal("Failed to construct test bridge: ", err)
	}
	defer tb.TearDown(ctx)

	if err := cca.ClearSavedDir(ctx, cr); err != nil {
		s.Fatal("Failed to clear saved directory: ", err)
	}

	perfValues := perf.NewValues()

	if err := cca.MeasurePerformance(ctx, cr, []string{s.DataPath("cca_ui.js")}, cca.MeasurementOptions{
		PerfValues:               perfValues,
		ShouldMeasureUIBehaviors: true,
		OutputDir:                s.OutDir(),
	}, tb); err != nil {
		var errJS *cca.ErrJS
		if errors.As(err, &errJS) {
			s.Error("There are JS errors when running CCA: ", err)
		} else {
			s.Fatal("Failed to measure performance: ", err)
		}
	}

	// It is used to measure the warm start time of CCA.
	if err := cca.MeasurePerformance(ctx, cr, []string{s.DataPath("cca_ui.js")}, cca.MeasurementOptions{
		PerfValues:               perfValues,
		ShouldMeasureUIBehaviors: false,
		OutputDir:                s.OutDir(),
	}, tb); err != nil {
		var errJS *cca.ErrJS
		if errors.As(err, &errJS) {
			s.Error("There are JS errors when running CCA: ", err)
		} else {
			s.Fatal("Failed to measure warm start time: ", err)
		}
	}

	if err := perfValues.Save(s.OutDir()); err != nil {
		s.Fatal("Failed to save perf metrics: ", err)
	}
}
