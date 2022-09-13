// Copyright 2020 The ChromiumOS Authors
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
	"chromiumos/tast/local/chrome/browser"
	"chromiumos/tast/local/chrome/metrics"
	"chromiumos/tast/testing"
)

// GPUDataSource is helper to get gpu data from Chrome.
type GPUDataSource struct {
	prefix   string
	tconns   map[browser.Type]*chrome.TestConn
	previous map[browser.Type]float64

	stopc chan struct{}

	dataMutex   sync.Mutex
	currentData map[browser.Type][]*metrics.Histogram
	dataErr     error
}

// NewGPUDataSource creates an instance of GPUDataSource.
func NewGPUDataSource(tconns map[browser.Type]*chrome.TestConn) *GPUDataSource {
	return &GPUDataSource{
		tconns:      tconns,
		previous:    make(map[browser.Type]float64),
		stopc:       make(chan struct{}),
		currentData: make(map[browser.Type][]*metrics.Histogram),
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
	recorders := make(map[browser.Type]*metrics.Recorder)
	for bt, tconn := range ds.tconns {
		recorder, err := metrics.StartRecorder(ctx, tconn, "Compositing.Browser.GPUMemoryForTilingsInKb")
		if err != nil {
			return err
		}
		recorders[bt] = recorder

		if err := testing.Sleep(ctx, time.Second); err != nil {
			return err
		}
		hists, err := recorder.Histogram(ctx, tconn)
		if err != nil {
			return err
		}
		ds.currentData[bt] = hists
	}

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

			for bt, recorder := range recorders {
				hists, err := recorder.Histogram(ctx, ds.tconns[bt])
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
					ds.currentData[bt] = hists
					ds.dataMutex.Unlock()
				}
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

func (ds *GPUDataSource) histograms() (map[browser.Type][]*metrics.Histogram, error) {
	ds.dataMutex.Lock()
	defer ds.dataMutex.Unlock()
	if ds.dataErr != nil {
		return nil, ds.dataErr
	}
	return ds.currentData, nil
}

// Snapshot implements perf.TimelineDatasource.Snapshot.
func (ds *GPUDataSource) Snapshot(ctx context.Context, values *perf.Values) error {
	histograms, err := ds.histograms()
	if err != nil {
		return err
	}

	// We cap 512MB for the GPU memory for each browser for tiling.
	maxGPUMemory := 512 * 1024 * len(histograms)

	GPUMemory := 0.0
	for bt, hists := range histograms {
		var memory float64
		if len(hists) == 0 || hists[0].TotalCount() == 0 {
			// When there are no updates observed, just use the previous data point.
			memory = ds.previous[bt]
		} else {
			mean, err := hists[0].Mean()
			if err != nil {
				return errors.Wrap(err, "failed to calculate mean")
			}
			memory = mean
			ds.previous[bt] = mean
		}
		values.Append(perf.Metric{
			Name:      ds.prefix + "GPU.Memory." + string(bt),
			Unit:      "KB",
			Direction: perf.SmallerIsBetter,
			Multiple:  true,
		}, memory)
		GPUMemory += memory
	}

	values.Append(perf.Metric{
		Name:      ds.prefix + "GPU.Memory",
		Unit:      "KB",
		Direction: perf.SmallerIsBetter,
		Multiple:  true,
	}, GPUMemory)

	var exceeds float64
	if GPUMemory > float64(maxGPUMemory/2) {
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
