// Copyright 2018 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package metrics

import (
	"context"
	"fmt"
	"strings"

	"chromiumos/tast/local/chrome"
)

// Histogram contains data from a Chrome histogram.
type Histogram struct {
	// Buckets contains ranges of reported values.
	Buckets []HistogramBucket `json:"buckets"`
}

// TotalCount returns the total number of samples stored across all buckets.
func (h *Histogram) TotalCount() int64 {
	var t int64
	for _, b := range h.Buckets {
		t += b.Count
	}
	return t
}

func (h *Histogram) String() string {
	var strs []string
	for _, b := range h.Buckets {
		strs = append(strs, fmt.Sprintf("[%d,%d):%d", b.Min, b.Max, b.Count))
	}
	return "[" + strings.Join(strs, " ") + "]"
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
// An error is returned if no samples have been reported for the histogram since Chrome was started.
func GetHistogram(ctx context.Context, cr *chrome.Chrome, name string) (*Histogram, error) {
	conn, err := cr.TestAPIConn(ctx)
	if err != nil {
		return nil, err
	}

	h := Histogram{}
	expr := fmt.Sprintf(
		`new Promise(function(resolve, reject) {
			chrome.autotestPrivate.getHistogram(%q, function(h) {
				if (chrome.runtime.lastError == undefined) {
					resolve(h);
				} else {
				    reject(chrome.runtime.lastError.message);
				}
			});
		})`, name)
	if err := conn.EvalPromise(ctx, expr, &h); err != nil {
		return nil, err
	}
	return &h, nil
}
