// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package graphics

import (
	"context"
	"net/http"
	"net/http/httptest"
	"time"

	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/metrics"
	"chromiumos/tast/local/media/caps"
	"chromiumos/tast/local/perf"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         WebRTCVideoPlaybackDelay,
		Desc:         "Runs a webrtc playback-only connection to get performance numbers",
		Contacts:     []string{"mcasas@chromium.org", "chromeos-gfx@google.com"},
		Attr:         []string{"group:crosbolt", "crosbolt_nightly"},
		SoftwareDeps: []string{"chrome", caps.HWDecodeVP8},
		Data:         []string{"webrtc_video_display_perf_test.html"},
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

	const presentationsHistogramName = "Media.VideoFrameSubmitter"
	initPresentationHistogram, err := metrics.GetHistogram(ctx, cr, presentationsHistogramName)
	if err != nil {
		s.Fatal("Failed to get initial histogram: ", err)
	}
	const decodeHistogramName = "Media.MojoVideoDecoder.Decode"
	initDecodeHistogram, err := metrics.GetHistogram(ctx, cr, decodeHistogramName)
	if err != nil {
		s.Fatal("Failed to get initial histogram: ", err)
	}

	const peerConnectionCode = `new Promise((resolve, reject) => {
			var pc1 = new RTCPeerConnection();
			var pc2 = new RTCPeerConnection();

			pc1.onicecandidate = e => pc2.addIceCandidate(e.candidate).catch(reject);
			pc2.onicecandidate = e => pc1.addIceCandidate(e.candidate).catch(reject);
			pc2.ontrack = e => {
				let remoteVideo = document.getElementById('remoteVideo');
				remoteVideo.srcObject = e.streams[0];
				resolve();
			};

			const offerOptions = {
				offerToReceiveAudio: 1,
				offerToReceiveVideo: 1
			};
			const constraints = {
				audio: false,
				video: {
					mandatory: {
						minWidth : 1920,
						maxWidth : 1920,
						minHeight : 1080,
						maxHeight : 1080
					}
				}
			};

			navigator.mediaDevices.getUserMedia(constraints)
			.then(stream => stream.getTracks().forEach(track => pc1.addTrack(track, stream)))
			.then(() => pc1.createOffer(offerOptions))
			.then(offer => pc1.setLocalDescription(offer))
			.then(() => pc2.setRemoteDescription(pc1.localDescription))
			.then(() => pc2.createAnswer())
			.then(offer => pc2.setLocalDescription(offer))
			.then(() => pc1.setRemoteDescription(pc2.localDescription))
			.catch(reject);
		});`
	if err := conn.EvalPromise(ctx, peerConnectionCode, nil); err != nil {
		s.Fatal("RTCPeerConnection establishment failed: ", err)
	}

	// There's no easy way to count the amount of frames played back by a <video>
	// element, so let the connection roll with a timeout. At the expected 30-60
	// fps, we need tens of seconds to accumulate a couple of hundred frames to
	// make the histograms significant.
	const playbackTimeInSeconds = 20 * time.Second
	if err := testing.Sleep(ctx, playbackTimeInSeconds); err != nil {
		s.Fatal("Error while waiting for playback delay perf collection: ", err)
	}

	laterPresentationHistogram, err := metrics.GetHistogram(ctx, cr, presentationsHistogramName)
	if err != nil {
		s.Fatal("Failed to get later histogram: ", err)
	}

	presentationHistogramDiff, err := laterPresentationHistogram.Diff(initPresentationHistogram)
	if err != nil {
		s.Fatal("Failed diffing histograms: ", err)
	}
	if len(presentationHistogramDiff.Buckets) == 0 {
		s.Fatal("Empty histogram diff")
	}

	perfValues := perf.NewValues()

	presentationMetric := perf.Metric{
		Name:      "tast_graphics_webrtc_video_playback_delay",
		Unit:      "ms",
		Direction: perf.SmallerIsBetter,
		Multiple:  true,
	}
	updatePerfMetricFromHistogram(ctx, presentationsHistogramName,
		presentationHistogramDiff, perfValues, presentationMetric)

	laterDecodeHistogram, err := metrics.GetHistogram(ctx, cr, decodeHistogramName)
	if err != nil {
		s.Fatal("Failed to get later histogram: ", err)
	}
	decodeHistogramDiff, err := laterDecodeHistogram.Diff(initDecodeHistogram)
	if err != nil {
		s.Fatal("Failed diffing histograms: ", err)
	}
	// Some devices don't have hardware decode acceleration, so the histogram diff
	// will be empty, this is not an error condition.
	if len(decodeHistogramDiff.Buckets) > 0 {
		decodeMetric := perf.Metric{
			Name:      "tast_graphics_webrtc_video_decode_delay",
			Unit:      "ms",
			Direction: perf.SmallerIsBetter,
			Multiple:  true,
		}
		updatePerfMetricFromHistogram(ctx, decodeHistogramName, decodeHistogramDiff,
			perfValues, decodeMetric)
	}

	if err = perfValues.Save(s.OutDir()); err != nil {
		s.Error("Cannot save perf data: ", err)
	}
}

func updatePerfMetricFromHistogram(ctx context.Context,
	histogramName string,
	histogramDiff *metrics.Histogram,
	perfValues *perf.Values,
	metric perf.Metric) {
	numHistogramSamples := float64(histogramDiff.TotalCount())
	var average float64
	// Walk the buckets of histogramDiff, append the central value of the
	// histogram bucket as many times as bucket entries to perfValues, and
	// calculate the average on the fly for debug printout purposes. This average
	// is a discrete approximation to the statistical average of the samples
	// underlying the histogramDiff histograms.
	for _, bucket := range histogramDiff.Buckets {
		bucketMidpoint := float64(bucket.Max+bucket.Min) / 2.0
		for i := 0; i < int(bucket.Count); i++ {
			perfValues.Append(metric, bucketMidpoint)
		}
		average += bucketMidpoint * float64(bucket.Count) / numHistogramSamples
	}
	testing.ContextLog(ctx, histogramName, ": histogram:", histogramDiff.String(), "; average: ", average)
}
