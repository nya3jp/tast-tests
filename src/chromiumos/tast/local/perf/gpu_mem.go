// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package perf

import (
	"context"
	"sync"
	"time"

	"chromiumos/tast/common/perf"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/metrics"
	"chromiumos/tast/testing"
)

// GPUDataSource is helper to get gpu data from Chrome.
type GPUDataSource struct {
	prefix   string
	tconn    *chrome.TestConn
	previous float64
	stopc    chan struct{}

	dataMutex   sync.Mutex
	currentData []*metrics.Histogram
	dataErr     error
}

// NewGPUDataSource creates an instance of GPUDataSource.
func NewGPUDataSource(tconn *chrome.TestConn) *GPUDataSource {
	return &GPUDataSource{
		tconn: tconn,
		stopc: make(chan struct{}),
	}
}

// Close stops the background goroutine in this data source.
func (ds *GPUDataSource) Close() {
	close(ds.stopc)
}

// Setup implements perf.TimelineDatasource.Setup.
func (ds *GPUDataSource) Setup(ctx context.Context, prefix string) error {
	ds.prefix = prefix
	return nil
}

// Start implements perf.TimelineDatasource.Start.
func (ds *GPUDataSource) Start(ctx context.Context) error {
	recorder, err := metrics.StartRecorder(ctx, ds.tconn, "Compositing.Browser.GPUMemoryForTilingsInKb")
	if err != nil {
		return err
	}

	if err := testing.Sleep(ctx, time.Second); err != nil {
		return err
	}
	hists, err := recorder.Histogram(ctx, ds.tconn)
	if err != nil {
		return err
	}
	ds.currentData = hists

	// Record the data continuously on background; otherwise it may disturbe other
	// timeline data because the histogram fetching may take a long time
	// (sometimes more than >300msecs).
	const interval = time.Second
	go func() {
		for {
			nextTick := time.Now().Add(interval)
			select {
			case <-ds.stopc:
				return
			case <-ctx.Done():
				return
			default:
				break
			}

			hists, err := recorder.Histogram(ctx, ds.tconn)
			if err != nil {
				ds.dataMutex.Lock()
				ds.dataErr = err
				ds.dataMutex.Unlock()
				return
			}
			var totalCount int64
			for _, h := range hists {
				totalCount += h.TotalCount()
			}
			if totalCount > 0 {
				ds.dataMutex.Lock()
				ds.currentData = hists
				ds.dataMutex.Unlock()
			}

			now := time.Now()
			if now.Before(nextTick) {
				select {
				case <-time.After(nextTick.Sub(now)):
					break
				case <-ds.stopc:
					return
				case <-ctx.Done():
					return
				}
			}
		}
	}()
	return nil
}

func (ds *GPUDataSource) histograms() ([]*metrics.Histogram, error) {
	ds.dataMutex.Lock()
	defer ds.dataMutex.Unlock()
	if ds.dataErr != nil {
		return nil, ds.dataErr
	}
	return ds.currentData, nil
}

// Snapshot implements perf.TimelineDatasource.Snapshot.
func (ds *GPUDataSource) Snapshot(ctx context.Context, values *perf.Values) error {
	// We cap 512MB for the GPU memory for tiling.
	const maxGPUMemory = 512 * 1024

	hists, err := ds.histograms()
	if err != nil {
		return err
	}
	var memory float64
	if len(hists) == 0 || hists[0].TotalCount() == 0 {
		// When there's no updates observed, just use the previous data point.
		memory = ds.previous
	} else {
		mean, err := hists[0].Mean()
		if err != nil {
			return errors.Wrap(err, "failed to calculate mean")
		}
		memory = mean
	}
	values.Append(perf.Metric{
		Name:      ds.prefix + "GPU.Memory",
		Unit:      "KB",
		Direction: perf.SmallerIsBetter,
		Multiple:  true,
	}, memory)

	var exceeds float64
	if memory > maxGPUMemory/2 {
		exceeds = 1
	}
	values.Append(perf.Metric{
		Name:      ds.prefix + "GPU.Memory.Exceeds50Percent",
		Unit:      "boolean",
		Direction: perf.SmallerIsBetter,
		Multiple:  true,
	}, exceeds)
	return nil
}

// Stop does nothing.
func (ds *GPUDataSource) Stop(_ context.Context, values *perf.Values) error {
	return nil
}
