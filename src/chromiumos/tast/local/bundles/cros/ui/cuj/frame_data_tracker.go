// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package cuj

import (
	"context"

	"chromiumos/tast/common/perf"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/bundles/cros/ui/perfutil"
	"chromiumos/tast/local/chrome"
)

// FrameDataTracker is helper to get animation frame data from Chrome.
type FrameDataTracker struct {
	animationData []perfutil.DisplayFrameData
	ds            float64
	dsData        *perfutil.DisplayFrameData
	dsTracker     *perfutil.DisplaySmoothnessTracker
}

// Close ensures that the browser state (display smoothness tracking) is cleared.
func (t *FrameDataTracker) Close(ctx context.Context, tconn *chrome.TestConn) error {
	return t.dsTracker.Close(ctx, tconn)
}

// Start starts the animation data collection.
func (t *FrameDataTracker) Start(ctx context.Context, tconn *chrome.TestConn) error {
	if err := tconn.Call(ctx, nil, `tast.promisify(chrome.autotestPrivate.startThroughputTrackerDataCollection)`); err != nil {
		return errors.Wrap(err, "failed to start data collection")
	}

	if err := t.dsTracker.Start(ctx, tconn, ""); err != nil {
		return errors.Wrap(err, "failed to start display smoothness tracking: ")
	}
	return nil
}

// Stop stops the animation data collection and stores the collected data.
func (t *FrameDataTracker) Stop(ctx context.Context, tconn *chrome.TestConn) error {
	var ds float64
	var dsData *perfutil.DisplayFrameData
	var err error
	if ds, dsData, err = t.dsTracker.Stop(ctx, tconn, ""); err != nil {
		return errors.Wrap(err, "failed to stop display smoothness tracking")
	}

	var data []perfutil.DisplayFrameData
	if err := tconn.Call(ctx, &data, `tast.promisify(chrome.autotestPrivate.stopThroughputTrackerDataCollection)`); err != nil {
		return errors.Wrap(err, "failed to stop data collection")
	}

	t.ds = ds
	t.dsData = dsData
	t.animationData = append(t.animationData, data...)
	return nil
}

// Record stores the collected data into pv for further processing.
func (t *FrameDataTracker) Record(pv *perf.Values) {
	if len(t.animationData) == 0 {
		return
	}

	feMetric := perf.Metric{
		Name:      "TPS.Animation.FramesExpected",
		Unit:      "count",
		Direction: perf.BiggerIsBetter,
		Multiple:  true,
	}
	fpMetric := perf.Metric{
		Name:      "TPS.Animation.FramesProduced",
		Unit:      "count",
		Direction: perf.BiggerIsBetter,
		Multiple:  true,
	}
	jcMetric := perf.Metric{
		Name:      "TPS.Animation.JankCount",
		Unit:      "count",
		Direction: perf.SmallerIsBetter,
		Multiple:  true,
	}

	for _, data := range t.animationData {
		pv.Append(feMetric, float64(data.FramesExpected))
		pv.Append(fpMetric, float64(data.FramesProduced))
		pv.Append(jcMetric, float64(data.JankCount))
	}

	if t.dsData != nil {
		pv.Set(perf.Metric{
			Name:      "TPS.DisplaySmoothness",
			Unit:      "percent",
			Direction: perf.BiggerIsBetter,
		}, float64(t.dsData.FramesProduced)/float64(t.dsData.FramesExpected)*100)
		pv.Set(perf.Metric{
			Name:      "TPS.DisplayJankMetric",
			Unit:      "percent",
			Direction: perf.SmallerIsBetter,
		}, float64(t.dsData.JankCount)/float64(t.dsData.FramesExpected)*100)
	} else {
		pv.Set(perf.Metric{
			Name:      "TPS.DisplaySmoothness",
			Unit:      "percent",
			Direction: perf.BiggerIsBetter,
		}, t.ds)
	}
}

// NewFrameDataTracker creates a new instance for FrameDataTracker.
func NewFrameDataTracker() (*FrameDataTracker, error) {
	return &FrameDataTracker{
		dsTracker: perfutil.NewDisplaySmoothnessTracker(),
	}, nil
}
