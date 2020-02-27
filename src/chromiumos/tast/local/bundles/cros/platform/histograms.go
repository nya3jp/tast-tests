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
		Desc:         "Checks that libmetrics reports histograms to Chrome",
		Contacts:     []string{"chromeos-systems@google.com"},
		SoftwareDeps: []string{"chrome"},
		Attr:         []string{"group:mainline"},
	})
}

func Histograms(ctx context.Context, s *testing.State) {
	const (
		name    = "Tast.TestHistogram"
		sample1 = 1
		sample2 = 2
		max     = 5
		timeout = 10 * time.Second
	)

	// report reports a linear histogram sample of val.
	report := func(val int) {
		s.Logf("Reporting %v sample with value %v", name, val)
		cmd := testexec.CommandContext(ctx, "metrics_client", name, "-e", strconv.Itoa(val), strconv.Itoa(max))
		if err := cmd.Run(); err != nil {
			defer cmd.DumpLog(ctx)
			s.Fatal("Failed to report sample: ", err)
		}
	}

	// check verifies that h contains a single bucket over range [val,val+1) containing a single sample.
	check := func(h *metrics.Histogram, val int) {
		if len(h.Buckets) != 1 {
			s.Fatalf("Got %v buckets instead of 1", len(h.Buckets))
		}
		if h.Buckets[0].Min != int64(val) || h.Buckets[0].Max != int64(val)+1 {
			s.Errorf("Bucket has range [%v, %v) but sample was %v", h.Buckets[0].Min, h.Buckets[0].Max, val)
		}
		if h.Buckets[0].Count != 1 {
			s.Errorf("Bucket has count %v instead of 1", h.Buckets[0].Count)
		} else if tc := h.TotalCount(); tc != 1 {
			s.Errorf("Histogram has total count %v instead of 1", tc)
		}
	}

	// Configure Chrome to check for new external histograms every second.
	cr, err := chrome.New(ctx, chrome.ExtraArgs("--external-metrics-collection-interval=1"))
	if err != nil {
		s.Fatal("Chrome login: ", err)
	}

	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to connect to test API: ", err)
	}

	report(sample1)
	s.Logf("Waiting for %v histogram", name)
	h1, err := metrics.WaitForHistogram(ctx, tconn, name, timeout)
	if err != nil {
		s.Fatal("Failed to get histogram: ", err)
	}
	s.Log("Got histogram: ", h1)
	check(h1, sample1)

	report(sample2)
	s.Logf("Waiting for %v histogram update", name)
	h2, err := metrics.WaitForHistogramUpdate(ctx, tconn, name, h1, timeout)
	if err != nil {
		s.Fatal("Failed to get histogram update: ", err)
	}
	s.Log("Got histogram update: ", h2)
	check(h2, sample2)
}
