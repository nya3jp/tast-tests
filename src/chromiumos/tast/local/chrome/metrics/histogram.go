// Copyright 2018 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package metrics

import (
	"context"
	"fmt"
	"math"
	"reflect"
	"strings"
	"time"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/testing"
)

// Histogram contains data from a single Chrome histogram.
type Histogram struct {
	// Name of the histogram.
	Name string
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

	diff := &Histogram{Name: h.Name}
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
	return h.Name + ": [" + strings.Join(strs, " ") + "]"
}

// Mean calculates the estimated mean of the histogram values. Returns 0 if
// there are no data points.
func (h *Histogram) Mean() float64 {
	if h.TotalCount() == 0 {
		return 0
	}
	var sum float64
	for i, bucket := range h.Buckets {
		// For some histograms which record times in buckets such as presentation time, the max value of the last bucket is max int value.
		// To prevent samples which fall into the last bucket from skewing the mean, use the min value as the max value.
		max := bucket.Max
		if i >= len(h.Buckets)-1 {
			max = bucket.Min
		}
		sum += (float64(max) + float64(bucket.Min)) * float64(bucket.Count)
	}
	return sum / (float64(h.TotalCount()) * 2)
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
func GetHistogram(ctx context.Context, cr *chrome.Chrome, name string) (*Histogram, error) {
	conn, err := cr.TestAPIConn(ctx)
	if err != nil {
		return nil, err
	}

	h := Histogram{Name: name}
	expr := fmt.Sprintf(`tast.promisify(chrome.autotestPrivate.getHistogram)(%q)`, name)
	if err := conn.EvalPromise(ctx, expr, &h); err != nil {
		if strings.Contains(err.Error(), fmt.Sprintf("Histogram %s not found", name)) {
			return &Histogram{Name: name}, nil
		}
		return nil, err
	}
	if err = h.validate(); err != nil {
		return nil, errors.Wrapf(err, "bad histogram %v", h)
	}
	return &h, nil
}

// WaitForHistogram is a convenience function that calls GetHistogram until the requested histogram is available,
// ctx's deadline is reached, or timeout (if positive) has elapsed.
func WaitForHistogram(ctx context.Context, cr *chrome.Chrome, name string, timeout time.Duration) (*Histogram, error) {
	var h *Histogram
	err := testing.Poll(ctx, func(ctx context.Context) error {
		var err error
		h, err = GetHistogram(ctx, cr, name)
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
func WaitForHistogramUpdate(ctx context.Context, cr *chrome.Chrome, name string,
	old *Histogram, timeout time.Duration) (*Histogram, error) {
	var h *Histogram
	err := testing.Poll(ctx, func(ctx context.Context) error {
		var err error
		if h, err = GetHistogram(ctx, cr, name); err != nil {
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
func GetHistograms(ctx context.Context, cr *chrome.Chrome, histogramNames []string) ([]*Histogram, error) {
	var result []*Histogram
	for _, name := range histogramNames {
		histogram, err := GetHistogram(ctx, cr, name)
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

// Run is a helper to calculate histogram diffs before and after running a given
// function.
func Run(ctx context.Context, cr *chrome.Chrome, f func() error, names ...string) ([]*Histogram, error) {
	if len(names) == 0 {
		return nil, errors.New("no histogram names given")
	}

	before, err := GetHistograms(ctx, cr, names)
	if err != nil {
		return nil, err
	}

	if err := f(); err != nil {
		return nil, err
	}

	after, err := GetHistograms(ctx, cr, names)
	if err != nil {
		return nil, err
	}

	return DiffHistograms(before, after)
}
