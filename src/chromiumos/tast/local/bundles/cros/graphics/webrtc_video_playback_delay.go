// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package graphics

import (
	"context"
	"net/http"
	"net/http/httptest"
	"time"

	"chromiumos/tast/common/media/caps"
	"chromiumos/tast/common/perf"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/metrics"
	"chromiumos/tast/local/graphics"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         WebRTCVideoPlaybackDelay,
		LacrosStatus: testing.LacrosVariantUnknown,
		Desc:         "Runs a webrtc playback-only connection to get performance numbers",
		Contacts:     []string{"mcasas@chromium.org", "chromeos-gfx@google.com"},
		Attr:         []string{"group:crosbolt", "crosbolt_nightly"},
		SoftwareDeps: []string{"chrome", "no_qemu"},
		Params: []testing.Param{{
			Name:              "vp8",
			Val:               "VP8",
			ExtraSoftwareDeps: []string{caps.HWDecodeVP8},
		}, {
			Name:              "vp9",
			Val:               "VP9",
			ExtraSoftwareDeps: []string{caps.HWDecodeVP9},
		}, {
			Name:              "h264",
			Val:               "H264",
			ExtraSoftwareDeps: []string{caps.HWDecodeH264, "proprietary_codecs"},
		}},
		Data:    []string{"webrtc_video_display_perf_test.html", "third_party/munge_sdp.js"},
		Fixture: "gpuWatchDog",
	})
}

func WebRTCVideoPlaybackDelay(ctx context.Context, s *testing.State) {
	server := httptest.NewServer(http.FileServer(s.DataFileSystem()))
	defer server.Close()
	testURL := server.URL + "/" + "webrtc_video_display_perf_test.html"

	cr, err := chrome.New(ctx, chrome.ExtraArgs(
		"--autoplay-policy=no-user-gesture-required",
		"--disable-rtc-smoothness-algorithm",
		"--use-fake-device-for-media-stream=fps=60",
		"--use-fake-ui-for-media-stream",
	))
	if err != nil {
		s.Fatal("Failed to create Chrome: ", err)
	}
	defer cr.Close(ctx)

	conn, err := cr.NewConn(ctx, testURL)
	if err != nil {
		s.Fatalf("Failed to open %s: %v", testURL, err)
	}
	defer conn.Close()
	defer conn.CloseTarget(ctx)

	// We could consider removing the CPU frequency scaling and thermal throttling
	// to get more consistent results, but then we wouldn't be measuring on the
	// same conditions as a user might encounter.

	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to connect to test API: ", err)
	}

	const presentationsHistogramName = "Media.VideoFrameSubmitter"
	initPresentationHistogram, err := metrics.GetHistogram(ctx, tconn, presentationsHistogramName)
	if err != nil {
		s.Fatal("Failed to get initial histogram: ", err)
	}
	const decodeHistogramName = "Media.MojoVideoDecoder.Decode"
	initDecodeHistogram, err := metrics.GetHistogram(ctx, tconn, decodeHistogramName)
	if err != nil {
		s.Fatal("Failed to get initial histogram: ", err)
	}
	const platformDecodeHistogramName = "Media.PlatformVideoDecoding.Decode"
	initPlatformDecodeHistogramName, err := metrics.GetHistogram(ctx, tconn, platformDecodeHistogramName)
	if err != nil {
		s.Fatal("Failed to get initial histogram: ", err)
	}

	profile := s.Param().(string)
	if err := conn.Call(ctx, nil, `(profile) => new Promise(async (resolve, reject) => {
		  let pc1 = new RTCPeerConnection();
		  let pc2 = new RTCPeerConnection();

		  pc1.onicecandidate = e => pc2.addIceCandidate(e.candidate).catch(reject);
		  pc2.onicecandidate = e => pc1.addIceCandidate(e.candidate).catch(reject);
		  pc2.ontrack = e => {
		    let remoteVideo = document.getElementById('remoteVideo');
		    remoteVideo.srcObject = e.streams[0];
		    resolve();
		  };

		  try {
		    let stream = await navigator.mediaDevices.getUserMedia({
		      audio: false,
		      video: { width: 1920, height: 1080 }
		    });
		    stream.getTracks().forEach(track => pc1.addTrack(track, stream));
		    let offer1 = await pc1.createOffer();
		    if (profile) {
		      offer1.sdp = setSdpDefaultVideoCodec(offer1.sdp, profile, false, "");
		    }
		    await pc1.setLocalDescription(offer1);
		    await pc2.setRemoteDescription(pc1.localDescription);
		    let offer2 = await pc2.createAnswer();
		    await pc2.setLocalDescription(offer2);
		    await pc1.setRemoteDescription(pc2.localDescription);
		  } catch (e) {
		    reject(e);
		  }
		})`, profile); err != nil {
		s.Fatal("RTCPeerConnection establishment failed: ", err)
	}

	// There's no easy way to count the amount of frames played back by a <video>
	// element, so let the connection roll with a timeout. At the expected 30-60
	// fps, we need tens of seconds to accumulate a couple of hundred frames to
	// make the histograms significant.
	const playbackTime = 20 * time.Second
	if err := testing.Sleep(ctx, playbackTime); err != nil {
		s.Fatal("Error while waiting for playback delay perf collection: ", err)
	}

	perfValues := perf.NewValues()
	if err := graphics.UpdatePerfMetricFromHistogram(ctx, tconn, presentationsHistogramName, initPresentationHistogram, perfValues, "tast_graphics_webrtc_video_playback_delay"); err != nil {
		s.Fatal("Failed to calculate Presentation perf metric: ", err)
	}
	if err := graphics.UpdatePerfMetricFromHistogram(ctx, tconn, decodeHistogramName, initDecodeHistogram, perfValues, "tast_graphics_webrtc_video_decode_delay"); err != nil {
		s.Fatal("Failed to calculate Decode perf metric: ", err)
	}
	if err := graphics.UpdatePerfMetricFromHistogram(ctx, tconn, platformDecodeHistogramName, initPlatformDecodeHistogramName, perfValues, "tast_graphics_webrtc_platform_video_decode_delay"); err != nil {
		s.Fatal("Failed to calculate Platform Decode perf metric: ", err)
	}

	if err = perfValues.Save(s.OutDir()); err != nil {
		s.Error("Cannot save perf data: ", err)
	}
}
