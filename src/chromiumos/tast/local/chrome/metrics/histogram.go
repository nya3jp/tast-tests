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

type Histogram struct {
	Buckets    []HistogramBucket `json:"buckets"`
	TotalCount int64             `json:"totalCount"`
}

func (h *Histogram) String() string {
	strs := []string{fmt.Sprintf("total:%d", h.TotalCount)}
	for _, b := range h.Buckets {
		strs = append(strs, fmt.Sprintf("%d-%d:%d", b.Min, b.Max, b.Count))
	}
	return "[" + strings.Join(strs, " ") + "]"
}

type HistogramBucket struct {
	Min   int64 `json:"min"`
	Max   int64 `json:"max"`
	Count int64 `json:"count"`
}

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
