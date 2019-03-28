// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package mediarecorder provides common code for video.MediaRecorder tests.
package mediarecorder

import (
	"context"
	"encoding/base64"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"time"

	"github.com/pixelbender/go-matroska/matroska"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/bundles/cros/video/lib/constants"
	"chromiumos/tast/local/bundles/cros/video/lib/cpu"
	"chromiumos/tast/local/bundles/cros/video/lib/histogram"
	"chromiumos/tast/local/bundles/cros/video/lib/logging"
	"chromiumos/tast/local/bundles/cros/video/lib/videotype"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/metrics"
	"chromiumos/tast/local/perf"
	"chromiumos/tast/testing"
)

const (
	stabilizationDuration = 5 * time.Second
	measurementDuration   = 15 * time.Second
	// The maximum time we will wait for the CPU to become idle.
	waitIdleCPUTimeout = 30 * time.Second

	// The CPU is considered idle when average usage is below this threshold.
	idleCPUUsagePercent = 10.0
)

// ReportPerf reports metric data to the server.
func ReportPerf(name, unit, outDir string, value float64, direction perf.Direction) error {
	p := perf.NewValues()
	p.Set(perf.Metric{
		Name:      name,
		Unit:      unit,
		Direction: direction,
	}, value)
	return p.Save(outDir)
}

// MeasurePerf measures the frame processing time and CPU usage while recording.
func MeasurePerf(ctx context.Context, fileSystem http.FileSystem, outDir string, chromeArgs []string, codec videotype.Codec, processingTime *int, cpuUsage *float64) error {
	cr, err := chrome.New(ctx, chrome.ExtraArgs(chromeArgs...))
	if err != nil {
		return errors.Wrap(err, "Failed to connect to Chrome")
	}
	defer cr.Close(ctx)

	// Wait until CPU is idle enough. CPU usage can be high immediately after login for various reasons (e.g. animated images on the lock screen).
	shortCtx, cleanupBenchmark, err := cpu.SetUpBenchmark(ctx)
	if err != nil {
		return errors.Wrap(err, "Failed to set up benchmark")
	}
	defer cleanupBenchmark()

	server := httptest.NewServer(http.FileServer(fileSystem))
	defer server.Close()

	conn, err := cr.NewConn(shortCtx, server.URL+"/loopback_media_recorder.html")
	if err != nil {
		return errors.Wrap(err, "Failed to open recorder page")
	}
	defer conn.Close()

	if err := conn.WaitForExpr(shortCtx, "pageLoaded"); err != nil {
		return errors.Wrap(err, "Timed out waiting for page loading")
	}

	startRecordJS := fmt.Sprintf("startRecording(%q)", codec)
	if err := conn.EvalPromise(shortCtx, startRecordJS, nil); err != nil {
		return errors.Wrapf(err, "Failed to evaluate %v", startRecordJS)
	}

	// While the video recording is in progress, measure CPU usage.
	if err := measureCPUUsage(shortCtx, conn, cpuUsage); err != nil {
		return errors.Wrap(err, "Failed to measure CPU")
	}

	stopRecordJS := "stopRecording()"
	videoBuffer := ""
	if err := conn.EvalPromise(shortCtx, stopRecordJS, &videoBuffer); err != nil {
		return errors.Wrapf(err, "Failed to evaluate %v", stopRecordJS)
	}

	elapsedTime := 0
	if err := conn.Eval(shortCtx, "elapsedTime", &elapsedTime); err != nil {
		return errors.Wrap(err, "Failed to evaluate elapsedTime")
	}

	videoBytes, err := base64.StdEncoding.DecodeString(videoBuffer)
	if err != nil {
		return errors.Wrap(err, "Failed to decode base64 string into byte array")
	}

	frames := 0
	if err := computeFrameNum(videoBytes, outDir, &frames); err != nil {
		return errors.Wrap(err, "Failed to compute number of frames")
	}

	*processingTime = elapsedTime / frames
	return nil
}

func computeFrameNum(videoBytes []byte, tmpDir string, frameNum *int) error {
	videoFilePath := filepath.Join(tmpDir, "recorded_video.mkv")
	if err := ioutil.WriteFile(videoFilePath, videoBytes, 0644); err != nil {
		return errors.Wrap(err, "Failed to open file")
	}

	doc, err := matroska.Decode(videoFilePath)
	if err != nil {
		return errors.Wrap(err, "Failed to parse video file")
	}

	videoTrackNum := 0
	for _, track := range doc.Segment.Tracks {
		for _, entry := range track.Entries {
			if entry.Type == matroska.TrackTypeVideo {
				videoTrackNum = int(entry.Number)
				break
			}
		}
		if videoTrackNum != 0 {
			break
		}
	}

	frames := 0
	for _, cluster := range doc.Segment.Cluster {
		for _, block := range cluster.SimpleBlock {
			if int(block.TrackNumber) != videoTrackNum {
				continue
			}
			if (block.Flags & matroska.LacingNone) != 0 {
				frames++
			} else {
				frames += (block.Frames + 1)
			}
		}
		for _, blockGroup := range cluster.BlockGroup {
			if int(blockGroup.Block.TrackNumber) != videoTrackNum {
				continue
			}
			if (blockGroup.Block.Flags & matroska.LacingNone) != 0 {
				frames++
			} else {
				frames += (blockGroup.Block.Frames + 1)
			}
		}
	}

	*frameNum = frames
	return nil
}

func measureCPUUsage(ctx context.Context, conn *chrome.Conn, usage *float64) error {
	testing.ContextLogf(ctx, "Sleeping %v to wait for CPU usage to stabilize", stabilizationDuration.Round(time.Second))
	select {
	case <-ctx.Done():
		return errors.Wrap(ctx.Err(), "Failed waiting for CPU usage to stabilize")
	case <-time.After(stabilizationDuration):
	}

	testing.ContextLogf(ctx, "Sleeping %v to measure CPU usage while playing video", measurementDuration.Round(time.Second))
	var err error
	*usage, err = cpu.MeasureUsage(ctx, measurementDuration)
	if err != nil {
		return errors.Wrap(err, "Failed to measure CPU usage")
	}
	return nil
}

// VerifyEncodeAccelUsed checks whether HW encode is used for given codec when running
// MediaRecorder.
func VerifyEncodeAccelUsed(ctx context.Context, s *testing.State, codec videotype.Codec) {
	chromeArgs := []string{
		logging.ChromeVmoduleFlag(),
		// See https://webrtc.org/testing/
		// "--use-fake-device-for-media-stream" feeds a test pattern to getUserMedia() instead of live camera input.
		// "--use-fake-ui-for-media-stream" avoids the need to grant camera/microphone permissions.
		"--use-fake-device-for-media-stream",
		"--use-fake-ui-for-media-stream",
	}

	cr, err := chrome.New(ctx, chrome.ExtraArgs(chromeArgs...))
	if err != nil {
		s.Fatal("Failed to connect to Chrome: ", err)
	}
	defer cr.Close(ctx)

	server := httptest.NewServer(http.FileServer(s.DataFileSystem()))
	defer server.Close()

	initHistogram, err := metrics.GetHistogram(ctx, cr, constants.MediaRecorderVEAUsed)
	if err != nil {
		s.Fatal("Failed to get initial histogram: ", err)
	}

	conn, err := cr.NewConn(ctx, server.URL+"/loopback_media_recorder.html")
	if err != nil {
		s.Fatal("Failed to open recorder page: ", err)
	}
	defer conn.Close()

	if err := conn.WaitForExpr(ctx, "pageLoaded"); err != nil {
		s.Fatal("Timed out waiting for page loading: ", err)
	}

	startRecordJS := fmt.Sprintf("startRecordingForResult(%q)", codec)
	if err := conn.EvalPromise(ctx, startRecordJS, nil); err != nil {
		s.Fatalf("Failed to evaluate %v: %v", startRecordJS, err)
	}

	if hwUsed, err := histogram.WasHWAccelUsed(ctx, cr, initHistogram, constants.MediaRecorderVEAUsed, int64(constants.MediaRecorderVEAUsedSuccess)); err != nil {
		s.Fatal("Failed to verify histogram: ", err)
	} else if !hwUsed {
		s.Error("HW accel was not used")
	}
}
