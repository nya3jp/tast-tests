// Copyright 2018 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package metrics

import (
	"context"
	"fmt"
	"math"
	"os"
	"reflect"
	"strings"
	"time"

	"golang.org/x/sys/unix"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/perf"
	"chromiumos/tast/testing"
)

const (
	// histogramTransferFile is the file that ChromeOS services write out Histogram
	// data to. At intervals, Chrome's chromeos::ExternalMetrics comes through and
	// adds events from this file into Chrome's internal list of Histograms. This
	// can be a hidden, asynchronous source of diffs to Histograms that are not
	// purely Chrome-based.
	histogramTransferFile = "/var/lib/metrics/uma-events"
)

// Histogram contains data from a single Chrome histogram.
type Histogram struct {
	// Name of the histogram.
	Name string
	// The sum of the all entries in the histogram.
	Sum int64 `json:"sum"`
	// Buckets contains ranges of reported values.
	// The buckets are disjoint and stored in ascending order.
	Buckets []HistogramBucket `json:"buckets"`
}

// validate checks h's buckets and returns an error if any have invalid ranges or overlap.
func (h *Histogram) validate() error {
	var lastMax int64 = math.MinInt64
	for _, b := range h.Buckets {
		if b.Min >= b.Max {
			return errors.Errorf("invalid bucket [%d,%d)", b.Min, b.Max)
		}
		if b.Min < lastMax {
			return errors.Errorf("bucket [%d,%d) overlaps previous bucket", b.Min, b.Max)
		}
		lastMax = b.Max
	}
	return nil
}

// TotalCount returns the total number of samples stored across all buckets.
func (h *Histogram) TotalCount() int64 {
	var t int64
	for _, b := range h.Buckets {
		t += b.Count
	}
	return t
}

// Diff returns a histogram containing the additional samples in h that aren't in old, an older version of the same histogram.
// Buckets that haven't changed are omitted from the returned histogram.
// old must be an earlier snapshot -- an error is returned if any counts decreased or if old contains buckets not present in h.
func (h *Histogram) Diff(old *Histogram) (*Histogram, error) {
	if h.Name != old.Name {
		return nil, errors.Errorf("unmatched histogram, %s vs %s", h.Name, old.Name)
	}
	if len(old.Buckets) > len(h.Buckets) {
		return nil, errors.Errorf("old histogram has %d bucket(s), new only has %d", len(old.Buckets), len(h.Buckets))
	}

	diff := &Histogram{Name: h.Name, Sum: h.Sum - old.Sum}
	oi := 0
	for _, hb := range h.Buckets {
		// If we've already looked at all of the old buckets, copy the new bucket over.
		if oi >= len(old.Buckets) {
			diff.Buckets = append(diff.Buckets, hb)
			continue
		}

		ob := old.Buckets[oi]

		switch {
		case ob.Min < hb.Min:
			// The old histogram shouldn't contain any buckets that aren't in the new one.
			return nil, errors.Errorf("bucket [%d,%d) is present in old histogram but not new one", ob.Min, ob.Max)
		case ob.Min > hb.Min:
			// If this bucket isn't present in the old histogram, just copy it over.
			if ob.Min < hb.Max {
				return nil, errors.Errorf("old bucket [%d,%d) overlaps new bucket [%d,%d)", ob.Min, ob.Max, hb.Min, hb.Max)
			}
			diff.Buckets = append(diff.Buckets, hb)
		case ob.Min == hb.Min:
			// If we're looking at the same bucket in both histograms, save the difference (if any) and move to the next old bucket.
			if ob.Max != hb.Max {
				return nil, errors.Errorf("old bucket [%d,%d) doesn't match new bucket [%d,%d)", ob.Min, ob.Max, hb.Min, hb.Max)
			}
			if hb.Count < ob.Count {
				return nil, errors.Errorf("old bucket [%d,%d) has count %d, new only has %d", ob.Min, ob.Max, ob.Count, hb.Count)
			} else if hb.Count > ob.Count {
				diff.Buckets = append(diff.Buckets, HistogramBucket{hb.Min, hb.Max, hb.Count - ob.Count})
			}
			oi++
		}
	}
	return diff, nil
}

// String contains a human-readable representation of h as "Name: [[0,5):2 [5,10):1 ...]",
// where each space-separated term is "[<min>,<max>):<count>".
func (h *Histogram) String() string {
	var strs []string
	for _, b := range h.Buckets {
		strs = append(strs, fmt.Sprintf("[%d,%d):%d", b.Min, b.Max, b.Count))
	}
	return h.Name + ": [" + strings.Join(strs, " ") + "]; " + fmt.Sprintf("sum %d", h.Sum)
}

// Mean calculates the estimated mean of the histogram values. It is an error
// when there are no data points.
func (h *Histogram) Mean() (float64, error) {
	if h.TotalCount() == 0 {
		return 0, errors.New("no histogram data")
	}
	return float64(h.Sum) / float64(h.TotalCount()), nil
}

// HistogramBucket contains a set of reported samples within a fixed range.
type HistogramBucket struct {
	// Min contains the minimum value that can be stored in this bucket.
	Min int64 `json:"min"`
	// Max contains the exclusive maximum value for this bucket.
	Max int64 `json:"max"`
	// Count contains the number of samples that are stored in this bucket.
	Count int64 `json:"count"`
}

// GetHistogram returns the current state of a Chrome histogram (e.g. "Tabs.TabCountActiveWindow").
// If no samples have been reported for the histogram since Chrome was started, the zero value for
// Histogram is returned.
func GetHistogram(ctx context.Context, tconn *chrome.TestConn, name string) (*Histogram, error) {
	h := Histogram{Name: name}
	expr := fmt.Sprintf(`tast.promisify(chrome.autotestPrivate.getHistogram)(%q)`, name)
	if err := tconn.EvalPromise(ctx, expr, &h); err != nil {
		if strings.Contains(err.Error(), fmt.Sprintf("Histogram %s not found", name)) {
			return &Histogram{Name: name}, nil
		}
		return nil, err
	}
	if err := h.validate(); err != nil {
		return nil, errors.Wrapf(err, "bad histogram %v", h)
	}
	return &h, nil
}

// WaitForHistogram is a convenience function that calls GetHistogram until the requested histogram is available,
// ctx's deadline is reached, or timeout (if positive) has elapsed.
func WaitForHistogram(ctx context.Context, tconn *chrome.TestConn, name string, timeout time.Duration) (*Histogram, error) {
	var h *Histogram
	err := testing.Poll(ctx, func(ctx context.Context) error {
		var err error
		h, err = GetHistogram(ctx, tconn, name)
		if err != nil {
			return err
		}
		if len(h.Buckets) == 0 {
			return errors.Errorf("histogram %s not found", name)
		}
		return nil
	}, &testing.PollOptions{Timeout: timeout})
	return h, err
}

// WaitForHistogramUpdate is a convenience function that calls GetHistogram until the requested histogram contains
// at least one sample not present in old, an earlier snapshot of the same histogram.
// A histogram containing the new samples is returned; see Histogram.Diff for details.
func WaitForHistogramUpdate(ctx context.Context, tconn *chrome.TestConn, name string,
	old *Histogram, timeout time.Duration) (*Histogram, error) {
	var h *Histogram
	err := testing.Poll(ctx, func(ctx context.Context) error {
		var err error
		if h, err = GetHistogram(ctx, tconn, name); err != nil {
			return err
		}
		if reflect.DeepEqual(h, old) {
			return errors.New("histogram unchanged")
		}
		return nil
	}, &testing.PollOptions{Timeout: timeout})

	if err != nil {
		return nil, err
	}
	return h.Diff(old)
}

// GetHistograms is a convenience function to get multiple histograms.
func GetHistograms(ctx context.Context, tconn *chrome.TestConn, histogramNames []string) ([]*Histogram, error) {
	var result []*Histogram
	for _, name := range histogramNames {
		histogram, err := GetHistogram(ctx, tconn, name)
		if err != nil {
			return nil, err
		}
		result = append(result, histogram)
	}
	return result, nil
}

// DiffHistograms is a convenience function to diff multiple histograms.
func DiffHistograms(older []*Histogram, newer []*Histogram) ([]*Histogram, error) {
	if len(older) != len(newer) {
		return nil, errors.New("histogram count mismatched")
	}

	var result []*Histogram

	for i := 0; i < len(older); i++ {
		if newer[i].Name != older[i].Name {
			return nil, errors.Errorf("unmatched histogram, index %d, %s vs %s", i, newer[i].Name, older[i].Name)
		}
		histogram, err := newer[i].Diff(older[i])
		if err != nil {
			return nil, err
		}
		result = append(result, histogram)
	}
	return result, nil
}

// Recorder tracks a snapshot to calculate diffs since start or last wait call.
type Recorder struct {
	snapshot []*Histogram
}

// names returns names of the histograms tracked by the recorder.
func (r *Recorder) names() []string {
	var names []string
	for _, h := range r.snapshot {
		names = append(names, h.Name)
	}
	return names
}

// StartRecorder captures a snapshot to calculate histograms diffs later.
func StartRecorder(ctx context.Context, tconn *chrome.TestConn, names ...string) (*Recorder, error) {
	s, err := GetHistograms(ctx, tconn, names)
	if err != nil {
		return nil, errors.Wrap(err, "failed to get snapshot")
	}

	return &Recorder{snapshot: s}, nil
}

// Histogram returns the histogram diffs since the recorder is started.
func (r *Recorder) Histogram(ctx context.Context, tconn *chrome.TestConn) ([]*Histogram, error) {
	names := r.names()

	s, err := GetHistograms(ctx, tconn, names)
	if err != nil {
		return nil, errors.Wrap(err, "failed to get snapshot")
	}

	return DiffHistograms(r.snapshot, s)
}

// waitMode defines ways to wait.
type waitMode int

const (
	waitAny waitMode = iota
	waitAll
)

// wait implements the wait logic for WaitAny and WaitAll.
func (r *Recorder) wait(ctx context.Context, tconn *chrome.TestConn, mode waitMode, timeout time.Duration) ([]*Histogram, error) {
	names := r.names()
	if len(names) == 0 {
		return nil, errors.New("no histograms to wait")
	}

	var diffs []*Histogram
	err := testing.Poll(ctx, func(ctx context.Context) error {
		s, err := GetHistograms(ctx, tconn, names)
		if err != nil {
			return err
		}

		diffs, err = DiffHistograms(r.snapshot, s)
		if err != nil {
			return err
		}

		cnt := 0
		for _, diff := range diffs {
			if len(diff.Buckets) != 0 {
				cnt++
			}
		}

		if mode == waitAll {
			if cnt != len(s) {
				return errors.New("not all histogram changed")
			}
		} else {
			if cnt == 0 {
				return errors.New("histograms unchanged")
			}
		}

		return nil
	}, &testing.PollOptions{Timeout: timeout})

	if err != nil {
		return nil, errors.Wrap(err, "failed to wait")
	}

	return diffs, nil
}

// WaitAny waits for update from any of histograms being recorded and returns
// the diffs since the recorder is started.
func (r *Recorder) WaitAny(ctx context.Context, tconn *chrome.TestConn, timeout time.Duration) ([]*Histogram, error) {
	return r.wait(ctx, tconn, waitAny, timeout)
}

// WaitAll waits for update from all of histograms being recorded and returns
// the diffs since the recorder is started.
func (r *Recorder) WaitAll(ctx context.Context, tconn *chrome.TestConn, timeout time.Duration) ([]*Histogram, error) {
	return r.wait(ctx, tconn, waitAll, timeout)
}

// Run is a helper to calculate histogram diffs before and after running a given
// function.
func Run(ctx context.Context, tconn *chrome.TestConn, f func() error, names ...string) ([]*Histogram, error) {
	r, err := StartRecorder(ctx, tconn, names...)
	if err != nil {
		return nil, err
	}

	if err := f(); err != nil {
		return nil, err
	}

	return r.Histogram(ctx, tconn)
}

// ClearHistogramTransferFile clears the histogramTransferFile. The
// histogramTransferFile is how Histograms get from ChromeOS services into Chrome --
// ChromeOS services write their updates to the file, and periodicially,
// Chrome's chromeos::ExternalMetrics services scrapes the file and adds the
// Histograms to its own internal list which this package queries.
//
// While this system works well in production, it can be a source of unexpected
// asynchronous changes in Histograms during tests. Tests intending to watch a
// Histogram which originates in ChromeOS should clear this file before
// establishing a "before" Histogram. This will avoid having events from previous
// tests show up as unexpected diffs in the current test.
func ClearHistogramTransferFile() error {
	return clearHistogramTransferFileByName(histogramTransferFile)
}

// clearHistogramTransferFileByName is the heart of ClearHistogramTransferFile.
// It is broken up into a separate function for ease of testing.
func clearHistogramTransferFileByName(fileName string) error {
	file, err := os.OpenFile(fileName, os.O_RDWR, 0666)
	if os.IsNotExist(err) {
		// File doesn't exist, so it's already truncated.
		return nil
	}
	if err != nil {
		return errors.Wrapf(err, "unable to open %s", fileName)
	}
	defer file.Close()

	if err := unix.Flock(int(file.Fd()), unix.LOCK_EX); err != nil {
		return errors.Wrapf(err, "unable to lock %s", fileName)
	}
	defer unix.Flock(int(file.Fd()), unix.LOCK_UN)

	if err := unix.Ftruncate(int(file.Fd()), 0); err != nil {
		return errors.Wrapf(err, "unable to truncate %s", fileName)
	}

	return nil
}

// StoreHistogramsMean stores the mean of each histogram as the performance metrics value.
func StoreHistogramsMean(ctx context.Context, pv *perf.Values, histograms []*Histogram, metric perf.Metric) error {
	for _, h := range histograms {
		mean, err := h.Mean()
		if err != nil {
			return errors.Wrapf(err, "failed to get mean for histogram %s: %v", h.Name, err)
		}
		testing.ContextLogf(ctx, "h name: %s mean value: %f", h.Name, mean)

		metric.Name = fmt.Sprintf("%s", h.Name)
		pv.Set(metric, mean)
	}

	return nil
}
