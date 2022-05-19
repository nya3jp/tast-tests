// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package cuj

import (
	"chromiumos/tast/errors"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/ui/cujrecorder"
)

// AddPerformanceCUJMetrics adds the metrics to the recorder for performance CUJ test.
func AddPerformanceCUJMetrics(tconn, bTconn *chrome.TestConn, recorder *cujrecorder.Recorder) error {
	// Use MetricsCollectFullSystem to collect as many metrics as possible from the browsers.
	aMetrics, lMetrics, cMetrics := cujrecorder.BrowserBasedMetricConfigs(cujrecorder.MetricsCollectFullSystem)
	if err := recorder.AddCollectedMetrics(tconn, append(aMetrics, cMetrics...)...); err != nil {
		errors.Wrap(err, "failed to add metrics for tconn")
	}
	if bTconn != nil {
		if err := recorder.AddCollectedMetrics(bTconn, append(lMetrics, cMetrics...)...); err != nil {
			errors.Wrap(err, "failed to add metrics for bTconn")
		}
	}
	return nil
}
