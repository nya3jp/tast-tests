// Copyright 2018 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package playback provides common code for video.PlayBack* tests.
package playback

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"time"

	"chromiumos/tast/local/bundles/cros/video/common"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/faillog"
	"chromiumos/tast/testing"
)

const (
	PlaybackWithHWAcceleration    = "playback_with_hw_acceleration"
	PlaybackWithoutHWAcceleration = "playback_without_hw_acceleration"

	MesurementDuration = 30

	DroppedFrameDesc        = "video_dropped_frames_"
	DroppedFramePercentDesc = "video_dropped_frames_percent_"
)

type metricsFunc func(context.Context, *chrome.Conn) map[string]float64

func getDroppedFrames(ctx context.Context, conn *chrome.Conn) map[string]float64 {
	time.Sleep(time.Duration(MesurementDuration) * time.Second)
	decodedFrameCount, droppedFrameCount := 0, 0
	if err := conn.Eval(ctx, "document.getElementsByTagName('video')[0].webkitDecodedFrameCount", &decodedFrameCount); err != nil {
		testing.ContextLogf(ctx, "Failed to get # of decoded frames, %v", err)
		return nil
	}
	if err := conn.Eval(ctx, "document.getElementsByTagName('video')[0].webkitDroppedFrameCount", &droppedFrameCount); err != nil {
		testing.ContextLogf(ctx, "Failed to get # of decoded frames, %v", err)
		return nil
	}
	droppedFramePercent := 0.0
	if decodedFrameCount != 0 {
		droppedFramePercent = 100.0 * float64(droppedFrameCount) / float64(decodedFrameCount)
	} else {
		testing.ContextLogf(ctx, "No frame is decoded. Set drop percent to 100.")
		droppedFramePercent = 100.0
	}
	return map[string]float64{
		DroppedFrameDesc:        float64(droppedFrameCount),
		DroppedFramePercentDesc: droppedFramePercent,
	}
}

func startPlayback(s *testing.State, video string, gatherResultFunc metricsFunc, keyvals *map[string]map[string]float64, disableHWAcceleration bool) {
	ctx := s.Context()
	cr, err := chrome.New(ctx)
	if err != nil {
		s.Fatal("Failed to connect to Chrome: ", err)
		return
	}
	defer cr.Close(ctx)
	// TODO(hiroh): enforce the idle checks after login.
	server := httptest.NewServer(http.FileServer(s.DataFileSystem()))
	defer server.Close()

	hd, err := common.NewHistogramDiffer(ctx, cr, common.MediaGVDInitStatus)
	if err != nil {
		s.Fatal("Failed to create HistogramDiffer.")
		return
	}
	conn, err := cr.NewConn(ctx, server.URL+"/"+video)
	defer conn.Close()
	result := gatherResultFunc(ctx, conn)

	histogramDiff := common.PollHistogramGrow(ctx, hd, 10, 1)
	if len(histogramDiff) > 1 {
		s.Fatal("Unexpected Histogram Difference: %v", histogramDiff)
		return
	}
	if _, found := histogramDiff[common.MediaGVDBucket]; found {
		if disableHWAcceleration {
			s.Fatal("Video Decode Acceleration should not be working.")
			return
		}
		(*keyvals)[PlaybackWithHWAcceleration] = result
	} else if disableHWAcceleration {
		// Software playback performance is ignored, unless HW Acceleration is disabled.
		(*keyvals)[PlaybackWithoutHWAcceleration] = result
	}
}

func playback(s *testing.State, video string, gatherResultFunc metricsFunc) map[string]map[string]float64 {
	// TODO(hiroh): enforce the idle checks after login.
	keyvals := make(map[string]map[string]float64)
	// Try without disabling HW Acceleration
	startPlayback(s, video, gatherResultFunc, &keyvals, true)
	// Try witout disabling HW Acceleration
	startPlayback(s, video, gatherResultFunc, &keyvals, false)
	return keyvals

}

func RunTest(s *testing.State, video string, videoDesc string) {
	defer common.DisableVideoLogs(common.EnableVideoLogs(s.Context()))
	defer faillog.SaveIfError(s)
	keyvals := playback(s, video, getDroppedFrames)
	fmt.Println(keyvals)
}
