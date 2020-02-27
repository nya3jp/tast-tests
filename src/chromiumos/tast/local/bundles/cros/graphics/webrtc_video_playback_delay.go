// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package graphics

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"time"

	"chromiumos/tast/errors"
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
		SoftwareDeps: []string{"chrome"},
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
			ExtraSoftwareDeps: []string{caps.HWDecodeH264, "chrome_internal"}, // "chrome_internal" is needed because H.264 is a proprietary codec.
		}},
		Data: []string{"webrtc_video_display_perf_test.html", "third_party/munge_sdp.js"},
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
	peerConnectionCode := fmt.Sprintf(`new Promise((resolve, reject) => {
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
			.then(offer => {
				const profile = %q;
				if (profile) {
					offer.sdp = setSdpDefaultVideoCodec(offer.sdp, profile, false, "");
				}
				pc1.setLocalDescription(offer)
			})
			.then(() => pc2.setRemoteDescription(pc1.localDescription))
			.then(() => pc2.createAnswer())
			.then(offer => pc2.setLocalDescription(offer))
			.then(() => pc1.setRemoteDescription(pc2.localDescription))
			.catch(reject);
		});`, profile)
	if err := conn.EvalPromise(ctx, peerConnectionCode, nil); err != nil {
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
	if err := updatePerfMetricFromHistogram(ctx, tconn, presentationsHistogramName, initPresentationHistogram, perfValues, "tast_graphics_webrtc_video_playback_delay"); err != nil {
		s.Fatal("Failed to calculate Presentation perf metric: ", err)
	}
	if err := updatePerfMetricFromHistogram(ctx, tconn, decodeHistogramName, initDecodeHistogram, perfValues, "tast_graphics_webrtc_video_decode_delay"); err != nil {
		s.Fatal("Failed to calculate Decode perf metric: ", err)
	}
	if err := updatePerfMetricFromHistogram(ctx, tconn, platformDecodeHistogramName, initPlatformDecodeHistogramName, perfValues, "tast_graphics_webrtc_platform_video_decode_delay"); err != nil {
		s.Fatal("Failed to calculate Platform Decode perf metric: ", err)
	}

	if err = perfValues.Save(s.OutDir()); err != nil {
		s.Error("Cannot save perf data: ", err)
	}
}

func updatePerfMetricFromHistogram(ctx context.Context, tconn *chrome.TestConn, histogramName string, initHistogram *metrics.Histogram, perfValues *perf.Values, metricName string) error {
	laterHistogram, err := metrics.GetHistogram(ctx, tconn, histogramName)
	if err != nil {
		return errors.Wrap(err, "failed to get later histogram: ")
	}
	histogramDiff, err := laterHistogram.Diff(initHistogram)
	if err != nil {
		return errors.Wrap(err, "failed diffing histograms: ")
	}
	// Some devices don't have hardware decode acceleration, so the histogram diff
	// will be empty, this is not an error condition.
	if len(histogramDiff.Buckets) > 0 {
		decodeMetric := perf.Metric{
			Name:      metricName,
			Unit:      "ms",
			Direction: perf.SmallerIsBetter,
			Multiple:  true,
		}

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
				perfValues.Append(decodeMetric, bucketMidpoint)
			}
			average += bucketMidpoint * float64(bucket.Count) / numHistogramSamples
		}
		testing.ContextLog(ctx, histogramName, ": histogram:", histogramDiff.String(), "; average: ", average)
	}
	return nil
}
