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
	"chromiumos/tast/local/bundles/cros/video/lib/pre"
	"chromiumos/tast/local/chrome"
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

const (
	gatherDecodeTimeout  = time.Minute
	gatherInterval       = time.Millisecond * 500
	numDecodeTimeSamples = 10

	webRTCWithHwAccel    = "webrtc_with_hw_acceleration"
	webRTCWithoutHwAccel = "webrtc_without_hw_acceleration"
	webRTCInternalsURL   = "chrome://webrtc-internals"
)

// Opens WebRTC internals page and replaces JS addStats() to
// intercept WebRTC performance metrics, "googMaxDecodeMs" and
// "googDecodeMs".
func openWebRTCInternalsPage(ctx context.Context, cr *chrome.Chrome) (*chrome.Conn, error) {
	conn, err := cr.NewConn(ctx, webRTCInternalsURL)
	if err != nil {
		conn.Close()
		return nil, errors.Wrap(err, "failed to open video page")
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

func median(s []int) float64 {
	sort.Ints(s)
	size := len(s)
	if size%2 != 0 {
		return float64(s[size/2])
	}
	return float64(s[size/2]+s[size/2-1]) / 2
}

func max(s []int) int {
	max := math.MinInt32
	for _, n := range s {
		if n > max {
			max = n
		}
	}
	return max
}

// Gathers maximum decode time and median decode time.
func gatherDecodeTime(ctx context.Context, s *testing.State, cr *chrome.Chrome) (string, error) {
	var err error
	conn, err := openWebRTCInternalsPage(ctx, cr)
	if err != nil {
		conn.Close()
		return "", err
	}
	defer conn.CloseTarget(ctx)

	// Maximum decode time of the frames of the last 10 seconds.
	var googMaxDecodeMs []int
	// The decode time of the last frame.
	var googDecodeMs []int

	if err = testing.Poll(ctx,
		func(ctx context.Context) error {
			var e error
			if e = conn.Eval(ctx, "googDecodeMs", &googDecodeMs); e != nil {
				return testing.PollBreak(errors.Wrap(e, "unable to eval googDecodeMs"))
			}
			if len(googDecodeMs) < numDecodeTimeSamples {
				return errors.New("not enough googDecodeMs")
			}
			if e = conn.Eval(ctx, "googMaxDecodeMs", &googMaxDecodeMs); e != nil {
				return testing.PollBreak(errors.Wrap(e, "unable to eval googMaxDecodeMs"))
			}
			return nil
		}, &testing.PollOptions{Interval: gatherInterval, Timeout: gatherDecodeTimeout}); err != nil {
		return "", err
	}

	if len(googMaxDecodeMs) < numDecodeTimeSamples {
		return "", errors.Errorf("#googMaxDecodeMs %d < %d", len(googMaxDecodeMs), numDecodeTimeSamples)
	}
	if len(googDecodeMs) < numDecodeTimeSamples {
		return "", errors.Errorf("#googDecodeMs %d < %d", len(googDecodeMs), numDecodeTimeSamples)
	}
	maxDecodeTime := max(googMaxDecodeMs)
	medianDecodeTime := median(googDecodeMs)
	s.Logf("max decode time list=%v", googMaxDecodeMs)
	s.Logf("decode time list=%v", googDecodeMs)
	s.Logf("maximum decode time=%d, median=%f", maxDecodeTime, medianDecodeTime)
	return fmt.Sprintf("(%d, %f)", maxDecodeTime, medianDecodeTime), nil
}

// RunWebRTCPerf opens a WebRTC loopback page and communicates WebRTC in a fake way.
// The capture stream on WebRTC is specified from streamName.
func RunWebRTCPerf(ctx context.Context, s *testing.State, streamName string) error {
	chromeArgs := pre.ChromeArgsWithCameraInput(s.DataPath(streamName))
	//if disableHardwareAcceleration {
	//	chromeArgs = append(chromeArgs, "--disable-accelerated-video-decode")
	//}
	cr, err := chrome.New(ctx, chrome.ExtraArgs(chromeArgs...))
	if err != nil {
		return errors.Wrap(err, "failed to connect to Chrome")
	}
	defer cr.Close(ctx)

	server := httptest.NewServer(http.FileServer(s.DataFileSystem()))
	defer server.Close()

	conn, err := cr.NewConn(ctx, server.URL+"/loopback.html")
	if err != nil {
		return errors.Wrap(err, "failed to open video page")
	}
	defer conn.Close()
	// Close the tab to stop loopback after test.
	defer conn.CloseTarget(ctx)

	if err := conn.WaitForExpr(ctx, "streamReady"); err != nil {
		return errors.Wrap(err, "timed out waiting for stream ready")
	}

	if err := checkError(ctx, conn); err != nil {
		return err
	}
	decodeTime, err := gatherDecodeTime(ctx, s, cr)
	if err != nil {
		return errors.Wrap(err, "failed to gather decode time metric")
	}
	s.Logf("metric: %s", decodeTime)
	return nil
}
