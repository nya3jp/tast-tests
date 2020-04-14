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

	// PerfStreamFile is the name of the data file used for performance testing.
	PerfStreamFile = "crowd720_25frames.y4m"
)

func reportMetric(name, unit string, value float64, direction perf.Direction, p *perf.Values) {
	p.Set(perf.Metric{
		Name:      name,
		Unit:      unit,
		Direction: direction,
	}, value)
}

// MeasurePerf measures the frame processing time and CPU usage while recording and report the results.
func MeasurePerf(ctx context.Context, fileSystem http.FileSystem, outDir, codec, streamFile string, hwAccelEnabled bool) error {

	p := perf.NewValues()
	hwAccelUsed, err := measureAndReport(ctx, fileSystem, outDir, codec, streamFile, hwAccelEnabled, p)
	if err != nil {
		return err
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

	if err = p.Save(outDir); err != nil {
		return errors.Wrap(err, "failed to store performance data")
	}
	return nil
}

func measureAndReport(ctx context.Context, fileSystem http.FileSystem, outDir, codec,
	streamFile string, hwAccelEnabled bool, p *perf.Values) (hwAccelUsed bool, err error) {
	processingTimePerFrame, cpuUsage, hwAccelUsed, err := doMeasurePerf(ctx, fileSystem, outDir, codec, !hwAccelEnabled, streamFile)
	if err != nil {
		return hwAccelUsed, errors.Wrap(err, "failed to measure perf")
	}
	testing.ContextLogf(ctx, "processing time per frame = %v, cpu usage = %v", processingTimePerFrame, cpuUsage)

	reportMetric("frame_processing_time", "millisecond", float64(processingTimePerFrame.Nanoseconds()*1000000), perf.SmallerIsBetter, p)
	reportMetric("cpu_usage", "percent", cpuUsage, perf.SmallerIsBetter, p)
	return hwAccelUsed, nil
}

func getChromeArgs(streamFile string, disableHWAccel bool, codec string) (chromeArgs []string) {
	chromeArgs = []string{
		// Use a fake media capture device instead of live webcam(s)/microphone(s);
		// this is needed to enable use-file-for-fake-video-capture below.
		// See https://webrtc.org/testing/
		"--use-fake-device-for-media-stream",
		// Avoids the need to grant camera/microphone permissions.
		"--use-fake-ui-for-media-stream",
		// Enable verbose logging of interesting code areas.
		"--vmodule=*recorder*=2,*video*=2",
		// Read a test file as input for the fake media capture device. The file,
		// usually a Y4M, specifies resolution (size) and frame rate.
		"--use-file-for-fake-video-capture=" + streamFile,
	}
	if disableHWAccel {
		chromeArgs = append(chromeArgs, "--disable-accelerated-video-encode")
	} else if codec == "VP9" {
		// Vaapi VP9 Encoder is disabled by default on Chrome. Enable the feature by the command line option.
		chromeArgs = append(chromeArgs, "--enable-features=VaapiVP9Encoder")
	} else if codec == "H264" {
		// Use command line option to enable the H264 encoder on AMD, as it's disabled by default.
		// TODO(b/145961243): Remove this option when VA-API H264 encoder is
		// enabled on grunt by default.
		chromeArgs = append(chromeArgs, "--enable-features=VaapiH264AMDEncoder")
	}

	return chromeArgs
}

// doMeasurePerf measures the frame processing time and CPU usage while recording.
func doMeasurePerf(ctx context.Context, fileSystem http.FileSystem, outDir, codec string, disableHWAccel bool,
	streamFile string) (processingTimePerFrame time.Duration, cpuUsage float64, hwAccelUsed bool, err error) {
	// time reserved for cleanup.
	const cleanupTime = 10 * time.Second

	cr, err := chrome.New(ctx, chrome.ExtraArgs(getChromeArgs(streamFile, disableHWAccel, codec)...))
	if err != nil {
		return 0, 0, false, errors.Wrap(err, "failed to connect to Chrome")
	}
	defer cr.Close(ctx)

	// Wait until CPU is idle enough. CPU usage can be high immediately after login for various reasons (e.g. animated images on the lock screen).
	cleanUpBenchmark, err := cpu.SetUpBenchmark(ctx)
	if err != nil {
		return 0, 0, false, errors.Wrap(err, "failed to set up benchmark")
	}
	defer cleanUpBenchmark(ctx)

	// Reserve time for cleanup at the end of the test.
	ctx, cancel := ctxutil.Shorten(ctx, cleanupTime)
	defer cancel()

	if err := cpu.WaitUntilIdle(ctx); err != nil {
		return 0, 0, false, errors.Wrap(err, "failed waiting for CPU to become idle")
	}

	server := httptest.NewServer(http.FileServer(fileSystem))
	defer server.Close()

	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		return 0, 0, false, err
	}

	initHistogram, err := metrics.GetHistogram(ctx, tconn, constants.MediaRecorderVEAUsed)
	if err != nil {
		return 0, 0, false, errors.Wrap(err, "failed to get initial histogram")
	}

	conn, err := cr.NewConn(ctx, server.URL+"/loopback_media_recorder.html")
	if err != nil {
		return 0, 0, false, errors.Wrap(err, "failed to open recorder page")
	}
	defer conn.Close()
	defer conn.CloseTarget(ctx)

	if err := conn.WaitForExpr(ctx, "pageLoaded"); err != nil {
		return 0, 0, false, errors.Wrap(err, "Timed out waiting for page loading")
	}

	// startRecording() will start recording a video in given format. The recording will end when stopRecording() is called.
	startRecordJS := fmt.Sprintf("startRecording(%q)", codec)
	if err := conn.EvalPromise(ctx, startRecordJS, nil); err != nil {
		return 0, 0, false, errors.Wrapf(err, "failed to evaluate %v", startRecordJS)
	}

	// While the video recording is in progress, measure CPU usage.
	cpuUsage = 0.0
	if cpuUsage, err = measureCPUUsage(ctx, conn); err != nil {
		return 0, 0, false, errors.Wrap(err, "failed to measure CPU")
	}

	// Recorded video will be saved in |videoBuffer| in base64 format.
	videoBuffer := ""
	if err := conn.EvalPromise(ctx, "stopRecording()", &videoBuffer); err != nil {
		return 0, 0, false, errors.Wrap(err, "failed to stop recording")
	}

	hwUsed, err := histogram.WasHWAccelUsed(ctx, tconn, initHistogram, constants.MediaRecorderVEAUsed, int64(constants.MediaRecorderVEAUsedSuccess))
	if err != nil {
		return 0, 0, false, errors.Wrap(err, "failed to get histogram")
	}
	if disableHWAccel && hwUsed {
		return 0, 0, false, errors.New("requested SW but got HW result")
	}

	elapsedTimeMs := 0
	if err := conn.Eval(ctx, "elapsedTime", &elapsedTimeMs); err != nil {
		return 0, 0, false, errors.Wrap(err, "failed to evaluate elapsedTime")
	}

	videoBytes, err := base64.StdEncoding.DecodeString(videoBuffer)
	if err != nil {
		return 0, 0, false, errors.Wrap(err, "failed to decode base64 string into byte array")
	}

	frames := 0
	if frames, err = computeNumFrames(videoBytes, outDir); err != nil {
		return 0, 0, false, errors.Wrap(err, "failed to compute number of frames")
	}

	processingTimePerFrame = time.Duration(elapsedTimeMs/frames) * time.Millisecond
	return processingTimePerFrame, cpuUsage, hwUsed, nil
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

func measureCPUUsage(ctx context.Context, conn *chrome.Conn) (usage float64, err error) {
	testing.ContextLogf(ctx, "Sleeping %v to wait for CPU usage to stabilize", stabilizationDuration.Round(time.Second))
	if err := testing.Sleep(ctx, stabilizationDuration); err != nil {
		return 0, errors.Wrap(ctx.Err(), "failed waiting for CPU usage to stabilize")
	}

	testing.ContextLogf(ctx, "Sleeping %v to measure CPU usage while playing video", measurementDuration.Round(time.Second))
	usage, err = cpu.MeasureCPUUsage(ctx, measurementDuration)
	if err != nil {
		return 0, errors.Wrap(err, "failed to measure CPU usage")
	}
	return usage, nil
}

// VerifyMediaRecorderUsesEncodeAccelerator checks whether MediaRecorder uses HW encoder for |codec|.
func VerifyMediaRecorderUsesEncodeAccelerator(ctx context.Context, s *testing.State, cr *chrome.Chrome, codec videotype.Codec) {
	server := httptest.NewServer(http.FileServer(s.DataFileSystem()))
	defer server.Close()

	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to connect to test API: ", err)
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
