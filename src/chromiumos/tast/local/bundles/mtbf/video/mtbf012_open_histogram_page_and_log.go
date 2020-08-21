// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package video

import (
	"context"

	"chromiumos/tast/common/mtbferrors"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/metrics"
	mtbfchrome "chromiumos/tast/local/mtbf/chrome"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         MTBF012OpenHistogramPageAndLog,
		Desc:         "Open histogram page and log data",
		Contacts:     []string{"xliu@cienet.com"},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"chrome", "chrome_internal"},
		Pre:          chrome.LoginReuse(),
	})
}

func MTBF012OpenHistogramPageAndLog(ctx context.Context, s *testing.State) {
	const (
		histogramName = "Media.GpuArcVideoDecodeAccelerator.InitializeResult"
		histogramURL  = "chrome://histograms/" + histogramName
	)
	cr := s.PreValue().(*chrome.Chrome)

	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal(mtbferrors.New(mtbferrors.ChromeTestConn, err))
	}

	histogramConn, mtbferr := mtbfchrome.NewConn(ctx, cr, histogramURL)
	if mtbferr != nil {
		s.Fatal(mtbferr)
	}
	defer histogramConn.Close()
	defer histogramConn.CloseTarget(ctx)

	histogram, err := metrics.GetHistogram(ctx, tconn, histogramName)
	if err != nil {
		s.Fatal(mtbferrors.New(mtbferrors.VideoGetHist, err, histogramName))
	}
	var bucketCount int64
	if len(histogram.Buckets) != 0 {
		bucketCount = histogram.Buckets[0].Count
	}
	s.Logf("[%v] Histogram.Buckets[0].Count: %d", histogramName, bucketCount)
}
