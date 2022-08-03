// Copyright 2022 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package histogramutil provides utilities to record and verify Chrome histograms.
package histogramutil

import (
	"context"
	"time"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/metrics"
	"chromiumos/tast/testing"
)

// HistogramVerifier is a function that verifies the histogram values.
type HistogramVerifier func(m *metrics.Histogram) error

// HistogramTests contains the names of the tracked histograms and their associated verifiers.
type HistogramTests map[string]HistogramVerifier

// names returns the names of the histograms tracked by |ht| as string slice.
func (ht HistogramTests) names() []string {
	names := make([]string, len(ht))
	i := 0
	for v := range ht {
		names[i] = v
		i++
	}
	return names
}

// Record starts recording changes of the histograms tracked by |ht|.
func (ht HistogramTests) Record(ctx context.Context, tconn *chrome.TestConn) (*metrics.Recorder, error) {
	metrics.ClearHistogramTransferFile()
	return metrics.StartRecorder(ctx, tconn, ht.names()...)
}

// wait waits until the histograms tracked by |ht| have new values.
func (ht HistogramTests) wait(ctx context.Context, tconn *chrome.TestConn, recorder *metrics.Recorder) ([]*metrics.Histogram, error) {
	// It takes time for Chrome to refresh the histograms. Leave 3 seconds
	// for the clean-up tasks in case of error.
	ctxDeadline, _ := ctx.Deadline()
	histTimeout := ctxDeadline.Sub(time.Now()) - 3*time.Second
	return recorder.WaitAll(ctx, tconn, histTimeout)
}

// Verify waits until the histograms tracked by |ht| are ready and verifies the
// histogram values recorded using the HistogramVerifier associated with each
// histogram.
func (ht HistogramTests) Verify(ctx context.Context, tconn *chrome.TestConn, recorder *metrics.Recorder) error {
	histograms, err := ht.wait(ctx, tconn, recorder)
	if err != nil {
		return errors.Wrap(err, "failed to get updated histograms")
	}

	for _, h := range histograms {
		if err := ht[h.Name](h); err != nil {
			// Dump the whole histogram diff so that we get a better
			// idea of what else also failed.
			testing.ContextLog(ctx, "Histogram diff: ", histograms)
			return err
		}
	}

	return nil
}

// AssertHistogramEq returns a HistogramVerifier that can be used to check if a
// histogram's value equals to |value|.
func AssertHistogramEq(value float64) HistogramVerifier {
	return func(m *metrics.Histogram) error {
		// We assume that there's only one sample hence we can check
		// against the histogram mean.
		if len(m.Buckets) != 1 {
			return errors.Errorf("invalid %s: %v", m.Name, m.Buckets)
		}
		if mean, err := m.Mean(); err != nil {
			return errors.Wrap(err, "failed to get histogram mean")
		} else if mean != value {
			return errors.Errorf("unexpected value of %s: %v does not equal to %v", m.Name, mean, value)
		}
		return nil
	}
}

// AssertHistogramIn returns a HistogramVerifier that can be used to check if a
// histogram's value is in |values|.
func AssertHistogramIn(values ...float64) HistogramVerifier {
	return func(m *metrics.Histogram) error {
		// We assume that there's only one sample hence we can check
		// against the histogram mean.
		if len(m.Buckets) != 1 {
			return errors.Errorf("invalid %s: %v", m.Name, m.Buckets)
		}
		mean, err := m.Mean()
		if err != nil {
			return errors.Wrap(err, "failed to get histogram mean")
		}
		for _, v := range values {
			if v == mean {
				return nil
			}
		}
		return errors.Errorf("unexpected value of %s: %v is not one of %v", m.Name, mean, values)
	}
}

// AssertHistogramMeanGt returns a HistogramVerifier that can be used to check
// if a histogram's mean value is greater than |value|.
func AssertHistogramMeanGt(value float64) HistogramVerifier {
	return func(m *metrics.Histogram) error {
		if len(m.Buckets) == 0 {
			return errors.Errorf("invalid %s: %v", m.Name, m.Buckets)
		}
		if mean, err := m.Mean(); err != nil {
			return errors.Wrap(err, "failed to get histogram mean")
		} else if mean <= value {
			return errors.Errorf("unexpected mean of %s: %v is not greater than %v", m.Name, mean, value)
		}
		return nil
	}
}

// AssertHistogramInRange returns a HistogramVerifier that can be used to check
// if a histogram's mean value is in the range [minValue, maxValue].
func AssertHistogramInRange(minValue, maxValue float64) HistogramVerifier {
	return func(m *metrics.Histogram) error {
		if len(m.Buckets) == 0 {
			return errors.Errorf("invalid %s: %v", m.Name, m.Buckets)
		}
		if mean, err := m.Mean(); err != nil {
			return errors.Wrap(err, "failed to get histogram mean")
		} else if mean < minValue || mean > maxValue {
			return errors.Errorf("unexpected mean of %s: %v is not in [%v, %v]", m.Name, mean, minValue, maxValue)
		}
		return nil
	}
}
