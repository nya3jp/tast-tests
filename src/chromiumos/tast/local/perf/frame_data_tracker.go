// Copyright 2020 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package perf

import (
	"context"
	"time"

	"chromiumos/tast/common/perf"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/testing"
)

const frameDataFetchInterval = time.Minute

// FrameDataTracker is helper to get animation frame data from Chrome.
type FrameDataTracker struct {
	prefix        string
	animationData []DisplayFrameData
	dsData        *DisplayFrameData
	dsTracker     *DisplaySmoothnessTracker
	collecting    chan bool
	collectingErr chan error
}

// Close ensures that the browser state (display smoothness tracking) is cleared.
func (t *FrameDataTracker) Close(ctx context.Context, tconn *chrome.TestConn) error {
	return t.dsTracker.Close(ctx, tconn)
}

// Start starts the animation data collection.
func (t *FrameDataTracker) Start(ctx context.Context, tconn *chrome.TestConn) error {
	if t.collecting != nil {
		return errors.New("already started")
	}

	if err := tconn.Call(ctx, nil, `tast.promisify(chrome.autotestPrivate.startThroughputTrackerDataCollection)`); err != nil {
		return errors.Wrap(err, "failed to start data collection")
	}

	if err := t.dsTracker.Start(ctx, tconn, ""); err != nil {
		return errors.Wrap(err, "failed to start display smoothness tracking")
	}

	t.collecting = make(chan bool)
	t.collectingErr = make(chan error, 1)

	go func() {
		testing.ContextLog(ctx, "FrameDataTracker: Collecting frame data in background")
		for {
			select {
			case <-t.collecting:
				close(t.collectingErr)
				return
			case <-time.After(frameDataFetchInterval):
				var data []DisplayFrameData
				if err := tconn.Call(ctx, &data, `tast.promisify(chrome.autotestPrivate.getThroughputTrackerData)`); err != nil {
					t.collectingErr <- errors.Wrap(err, "failed to get collected data")
					return
				}
				t.animationData = append(t.animationData, data...)
			case <-ctx.Done():
				t.collectingErr <- ctx.Err()
				return
			}
		}
	}()

	return nil
}

// Stop stops the animation data collection and stores the collected data.
func (t *FrameDataTracker) Stop(ctx context.Context, tconn *chrome.TestConn) error {
	if t.collecting == nil {
		return errors.New("not started")
	}
	close(t.collecting)

	var firstErr error
	select {
	case firstErr = <-t.collectingErr:
	case <-ctx.Done():
		return ctx.Err()
	}

	var dsData *DisplayFrameData
	var err error
	if dsData, err = t.dsTracker.Stop(ctx, tconn, ""); err != nil {
		if firstErr == nil {
			firstErr = errors.Wrap(err, "failed to stop display smoothness tracking")
		}
	}

	var data []DisplayFrameData
	if err := tconn.Call(ctx, &data, `tast.promisify(chrome.autotestPrivate.stopThroughputTrackerDataCollection)`); err != nil {
		if firstErr == nil {
			firstErr = errors.Wrap(err, "failed to stop data collection")
		}
	}

	if firstErr != nil {
		return firstErr
	}

	t.dsData = dsData
	t.animationData = append(t.animationData, data...)
	return nil
}

// Record stores the collected data into pv for further processing.
func (t *FrameDataTracker) Record(pv *perf.Values) {
	feMetric := perf.Metric{
		Name:      t.prefix + "Animation.FramesExpected",
		Unit:      "count",
		Direction: perf.BiggerIsBetter,
		Multiple:  true,
	}
	fpMetric := perf.Metric{
		Name:      t.prefix + "Animation.FramesProduced",
		Unit:      "count",
		Direction: perf.BiggerIsBetter,
		Multiple:  true,
	}
	jcMetric := perf.Metric{
		Name:      t.prefix + "Animation.JankCount",
		Unit:      "count",
		Direction: perf.SmallerIsBetter,
		Multiple:  true,
	}

	for _, data := range t.animationData {
		pv.Append(feMetric, float64(data.FramesExpected))
		pv.Append(fpMetric, float64(data.FramesProduced))
		pv.Append(jcMetric, float64(data.JankCount))
	}

	// FrameData collecting on the DUTs may fail (b/210185705) or return no data.
	// Check if data is collected before recording it.
	if t.dsData == nil {
		return
	}

	pv.Set(perf.Metric{
		Name:      t.prefix + "DisplayJankMetric",
		Unit:      "percent",
		Direction: perf.SmallerIsBetter,
	}, float64(t.dsData.JankCount)/float64(t.dsData.FramesExpected)*100)

	pv.Set(perf.Metric{
		Name:      t.prefix + "Display.FramesExpected",
		Unit:      "count",
		Direction: perf.BiggerIsBetter,
	}, float64(t.dsData.FramesExpected))
	pv.Set(perf.Metric{
		Name:      t.prefix + "Display.FramesProduced",
		Unit:      "count",
		Direction: perf.BiggerIsBetter,
	}, float64(t.dsData.FramesProduced))
	pv.Set(perf.Metric{
		Name:      t.prefix + "Display.JankCount",
		Unit:      "count",
		Direction: perf.SmallerIsBetter,
	}, float64(t.dsData.JankCount))

	smMetric := perf.Metric{
		Name:      t.prefix + "Display.Smoothness",
		Multiple:  true,
		Unit:      "percent",
		Direction: perf.BiggerIsBetter,
	}
	for _, data := range t.dsData.Throughput {
		pv.Append(smMetric, float64(data))
	}
}

// NewFrameDataTracker creates a new instance for FrameDataTracker.
func NewFrameDataTracker(metricPrefix string) (*FrameDataTracker, error) {
	return &FrameDataTracker{
		prefix:    metricPrefix,
		dsTracker: NewDisplaySmoothnessTracker(),
	}, nil
}
