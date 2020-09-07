// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package webrtc

import (
	"context"
	"time"

	"golang.org/x/sync/errgroup"

	"chromiumos/tast/local/bundles/cros/webrtc/mediarecorder"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/metrics"
	"chromiumos/tast/local/media/caps"
	"chromiumos/tast/local/media/constants"
	"chromiumos/tast/local/media/pre"
	"chromiumos/tast/local/media/videotype"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func: MediaRecorderMulti,
		Desc: "Verifies that MediaRecorder can use multiple encoders in parallel",
		Contacts: []string{
			"mcasas@chromium.org",
			"chromeos-gfx-video@google.com",
		},
		SoftwareDeps: []string{"chrome"},
		Data:         []string{"loopback_media_recorder.html"},
		Attr:         []string{"group:graphics", "graphics_video", "graphics_perbuild"},
		Params: []testing.Param{{
			Name:              "vp8_vp8",
			Val:               []videotype.Codec{videotype.VP8, videotype.VP8},
			ExtraSoftwareDeps: []string{caps.HWEncodeVP8},
			Pre:               pre.ChromeVideoWithFakeWebcam(),
		}, {
			Name:              "vp8_h264",
			Val:               []videotype.Codec{videotype.H264, videotype.VP8},
			ExtraSoftwareDeps: []string{caps.HWEncodeH264, caps.HWEncodeVP8, "proprietary_codecs"},
			Pre:               pre.ChromeVideoWithFakeWebcam(),
		}, {
			Name:              "h264_h264",
			Val:               []videotype.Codec{videotype.H264, videotype.H264},
			ExtraSoftwareDeps: []string{caps.HWEncodeH264, "proprietary_codecs"},
			Pre:               pre.ChromeVideoWithFakeWebcam(),
		}},
	})
}

// MediaRecorderMulti verifies that multiple video encoders can be used
// in parallel.
func MediaRecorderMulti(ctx context.Context, s *testing.State) {
	const (
		// Record for several seconds to ensure the encodings are overlapping.
		// Also, any interaction between the two encoders may take several
		// seconds to occur.
		recordDuration = 10 * time.Second
	)

	cr := s.PreValue().(*chrome.Chrome)
	codecs := s.Param().([]videotype.Codec)

	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to connect to test API: ", err)
	}

	initHistogram, err := metrics.GetHistogram(ctx, tconn, constants.MediaRecorderVEAUsed)
	if err != nil {
		s.Fatal("Failed to get initial histogram: ", err)
	}

	g, encCtx := errgroup.WithContext(ctx)
	for _, codec := range codecs {
		// https://golang.org/doc/faq#closures_and_goroutines
		c := codec
		g.Go(func() error {
			s.Log("Running codec ", c)
			return mediarecorder.VerifyMediaRecorderUsesEncodeAccelerator(
				encCtx, cr, s.DataFileSystem(), c, recordDuration)
		})
	}

	if err := g.Wait(); err != nil {
		s.Error("Failed to run VerifyMediaRecorderUsesEncodeAccelerator: ", err)
	}

	// Ensure that at least len(codecs) HW encoders have been used.
	// This condition is "at least" because success count has been observed to be
	// higher than expected (crbug.com/985068).
	histogramDiff, err := metrics.WaitForHistogramUpdate(ctx, tconn, constants.MediaRecorderVEAUsed, initHistogram, time.Second)
	if err != nil {
		s.Fatal("Failed to wait for histogram update: ", err)
	}
	if len(histogramDiff.Buckets) > 1 {
		s.Error("Unexpected histogram update: ", histogramDiff)
	} else if len(histogramDiff.Buckets) == 0 {
		s.Fatal("No histogram update observed")
	}
	diff := histogramDiff.Buckets[0]
	if diff.Max != constants.MediaRecorderVEAUsedSuccess+1 {
		s.Error("Unexpected bucket range: ", diff)
	} else if diff.Count < int64(len(codecs)) {
		s.Errorf("Insufficient histogram updates, got: %d, want: %d",
			diff.Count, len(codecs))
	}
}
