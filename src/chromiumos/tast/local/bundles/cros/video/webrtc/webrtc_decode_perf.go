// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Provides code for video.WebRTCDecodePerf* tests.

package webrtc

import (
	"context"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"sort"
	"time"

	"chromiumos/tast/ctxutil"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/bundles/cros/video/lib/constants"
	"chromiumos/tast/local/bundles/cros/video/lib/histogram"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/metrics"
	"chromiumos/tast/local/perf"
	"chromiumos/tast/testing"
)

// openWebRTCInternalsPage opens WebRTC internals page and replaces JS
// addStats() to intercept WebRTC performance metrics, "googMaxDecodeMs"
// and "googDecodeMs".
func openWebRTCInternalsPage(ctx context.Context, cr *chrome.Chrome, addStatsJS string) (*chrome.Conn, error) {
	const url = "chrome://webrtc-internals"
	conn, err := cr.NewConn(ctx, url)
	if err != nil {
		return nil, errors.Wrap(err, "failed to open "+url)
	}
	err = conn.WaitForExpr(ctx, "document.readyState === 'complete'")
	if err != nil {
		conn.Close()
		return nil, err
	}
	if err = conn.Exec(ctx, addStatsJS); err != nil {
		conn.Close()
		return nil, err
	}
	return conn, nil
}

// getMedian returns the median of the given positive duration.
// If the number of inputs is even, it returns the average of the middle two values.
// If input is empty, returns 0.
func getMedian(s []time.Duration) time.Duration {
	size := len(s)
	if size == 0 {
		return time.Duration(0)
	}
	ss := make([]time.Duration, size)
	copy(ss, s)
	sort.Slice(ss, func(i, j int) bool { return ss[i] < ss[j] })
	if size%2 != 0 {
		return ss[size/2]
	}
	return (ss[size/2] + ss[size/2-1]) / 2
}

// getMax returns the maximum of the given positive duration.
// If input is empty, returns 0.
func getMax(s []time.Duration) time.Duration {
	var max time.Duration
	for _, n := range s {
		if n > max {
			max = n
		}
	}
	return max
}

// measureDecodeTime returns largest observed frames' decode time and median of decode time samples.
// The decode time samples are obtained from chrome://webrtc-internals page.
func measureDecodeTime(ctx context.Context, webRTCInternals *chrome.Conn) (max, median time.Duration, err error) {
	const (
		measureTimeout  = time.Minute
		measureInterval = 500 * time.Millisecond
		numSamples      = 10
	)

	// Current frame's decode time.
	var decodeTimes []time.Duration
	// Maximum observed frame decode time.
	var maxDecodeTimes []time.Duration

	err = testing.Poll(ctx,
		func(ctx context.Context) error {
			var maxTimesMs []int
			if err := webRTCInternals.Eval(ctx, "googMaxDecodeMs", &maxTimesMs); err != nil {
				return testing.PollBreak(errors.Wrap(err, "unable to eval googMaxDecodeMs"))

			}
			if len(maxTimesMs) < numSamples {
				return errors.New("insufficient samples")
			}
			maxDecodeTimes = make([]time.Duration, len(maxTimesMs))
			for i, ms := range maxTimesMs {
				maxDecodeTimes[i] = time.Duration(ms) * time.Millisecond
			}

			var timesMs []int
			if err := webRTCInternals.Eval(ctx, "googDecodeMs", &timesMs); err != nil {
				return testing.PollBreak(errors.Wrap(err, "unable to eval googDecodeMs"))
			}
			if len(timesMs) < numSamples {
				return errors.New("insufficient samples")
			}
			decodeTimes = make([]time.Duration, len(timesMs))
			for i, ms := range timesMs {
				decodeTimes[i] = time.Duration(ms) * time.Millisecond
			}
			return nil
		}, &testing.PollOptions{Interval: measureInterval, Timeout: measureTimeout})
	if err != nil {
		return
	}
	if len(maxDecodeTimes) < numSamples {
		return max, median, errors.Errorf("got %d max decode time sample(s); want %d", len(maxDecodeTimes), numSamples)
	}
	if len(decodeTimes) < numSamples {
		return max, median, errors.Errorf("got %d decode time sample(s); want %d", len(decodeTimes), numSamples)
	}
	max = getMax(maxDecodeTimes)
	median = getMedian(decodeTimes)
	testing.ContextLog(ctx, "Max decode times: ", maxDecodeTimes)
	testing.ContextLog(ctx, "Decode times: ", decodeTimes)
	testing.ContextLogf(ctx, "Largest max is %v, median is %v", max, median)
	return
}

// webRTCPerf starts a Chrome instance (with or without hardware video decoder),
// opens an WebRTC loopback page that repeatly loopbacks a camera stream
// to measure decode time by looking at chrome://webrtc-internals page,
// and stores to perf.Values struct.
// webRTCPerf returns true if video decode is hardware accelerated; otherwise, returns false.
func webRTCPerf(ctx context.Context, s *testing.State, streamFile, loopbackURL, addStatsJS string,
	disableHWAccel bool, p *perf.Values) (hwAccelUsed bool) {
	chromeArgs := chromeArgsWithCameraInput(streamFile)
	if disableHWAccel {
		chromeArgs = append(chromeArgs, "--disable-accelerated-video-decode")
	}
	cr, err := chrome.New(ctx, chrome.ExtraArgs(chromeArgs...))
	if err != nil {
		s.Fatal("Failed to create Chrome: ", err)
	}
	defer cr.Close(ctx)

	shortCtx, cancel := ctxutil.Shorten(ctx, 10*time.Second)
	defer cancel()

	rtcInitHistogram, err := metrics.GetHistogram(shortCtx, cr, constants.RTCVDInitStatus)
	if err != nil {
		s.Fatalf("Failed to get histogram %s: %v", constants.RTCVDInitStatus, err)
	}

	conn, err := cr.NewConn(shortCtx, loopbackURL)
	if err != nil {
		s.Fatalf("Failed to open %s: %v", loopbackURL, err)
	}
	defer conn.Close()
	// Close the tab to stop loopback after test.
	defer conn.CloseTarget(shortCtx)

	if err := conn.WaitForExpr(shortCtx, "streamReady"); err != nil {
		s.Fatal("Timed out waiting for stream ready: ", err)
	}
	if err := checkError(shortCtx, conn); err != nil {
		s.Fatal("Error sanity check loopback web page: ", err)
	}

	webRTCInternals, err := openWebRTCInternalsPage(shortCtx, cr, addStatsJS)
	if err != nil {
		s.Fatal("Failed to open WebRTC-internals page: ", err)
	}
	defer webRTCInternals.Close()
	defer webRTCInternals.CloseTarget(shortCtx)

	max, median, err := measureDecodeTime(shortCtx, webRTCInternals)
	if err != nil {
		s.Fatal("Failed to measure decode time: ", err)
	}

	hwAccelUsed, err = histogram.WasHWAccelUsed(shortCtx, cr, rtcInitHistogram, constants.RTCVDInitStatus, int64(constants.RTCVDInitSuccess))
	s.Log("Use hardware video decoder? ", hwAccelUsed)
	if disableHWAccel && hwAccelUsed {
		s.Fatal("Hardware video decoder unexpectedly used")
	}

	hwPrefix := "sw_"
	if hwAccelUsed {
		hwPrefix = "hw_"
	}

	// TODO(crbug.com/918362): Remove "tast_" prefix after removing video_WebRtcPerf in autotest.
	const tastPrefix = "tast_"
	p.Set(perf.Metric{
		Name:      tastPrefix + hwPrefix + "decode_time.max",
		Unit:      "milliseconds",
		Direction: perf.SmallerIsBetter},
		float64(max)/float64(time.Millisecond))
	p.Set(perf.Metric{
		Name:      tastPrefix + hwPrefix + "decode_time.percentile_0.50",
		Unit:      "milliseconds",
		Direction: perf.SmallerIsBetter},
		float64(median)/float64(time.Millisecond))

	return hwAccelUsed
}

// RunWebRTCDecodePerf starts a Chrome instance (with or without hardware video decoder),
// opens an WebRTC loopback page that repeatly loopbacks a camera stream
// to measure decode time by looking at chrome://webrtc-internals page,
// and stores decode time measurements to perf.
func RunWebRTCDecodePerf(ctx context.Context, s *testing.State, streamName string) {
	server := httptest.NewServer(http.FileServer(s.DataFileSystem()))
	defer server.Close()
	loopbackURL := server.URL + "/" + LoopbackPage

	b, err := ioutil.ReadFile(s.DataPath(AddStatsJSFile))
	if err != nil {
		s.Fatal("Failed to read JS for gathering decode time: ", err)
	}
	addStatsJS := string(b)

	p := perf.NewValues()
	// Try hardware accelerated WebRTC first.
	// If it is hardware accelerated, run without hardware acceleration again.
	streamFilePath := s.DataPath(streamName)
	hwAccelUsed := webRTCPerf(ctx, s, streamFilePath, loopbackURL, addStatsJS, false, p)
	if hwAccelUsed {
		webRTCPerf(ctx, s, streamFilePath, loopbackURL, addStatsJS, true, p)
	}
	p.Save(s.OutDir())
}
