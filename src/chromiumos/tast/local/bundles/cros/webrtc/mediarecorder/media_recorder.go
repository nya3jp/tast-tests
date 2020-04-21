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

	"chromiumos/tast/ctxutil"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/ash"
	"chromiumos/tast/local/chrome/display"
	"chromiumos/tast/local/chrome/metrics"
	"chromiumos/tast/local/media/constants"
	"chromiumos/tast/local/media/cpu"
	"chromiumos/tast/local/media/histogram"
	"chromiumos/tast/local/media/videotype"
	"chromiumos/tast/local/perf"
	"chromiumos/tast/testing"
)

const (
	stabilizationDuration = 5 * time.Second
	measurementDuration   = 15 * time.Second
)

func reportMetric(name, unit string, value float64, direction perf.Direction, p *perf.Values) {
	p.Set(perf.Metric{
		Name:      name,
		Unit:      unit,
		Direction: direction,
	}, value)
}

// MeasurePerf measures the frame processing time and CPU usage while recording and report the results.
func MeasurePerf(ctx context.Context, cr *chrome.Chrome, fileSystem http.FileSystem, outDir, codec string, hwAccelEnabled bool) error {

	p := perf.NewValues()
	// Wait until CPU is idle enough. CPU usage can be high immediately after login for various reasons (e.g. animated images on the lock screen).
	cleanUpBenchmark, err := cpu.SetUpBenchmark(ctx)
	if err != nil {
		return errors.Wrap(err, "failed to set up CPU benchmark")
	}
	defer cleanUpBenchmark(ctx)

	// Reserve time for cleanup at the end of the test.
	const cleanupTime = 10 * time.Second
	ctx, cancel := ctxutil.Shorten(ctx, cleanupTime)
	defer cancel()

	if err := cpu.WaitUntilIdle(ctx); err != nil {
		return errors.Wrap(err, "failed waiting for CPU to become idle")
	}

	server := httptest.NewServer(http.FileServer(fileSystem))
	defer server.Close()

	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		return err
	}

	initHistogram, err := metrics.GetHistogram(ctx, tconn, constants.MediaRecorderVEAUsed)
	if err != nil {
		return errors.Wrap(err, "failed to get initial histogram")
	}

	conn, err := cr.NewConn(ctx, server.URL+"/loopback_media_recorder.html")
	if err != nil {
		return errors.Wrap(err, "failed to open recorder page")
	}
	defer conn.Close()
	defer conn.CloseTarget(ctx)

	if err := conn.WaitForExpr(ctx, "pageLoaded"); err != nil {
		return errors.Wrap(err, "timed out waiting for page loading")
	}

	// startRecording() a video in given format until stopRecording() is called.
	if err := conn.Call(ctx, nil, "startRecording", codec); err != nil {
		return errors.Wrapf(err, "failed to evaluate startRecording(%s)", codec)
	}

	testing.ContextLogf(ctx, "Sleeping %v to wait for CPU usage to stabilize", stabilizationDuration.Round(time.Second))
	// TODO(mcasas): Remove testing.Sleep() and use testing.Poll() instead.
	if err := testing.Sleep(ctx, stabilizationDuration); err != nil {
		return errors.Wrap(err, "failed waiting for CPU usage to stabilize")
	}

	// While the video recording is in progress, measure CPU usage.
	measurements, err := cpu.MeasureUsage(ctx, measurementDuration)
	if err != nil {
		return errors.Wrap(err, "error measuring cpu")
	}

	// Recorded video will be saved in |videoBuffer| in base64 format.
	var videoBuffer string
	if err := conn.Eval(ctx, "stopRecording()", &videoBuffer); err != nil {
		return errors.Wrap(err, "failed to stop recording")
	}

	hwAccelUsed, err := histogram.WasHWAccelUsed(ctx, tconn, initHistogram, constants.MediaRecorderVEAUsed, int64(constants.MediaRecorderVEAUsedSuccess))
	if err != nil {
		return errors.Wrap(err, "failed to get histogram")
	}
	if hwAccelEnabled {
		if !hwAccelUsed {
			return errors.Wrap(err, "Hw accelerator requested but not used")
		}
	} else {
		if hwAccelUsed {
			return errors.Wrap(err, "Hw accelerator not requested but used")
		}
	}

	processingTimePerFrame, err := calculateTimePerFrame(ctx, conn, videoBuffer, outDir)
	if err != nil {
		return errors.Wrap(err, "failed to calculate the processig time per frame")
	}

	reportMetric("frame_processing_time", "millisecond", float64(processingTimePerFrame.Milliseconds()), perf.SmallerIsBetter, p)
	cpuUsage := measurements["cpu"]
	reportMetric("cpu_usage", "percent", cpuUsage, perf.SmallerIsBetter, p)
	testing.ContextLogf(ctx, "Processing time per frame = %v, cpu usage = %v", processingTimePerFrame, cpuUsage)

	if power, ok := measurements["power"]; ok {
		reportMetric("power", "W", power, perf.SmallerIsBetter, p)
		testing.ContextLogf(ctx, "Avg pkg power usage: %fW", power)
	}

	if err := p.Save(outDir); err != nil {
		return errors.Wrap(err, "failed to store performance data")
	}
	return nil
}

func calculateTimePerFrame(ctx context.Context, conn *chrome.Conn, videoBuffer, outDir string) (timePerFrame time.Duration, err error) {
	elapsedTimeMs := 0
	if err := conn.Eval(ctx, "elapsedTime", &elapsedTimeMs); err != nil {
		return 0, errors.Wrap(err, "failed to evaluate elapsedTime")
	}

	videoBytes, err := base64.StdEncoding.DecodeString(videoBuffer)
	if err != nil {
		return 0, errors.Wrap(err, "failed to decode base64 string into byte array")
	}

	frames := 0
	if frames, err = computeNumFrames(videoBytes, outDir); err != nil {
		return 0, errors.Wrap(err, "failed to compute number of frames")
	}

	return time.Duration(elapsedTimeMs/frames) * time.Millisecond, nil
}

// computeNumFrames computes number of frames in the given MKV video byte array.
func computeNumFrames(videoBytes []byte, tmpDir string) (frameNum int, err error) {
	videoFilePath := filepath.Join(tmpDir, "recorded_video.mkv")
	if err := ioutil.WriteFile(videoFilePath, videoBytes, 0644); err != nil {
		return 0, errors.Wrap(err, "failed to open file")
	}

	doc, err := matroska.Decode(videoFilePath)
	if err != nil {
		return 0, errors.Wrap(err, "failed to parse video file")
	}

	videoTrackNum := 0
VideoTrackNumLoop:
	for _, track := range doc.Segment.Tracks {
		for _, entry := range track.Entries {
			if entry.Type == matroska.TrackTypeVideo {
				videoTrackNum = int(entry.Number)
				break VideoTrackNumLoop
			}
		}
	}

	frameNum = 0
	for _, cluster := range doc.Segment.Cluster {
		for _, block := range cluster.SimpleBlock {
			if int(block.TrackNumber) != videoTrackNum {
				continue
			}
			if (block.Flags & matroska.LacingNone) != 0 {
				frameNum++
			} else {
				frameNum += (block.Frames + 1)
			}
		}
		for _, blockGroup := range cluster.BlockGroup {
			if int(blockGroup.Block.TrackNumber) != videoTrackNum {
				continue
			}
			if (blockGroup.Block.Flags & matroska.LacingNone) != 0 {
				frameNum++
			} else {
				frameNum += (blockGroup.Block.Frames + 1)
			}
		}
	}

	return frameNum, nil
}

// VerifyMediaRecorderUsesEncodeAccelerator checks whether MediaRecorder uses HW encoder for |codec|.
func VerifyMediaRecorderUsesEncodeAccelerator(ctx context.Context, s *testing.State, cr *chrome.Chrome, codec videotype.Codec) {
	server := httptest.NewServer(http.FileServer(s.DataFileSystem()))
	defer server.Close()

	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to connect to test API: ", err)
	}

	// Real webcams on tablets might capture a rotated feed and make one of the
	// dimensions too large for the hardware encoder, see crbug.com/1071979. Set
	// the device in landscape mode to match the expected video call orientation.
	tabletModeEnabled, err := ash.TabletModeEnabled(ctx, tconn)
	if err != nil {
		s.Fatal("Failed to get tablet mode: ", err)
	}
	if tabletModeEnabled {
		dispInfo, err := display.GetInternalInfo(ctx, tconn)
		if err != nil {
			s.Fatal("Failed to get internal display info: ", err)
		}
		// Ideally we'd use screen.orientation.lock("landscape"), but that needs the
		// content to be in full screen (requestFullscreen()), which needs a user
		// gesture. Instead, implement the algorithm: landscape is, by definition,
		// when the screen's width is larger than the height, see
		// https://w3c.github.io/screen-orientation/#dfn-landscape-primary
		var width, height int64
		if err := tconn.Eval(ctx, "window.screen.width", &width); err != nil {
			s.Fatal("Failed to retrieve screen width: ", err)
		}
		if err := tconn.Eval(ctx, "window.screen.height", &height); err != nil {
			s.Fatal("Failed to retrieve screen height: ", err)
		}
		rotation := display.Rotate0
		if height > width {
			rotation = display.Rotate90
		}

		if err := display.SetDisplayRotationSync(ctx, tconn, dispInfo.ID, rotation); err != nil {
			s.Fatal("Failed to rotate display: ", err)
		}
	}

	initHistogram, err := metrics.GetHistogram(ctx, tconn, constants.MediaRecorderVEAUsed)
	if err != nil {
		s.Fatal("Failed to get initial histogram: ", err)
	}

	conn, err := cr.NewConn(ctx, server.URL+"/loopback_media_recorder.html")
	if err != nil {
		s.Fatal("Failed to open recorder page: ", err)
	}
	defer conn.Close()
	defer conn.CloseTarget(ctx)

	if err := conn.WaitForExpr(ctx, "pageLoaded"); err != nil {
		s.Fatal("Timed out waiting for page loading: ", err)
	}

	startRecordJS := fmt.Sprintf("startRecordingForResult(%q)", codec)
	if err := conn.EvalPromise(ctx, startRecordJS, nil); err != nil {
		s.Fatalf("Failed to evaluate %v: %v", startRecordJS, err)
	}

	if hwUsed, err := histogram.WasHWAccelUsed(ctx, tconn, initHistogram, constants.MediaRecorderVEAUsed, int64(constants.MediaRecorderVEAUsedSuccess)); err != nil {
		s.Fatal("Failed to verify histogram: ", err)
	} else if !hwUsed {
		s.Error("HW accel was not used")
	}
}
