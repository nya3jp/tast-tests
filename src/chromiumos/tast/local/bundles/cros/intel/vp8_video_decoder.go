// Copyright 2022 The ChromiumOS Authors.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package intel

import (
	"context"
	"net/http"
	"net/http/httptest"
	"time"

	"chromiumos/tast/common/perf"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/media/devtools"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         VP8VideoDecoder,
		LacrosStatus: testing.LacrosVariantUnneeded,
		Desc:         "Verifies whether VP8 decoder is supported or not",
		Contacts:     []string{"ambalavanan.m.m@intel.com", "intel-chrome-system-automation-team@intel.com"},
		SoftwareDeps: []string{"chrome"},
		Attr:         []string{"group:mainline", "informational"},
		Data:         []string{"1080p_60fps_600frames.vp8.webm", "video.html", "playback.js"},
		Fixture:      "chromeLoggedIn",
		Timeout:      3 * time.Minute,
	})
}

func VP8VideoDecoder(ctx context.Context, s *testing.State) {
	cr := s.FixtValue().(*chrome.Chrome)

	srv := httptest.NewServer(http.FileServer(s.DataFileSystem()))
	defer srv.Close()
	url := srv.URL + "/video.html"
	conn, err := cr.NewConn(ctx, url)
	if err != nil {
		s.Fatal("Failed to load video.html: ", err)
	}
	defer conn.Close()

	videoFile := "1080p_60fps_600frames.vp8.webm"
	if err := conn.Call(ctx, nil, "playRepeatedly", videoFile, false, true); err != nil {
		s.Fatal("Failed to play video: ", err)
	}

	// Video Element in the page to play a video.
	videoElement := "document.getElementsByTagName('video')[0]"
	var decodedFrameCount, droppedFrameCount int64
	if err := conn.Eval(ctx, videoElement+".getVideoPlaybackQuality().totalVideoFrames", &decodedFrameCount); err != nil {
		s.Fatal("Failed to get number of decoded frames: ", err)
	}
	if err := conn.Eval(ctx, videoElement+".getVideoPlaybackQuality().droppedVideoFrames", &droppedFrameCount); err != nil {
		s.Fatal("Failed to get number of dropped frames: ", err)
	}

	var droppedFramePercent float64
	if decodedFrameCount != 0 {
		droppedFramePercent = 100.0 * float64(droppedFrameCount) / float64(decodedFrameCount)
	} else {
		s.Log("No decoded frames; setting dropped percent to 100")
		droppedFramePercent = 100.0
	}

	p := perf.NewValues()

	p.Set(perf.Metric{
		Name:      "dropped_frames",
		Unit:      "frames",
		Direction: perf.SmallerIsBetter,
	}, float64(droppedFrameCount))
	p.Set(perf.Metric{
		Name:      "dropped_frames_percent",
		Unit:      "percent",
		Direction: perf.SmallerIsBetter,
	}, droppedFramePercent)

	s.Logf("Dropped frames: %d (%f%%)", droppedFrameCount, droppedFramePercent)

	observer, err := conn.GetMediaPropertiesChangedObserver(ctx)
	if err != nil {
		s.Fatal("Failed to retrieve DevTools Media messages: ", err)
	}

	isPlatform, decoderName, err := devtools.GetVideoDecoder(ctx, observer, url)
	if err != nil {
		s.Fatal("Failed to parse Media DevTools: ", err)
	}

	wantDecoder := "VaapiVideoDecoder"
	if !isPlatform && decoderName != wantDecoder {
		s.Fatalf("Failed: Hardware decoding accelerator was expected with decoder name but wasn't used: got: %q, want: %q", decoderName, wantDecoder)
	}

	if err := conn.Eval(ctx, videoElement+".pause()", nil); err != nil {
		s.Fatal("Failed to stop video: ", err)
	}
}
