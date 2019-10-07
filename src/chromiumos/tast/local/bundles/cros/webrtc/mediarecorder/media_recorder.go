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
	"chromiumos/tast/local/audio"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/metrics"
	"chromiumos/tast/local/media/constants"
	"chromiumos/tast/local/media/cpu"
	"chromiumos/tast/local/media/histogram"
	"chromiumos/tast/local/media/logging"
	"chromiumos/tast/local/media/videotype"
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

	// PerfStreamFile is the name of the data file used for performance testing.
	PerfStreamFile = "crowd720_25frames.y4m"
)

func reportMetric(name, unit string, value float64, direction perf.Direction, p *perf.Values) {
	p.Set(perf.Metric{
		// TODO(crbug.com/955957): Remove "tast_" prefix after removing video_MediaRecoderPerf in autotest
		Name:      "tast_" + name,
		Unit:      unit,
		Direction: direction,
	}, value)
}

func getMetricName(name string, hwAccelUsed bool) string {
	if hwAccelUsed {
		return "hw_" + name
	}
	return "sw_" + name
}

// MeasurePerf measures the frame processing time and CPU usage while recording and report the results.
func MeasurePerf(ctx context.Context, fileSystem http.FileSystem, outDir string, codec videotype.Codec, streamFile string, fps int) error {
	if err := audio.Mute(ctx); err != nil {
		return errors.Wrap(err, "failed to mute device")
	}
	defer audio.Unmute(ctx)

	p := perf.NewValues()
	hwAccelUsed, err := measureAndReport(ctx, fileSystem, outDir, codec, streamFile, fps, true, p)
	if err != nil {
		return err
	}

	if !hwAccelUsed {
		// Requested HW but got SW result. Don't need to measure SW again.
		if err := p.Save(outDir); err != nil {
			return errors.Wrap(err, "failed to store performance data")
		}
		return nil
	}
	_, err = measureAndReport(ctx, fileSystem, outDir, codec, streamFile, fps, false, p)
	if err != nil {
		return err
	}
	if err = p.Save(outDir); err != nil {
		return errors.Wrap(err, "failed to store performance data")
	}
	return nil
}

func measureAndReport(ctx context.Context, fileSystem http.FileSystem, outDir string, codec videotype.Codec,
	streamFile string, fps int, hwAccelEnabled bool, p *perf.Values) (hwAccelUsed bool, err error) {
	processingTime, cpuUsage, hwAccelUsed, err := doMeasurePerf(ctx, fileSystem, outDir, codec, !hwAccelEnabled, streamFile, fps)
	if err != nil {
		return hwAccelUsed, errors.Wrapf(err, "failed to measure perf. HWAccel requested = %v used = %v", hwAccelEnabled, hwAccelUsed)
	}
	testing.ContextLogf(ctx, "HW requested = %v used = %v processing time = %v cpu usage = %v", hwAccelEnabled, hwAccelUsed, processingTime, cpuUsage)
	reportPerf(processingTime, cpuUsage, hwAccelUsed, p)
	return hwAccelUsed, nil
}

func getChromeArgs(fps int, streamFile string, disableHWAccel bool, codec videotype.Codec) (chromeArgs []string) {
	chromeArgs = []string{
		// "--use-fake-ui-for-media-stream" avoids the need to grant camera/microphone permissions.
		// "--use-fake-device-for-media-stream" feeds fake stream with specified fps to getUserMedia() instead of live camera input.
		// "--use-file-for-fake-video-capture=" feeds a Y4M test file to getUserMedia() instead of live camera input.
		"--use-fake-ui-for-media-stream",
		fmt.Sprintf("--use-fake-device-for-media-stream=\"fps=%d\"", fps),
		"--use-file-for-fake-video-capture=" + streamFile,
	}
	if disableHWAccel {
		chromeArgs = append(chromeArgs, "--disable-accelerated-video-encode")
	} else if codec == videotype.VP9 {
		// Vaapi VP9 Encoder is disabled by default on Chrome. Enable the feature by the command line option.
		chromeArgs = append(chromeArgs, "--enable-features=VaapiVP9Encoder")
	}

	return chromeArgs
}

func reportPerf(processingTime time.Duration, cpuUsage float64, hwAccelUsed bool, p *perf.Values) {
	metricName := getMetricName("frame_processing_time", hwAccelUsed)
	reportMetric(metricName, "millisecond", float64(processingTime.Nanoseconds()*1000000), perf.SmallerIsBetter, p)
	metricName = getMetricName("cpu_usage", hwAccelUsed)
	reportMetric(metricName, "percent", cpuUsage, perf.SmallerIsBetter, p)
}

// doMeasurePerf measures the frame processing time and CPU usage while recording.
func doMeasurePerf(ctx context.Context, fileSystem http.FileSystem, outDir string, codec videotype.Codec, disableHWAccel bool,
	streamFile string, fps int) (processingTime time.Duration, cpuUsage float64, hwAccelUsed bool, err error) {
	// time reserved for cleanup.
	const cleanupTime = 10 * time.Second

	cr, err := chrome.New(ctx, chrome.ExtraArgs(getChromeArgs(fps, streamFile, disableHWAccel, codec)...))
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

	initHistogram, err := metrics.GetHistogram(ctx, cr, constants.MediaRecorderVEAUsed)
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

	// startRecording() will start record a video in given format. The recording will end when stopRecording() is called.
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

	hwUsed, err := histogram.WasHWAccelUsed(ctx, cr, initHistogram, constants.MediaRecorderVEAUsed, int64(constants.MediaRecorderVEAUsedSuccess))
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

	processingTime = time.Duration(elapsedTimeMs/frames) * time.Millisecond
	return processingTime, cpuUsage, hwUsed, nil
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
	usage, err = cpu.MeasureUsage(ctx, measurementDuration)
	if err != nil {
		return 0, errors.Wrap(err, "failed to measure CPU usage")
	}
	return usage, nil
}

// VerifyEncodeAccelUsed checks whether HW encode is used for given codec when running
// MediaRecorder.
func VerifyEncodeAccelUsed(ctx context.Context, s *testing.State, codec videotype.Codec) {
	if err := audio.Mute(ctx); err != nil {
		s.Fatal("Failed to mute device: ", err)
	}
	defer audio.Unmute(ctx)

	chromeArgs := []string{
		logging.ChromeVmoduleFlag(),
		// See https://webrtc.org/testing/
		// "--use-fake-device-for-media-stream" feeds a test pattern to getUserMedia() instead of live camera input.
		// "--use-fake-ui-for-media-stream" avoids the need to grant camera/microphone permissions.
		"--use-fake-device-for-media-stream",
		"--use-fake-ui-for-media-stream",
	}
	if codec == videotype.VP9 {
		// Vaapi VP9 Encoder is disabled by default on Chrome. Enable the feature by the command line option.
		chromeArgs = append(chromeArgs, "--enable-features=VaapiVP9Encoder")
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
	defer conn.CloseTarget(ctx)

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
