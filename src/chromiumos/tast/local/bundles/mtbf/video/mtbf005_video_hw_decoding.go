// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package video

import (
	"context"
	"time"

	"chromiumos/tast/common/mtbferrors"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/metrics"
	"chromiumos/tast/local/input"
	mtbfchrome "chromiumos/tast/local/mtbf/chrome"
	"chromiumos/tast/testing"
)

const videoHistName = "Media.GpuVideoDecoderInitializeStatus"

func init() {
	testing.AddTest(&testing.Test{
		Func:         MTBF005VideoHWDecoding,
		Desc:         "HWDecodingResolutionChanging(MTBF005): Check what codecs are supported on your device under test at go/croscodecmatrix, includes H264, VP8, VP9",
		Contacts:     []string{"xliu@cienet.com"},
		SoftwareDeps: []string{"chrome", "chrome_internal"},
		Attr:         []string{"group:mainline", "informational"},
		Pre:          chrome.LoginReuse(),
		Params: []testing.Param{{
			Name: "h264",
			Val:  "http://commondatastorage.googleapis.com/chromiumos-test-assets-public/MSE/foo.html?switch_1080p_720p.mp4",
		}, {
			Name: "vp8",
			Val:  "http://commondatastorage.googleapis.com/chromiumos-test-assets-public/MSE/foo.html?switch_1080p_720p.webm",
		}},
	})
}

// getHistogramCount retrieves Histogram Count for Bucket 0.
func getHistogramCount(ctx context.Context, cr *chrome.Chrome, name string) (*int, error) {
	v := 0
	h, err := metrics.GetHistogram(ctx, cr, name)
	if err != nil {
		return nil, mtbferrors.New(mtbferrors.VideoNoHist, err, name)
	}

	if len((*h).Buckets) <= 0 {
		return nil, mtbferrors.New(mtbferrors.VideoZeroBucket, nil, name)
	}

	v = int((*h).Buckets[0].Count)
	return &v, nil
}

// MTBF005VideoHWDecoding case checks H264, VP8, and VP9 codecs are supported.
// Log out and back in to ensure clean histogram state. Go to the video link for the chosen codec.
// Check histogram (reload to ensure data is current). After 4-5 seconds, reload the histogram page.
func MTBF005VideoHWDecoding(ctx context.Context, s *testing.State) {
	cr := s.PreValue().(*chrome.Chrome)
	url := s.Param().(string)

	s.Log("Access chrome://histogram/Media.G URL")
	histURL := "chrome://histograms/Media.G"

	connHist, mtbferr := mtbfchrome.NewConn(ctx, cr, histURL)
	if mtbferr != nil {
		s.Fatal(mtbferr)
	}
	defer connHist.Close()
	defer connHist.CloseTarget(ctx)

	cntBefore, mtbferr := getHistogramCount(ctx, cr, videoHistName)
	if mtbferr != nil {
		s.Fatal(mtbferr)
	}

	s.Log("Link to video URL")
	connVideo, mtbferr := mtbfchrome.NewConn(ctx, cr, url)
	if mtbferr != nil {
		s.Fatal(mtbferr)
	}
	defer connVideo.Close()
	defer connVideo.CloseTarget(ctx)

	s.Log("Reload histogram page")
	if err := connHist.Eval(ctx, "window.location.reload()", nil); err != nil {
		s.Fatal(mtbferrors.New(mtbferrors.ChromeExeJs, err, "window.location.reload()"))
	}

	s.Log("Wait 4 ~ 5 seconds")
	testing.Sleep(ctx, time.Second*5)

	s.Log("Reload histogram page again")
	if err := connHist.Eval(ctx, "window.location.reload()", nil); err != nil {
		s.Fatal(mtbferrors.New(mtbferrors.ChromeExeJs, err, "window.location.reload()"))
	}

	cntAfter, mtbferr := getHistogramCount(ctx, cr, videoHistName)
	if mtbferr != nil {
		s.Fatal(mtbferr)
	}

	if *cntAfter <= *cntBefore {
		s.Fatal(mtbferrors.New(mtbferrors.VideoHistNotGrow, nil, videoHistName, *cntBefore, *cntAfter))
	}

	kb, err := input.Keyboard(ctx)
	if err != nil {
		s.Fatal(mtbferrors.New(mtbferrors.ChromeGetKeyboard, err))
	}
	defer kb.Close()

	if err := kb.Accel(ctx, "Ctrl+1"); err != nil {
		s.Fatal(mtbferrors.New(mtbferrors.ChromeKeyPress, err, "Ctrl+1"))
	}

	testing.Sleep(ctx, time.Second*2)
}
