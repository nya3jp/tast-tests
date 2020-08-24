// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package cuj

import (
	"context"

	"chromiumos/tast/common/perf"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/chrome"
)

// AnimationFrameData stores frame data for an animation.
type AnimationFrameData struct {
	FramesExpected int `json:"framesExpected"`
	FramesProduced int `json:"framesProduced"`
}

// FrameDataTracker is helper to get animation frame data from Chrome.
type FrameDataTracker struct {
	animationData []AnimationFrameData
}

// Start starts the animation data collection.
func (t *FrameDataTracker) Start(ctx context.Context, tconn *chrome.TestConn) error {
	if err := tconn.EvalPromise(ctx,
		`tast.promisify(chrome.autotestPrivate.startThroughputTrackerDataCollection)()`,
		nil); err != nil {
		return errors.Wrap(err, "failed to start data collection")
	}

	return nil
}

// Stop stops the animation data collection and stores the collected data.
func (t *FrameDataTracker) Stop(ctx context.Context, tconn *chrome.TestConn) error {
	var data []AnimationFrameData
	if err := tconn.EvalPromise(ctx,
		`tast.promisify(chrome.autotestPrivate.stopThroughputTrackerDataCollection)()`,
		&data); err != nil {
		return errors.Wrap(err, "failed to stop data collection")
	}

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

	for _, data := range t.animationData {
		pv.Set(feMetric, float64(data.FramesExpected))
		pv.Set(fpMetric, float64(data.FramesProduced))
	}
}

// NewFrameDataTracker creates a new instance for FrameDataTracker.
func NewFrameDataTracker() (*FrameDataTracker, error) {
	return &FrameDataTracker{}, nil
}
