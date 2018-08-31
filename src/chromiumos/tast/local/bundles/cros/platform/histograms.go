// Copyright 2018 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package platform

import (
	"context"
	"strconv"
	"time"

	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/metrics"
	"chromiumos/tast/local/testexec"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         Histograms,
		Desc:         "Checks that libmetrics can report histograms to Chrome",
		SoftwareDeps: []string{"chrome_login"},
	})
}

func Histograms(s *testing.State) {
	const (
		name    = "Tast.TestHistogram"
		sample  = 100
		min     = 1
		max     = 200
		buckets = 10
	)

	ctx := s.Context()
	cr, err := chrome.New(ctx, chrome.ExtraArgs([]string{"--external-metrics-collection-interval=1"}))
	if err != nil {
		s.Fatal("Chrome login: ", err)
	}

	s.Logf("Reporting %v sample with value %v", name, sample)
	cmd := testexec.CommandContext(ctx, "metrics_client", name,
		strconv.Itoa(sample), strconv.Itoa(min), strconv.Itoa(max), strconv.Itoa(buckets))
	if err := cmd.Run(); err != nil {
		s.Fatal("Failed to report sample: ", err)
	}

	s.Logf("Waiting for %v histogram", name)
	rctx, rcancel := context.WithTimeout(ctx, 10*time.Second)
	defer rcancel()
	var h *metrics.Histogram
	for {
		if h, err = metrics.GetHistogram(ctx, cr, name); err == nil {
			break
		}
		select {
		case <-time.After(100 * time.Millisecond):
		case <-rctx.Done():
			s.Fatalf("Failed to get histogram: %v (%v)", rctx.Err(), err)
		}
	}
	s.Log("Got histogram: ", h)

	if h.TotalCount != 1 {
		s.Errorf("Total count is %v instead of 1", h.TotalCount)
	}
	if len(h.Buckets) != 1 {
		s.Errorf("Got %v buckets instead of 1", len(h.Buckets))
	} else {
		if h.Buckets[0].Min > sample || h.Buckets[0].Max < sample {
			s.Errorf("Bucket has range [%v, %v] but sample was %v", h.Buckets[0].Min, h.Buckets[0].Max, sample)
		}
		if h.Buckets[0].Count != 1 {
			s.Errorf("Bucket has count %v instead of 1", h.Buckets[0].Count)
		}
	}
}
