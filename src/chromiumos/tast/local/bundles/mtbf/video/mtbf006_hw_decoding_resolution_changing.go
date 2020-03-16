// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package video

import (
	"context"
	"fmt"
	"time"

	"chromiumos/tast/common/mtbferrors"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/metrics"
	mtbfchrome "chromiumos/tast/local/mtbf/chrome"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         MTBF006HWDecodingResolutionChanging,
		Desc:         "Check what codecs are supported on your device under test at go/croscodecmatrix. There are two codecs H264 and VP8",
		Contacts:     []string{"xliu@cienet.com"},
		SoftwareDeps: []string{"chrome", "chrome_internal"},
		Attr:         []string{"group:mainline", "informational"},
		Pre:          chrome.LoginReuse(),
		Params: []testing.Param{{
			Name: "h264",
			Val:  "https://storage.googleapis.com/chromiumos-test-assets-public/Shaka-Dash/switch_1080p_720p.mp4",
		}, {
			Name: "vp8",
			Val:  "http://storage.googleapis.com/chromiumos-test-assets-public/Shaka-Dash/switch_1080p_720p.webm",
		}},
	})
}

func getHistogram(ctx context.Context, s *testing.State, cr *chrome.Chrome, histogramName string) *metrics.Histogram {
	histogram, err := metrics.GetHistogram(ctx, cr, histogramName)
	if err != nil {
		s.Fatal(mtbferrors.New(mtbferrors.VideoGetHist, err, histogramName))
	}

	return histogram
}

func getFirstBucketCountFrom(histogram *metrics.Histogram) int64 {
	if len(histogram.Buckets) != 0 {
		return histogram.Buckets[0].Count
	}
	return 0
}

func MTBF006HWDecodingResolutionChanging(ctx context.Context, s *testing.State) {
	const (
		histogramName = "Media.GpuVideoDecoderInitializeStatus"
		histogramURL  = "chrome://histograms/" + histogramName
	)
	testing.Sleep(ctx, 5*time.Second) // Sleep to make sure last login stable.
	cr := s.PreValue().(*chrome.Chrome)

	histogramConn, mtbferr := mtbfchrome.NewConn(ctx, cr, histogramURL)
	if mtbferr != nil {
		s.Fatal(mtbferr)
	}
	defer histogramConn.Close()
	defer histogramConn.CloseTarget(ctx)

	histogramConn.Eval(ctx, "window.location.reload()", nil)
	histogram := getHistogram(ctx, s, cr, histogramName)
	firstBucketCount := getFirstBucketCountFrom(histogram)
	s.Logf("[%v] Histogram.Buckets[0].Count: %d", histogramName, firstBucketCount)

	url := s.Param().(string)
	conn, mtbferr := mtbfchrome.NewConn(ctx, cr, url)
	if mtbferr != nil {
		s.Fatal(mtbferr)
	}
	defer conn.Close()
	testing.Sleep(ctx, 5*time.Second)

	histogramConn.Eval(ctx, "window.location.reload()", nil)
	histogram = getHistogram(ctx, s, cr, histogramName)
	if count := getFirstBucketCountFrom(histogram); count != (firstBucketCount + 1) {
		s.Fatal(mtbferrors.New(mtbferrors.VideoHist, mtbferr,
			fmt.Sprintf("first bucket count should be increased by 1: %d -> %d(%d)",
				firstBucketCount, count, firstBucketCount+1)))
	} else {
		s.Logf("[%v] Histogram.Buckets[0].Count: %d", histogramName, count)
	}
	testing.Sleep(ctx, 5*time.Second)
}
