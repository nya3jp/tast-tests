// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package webrtc

import (
	"context"
	"fmt"
	"math"
	"net/http"
	"net/http/httptest"
	"sort"
	"time"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/bundles/cros/video/lib/constants"
	"chromiumos/tast/local/bundles/cros/video/lib/pre"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/metrics"
	"chromiumos/tast/local/perf"
	"chromiumos/tast/testing"
)

const addStatsJs = `
var googMaxDecodeMs = new Array();
var googDecodeMs = new Array();
addStats = function(data) {
  reports = data.reports;
  for (var i=0; i < reports.length; i++) {
    if (reports[i].type == "ssrc") {
      values = reports[i].stats.values;
      for (var j=0; j < values.length; j++) {
        if (values[j] == "googMaxDecodeMs")
          googMaxDecodeMs[googMaxDecodeMs.length] = values[j+1];
        else if (values[j] == "googDecodeMs")
          googDecodeMs[googDecodeMs.length] = values[j+1];
      }
    }
  }
}`

// openWebRTCInternalsPage opens WebRTC internals page and replaces JS
// addStats() to intercept WebRTC performance metrics, "googMaxDecodeMs"
// and "googDecodeMs".
func openWebRTCInternalsPage(ctx context.Context, cr *chrome.Chrome) (*chrome.Conn, error) {
	const url = "chrome://webrtc-internals"
	conn, err := cr.NewConn(ctx, url)
	if err != nil {
		return nil, errors.Wrap(err, "failed to open "+url)
	}
	err = conn.WaitForExpr(ctx, "document.readyState == 'complete'")
	if err != nil {
		conn.Close()
		return nil, err
	}
	if err = conn.Exec(ctx, addStatsJs); err != nil {
		conn.Close()
		return nil, err
	}
	return conn, nil
}

func getMedian(s []int) float64 {
	sort.Ints(s)
	size := len(s)
	if size%2 != 0 {
		return float64(s[size/2])
	}
	return float64(s[size/2]+s[size/2-1]) / 2
}

func getMax(s []int) int {
	max := math.MinInt32
	for _, n := range s {
		if n > max {
			max = n
		}
	}
	return max
}

// Gathers maximum decode time and median decode time.
func gatherDecodeTime(ctx context.Context, s *testing.State, cr *chrome.Chrome, max *int, median *float64) error {
	const (
		gatherDecodeTimeout  = time.Minute * 1
		gatherInterval       = time.Millisecond * 500
		numDecodeTimeSamples = 10
	)
	conn, err := openWebRTCInternalsPage(ctx, cr)
	if err != nil {
		return err
	}
	defer conn.Close()
	defer conn.CloseTarget(ctx)

	// Maximum decode time of the frames of the last 10 seconds.
	var googMaxDecodeMs []int
	// The decode time of the last frame.
	var googDecodeMs []int

	if err := testing.Poll(ctx,
		func(ctx context.Context) error {
			if e := conn.Eval(ctx, "googDecodeMs", &googDecodeMs); e != nil {
				return testing.PollBreak(errors.Wrap(e, "unable to eval googDecodeMs"))
			}
			if len(googDecodeMs) < numDecodeTimeSamples {
				return errors.New("not enough googDecodeMs")
			}
			if e := conn.Eval(ctx, "googMaxDecodeMs", &googMaxDecodeMs); e != nil {
				return testing.PollBreak(errors.Wrap(e, "unable to eval googMaxDecodeMs"))
			}
			return nil
		}, &testing.PollOptions{Interval: gatherInterval, Timeout: gatherDecodeTimeout}); err != nil {
		return err
	}

	if len(googMaxDecodeMs) < numDecodeTimeSamples {
		return errors.Errorf("#googMaxDecodeMs %d < %d", len(googMaxDecodeMs), numDecodeTimeSamples)
	}
	if len(googDecodeMs) < numDecodeTimeSamples {
		return errors.Errorf("#googDecodeMs %d < %d", len(googDecodeMs), numDecodeTimeSamples)
	}
	*max = getMax(googMaxDecodeMs)
	*median = getMedian(googDecodeMs)
	s.Logf("Max decode time list=%v", googMaxDecodeMs)
	s.Logf("Decode time list=%v", googDecodeMs)
	s.Logf("Maximum decode time=%d, median=%f", *max, *median)
	return nil
}

func isDecodeHardwareAccelerated(ctx context.Context, s *testing.State, cr *chrome.Chrome, isHwAccel *bool) error {
	h, err := metrics.GetHistogram(ctx, cr, constants.RTCVDInitStatus)
	if err != nil {
		return err
	}
	if h == nil {
		return errors.New("unable to get histogram " + constants.RTCVDInitStatus)
	}
	buckets := ""
	for _, b := range h.Buckets {
		buckets += fmt.Sprintf("(%d, %d, %d), ", b.Min, b.Max, b.Count)
	}
	s.Logf("Got histogram %s with buckets (min,max,count): [%s]", constants.RTCVDInitStatus, buckets)
	*isHwAccel = len(h.Buckets) == 1 && h.Buckets[0].Min == constants.RTCVDInitSuccess
	return nil
}

func webRTCPerf(ctx context.Context, s *testing.State, streamFile, loopbackURL string,
	disableHwAccel bool, isHwAccel *bool, p *perf.Values) {
	chromeArgs := pre.ChromeArgsWithCameraInput(streamFile)
	if disableHwAccel {
		chromeArgs = append(chromeArgs, "--disable-accelerated-video-decode")
	}
	cr, err := chrome.New(ctx, chrome.ExtraArgs(chromeArgs...))
	if err != nil {
		s.Fatal("Failed to create Chrome: " + err.Error())
	}
	defer cr.Close(ctx)

	conn, err := cr.NewConn(ctx, loopbackURL)
	if err != nil {
		s.Fatalf("Failed to open %s: %s", loopbackURL, err.Error())
	}
	defer conn.Close()
	// Close the tab to stop loopback after test.
	defer conn.CloseTarget(ctx)

	if err := conn.WaitForExpr(ctx, "streamReady"); err != nil {
		s.Fatal("Timed out waiting for stream ready: " + err.Error())
	}

	if err := checkError(ctx, conn); err != nil {
		s.Fatal("Error sanity check loopback web page: " + err.Error())
	}

	var maxDecodeTime int
	var medianDecodeTime float64
	if err := gatherDecodeTime(ctx, s, cr, &maxDecodeTime, &medianDecodeTime); err != nil {
		s.Error("Failed to gather decode time metric: " + err.Error())
	}

	if err := isDecodeHardwareAccelerated(ctx, s, cr, isHwAccel); err != nil {
		s.Error("Failed to determine if hardware accelerator is used: " + err.Error())
	}
	if disableHwAccel && *isHwAccel {
		s.Fatal("HW decode should not be used")
	}
	prefix := "sw_"
	if *isHwAccel {
		prefix = "hw_"
	}

	p.Set(perf.Metric{
		Name:      prefix + "decode_time.max",
		Unit:      "milliseconds",
		Direction: perf.SmallerIsBetter},
		float64(maxDecodeTime))
	p.Set(perf.Metric{
		Name:      prefix + "decode_time.percentile_0.50",
		Unit:      "milliseconds",
		Direction: perf.SmallerIsBetter},
		medianDecodeTime)

	return
}

// RunWebRTCPerf opens a WebRTC loopback page and communicates WebRTC in a fake way.
// The capture stream on WebRTC is specified from streamName.
func RunWebRTCPerf(ctx context.Context, s *testing.State, streamName string) {
	streamFilePath := s.DataPath(streamName)
	server := httptest.NewServer(http.FileServer(s.DataFileSystem()))
	defer server.Close()
	loopbackURL := server.URL + "/loopback.html"

	p := perf.NewValues()
	// Try hardware accelerated WebRTC first.
	isHwAccel := false
	webRTCPerf(ctx, s, streamFilePath, loopbackURL, false, &isHwAccel, p)
	if !isHwAccel {
		return
	}
	// If previous run is hardware accelerated, run without hardware acceleration again.
	webRTCPerf(ctx, s, streamFilePath, loopbackURL, true, &isHwAccel, p)
	p.Save(s.OutDir())
	return
}
