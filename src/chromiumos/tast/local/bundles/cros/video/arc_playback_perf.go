// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package video

import (
	"context"
	"path/filepath"
	"regexp"
	"strconv"
	"time"

	"chromiumos/tast/ctxutil"
	"chromiumos/tast/local/arc"
	"chromiumos/tast/local/arc/ui"
	"chromiumos/tast/local/media/cpu"
	"chromiumos/tast/local/perf"
	"chromiumos/tast/local/testexec"
	"chromiumos/tast/testing"
)

// apkName is the APK file name for testing ARC++ playback.
// Source: https://cs.corp.google.com/arc-nyc-mr1/vendor/google_arc/packages/development/ArcVideoTest/src/org/chromium/arc/testapp/video/MainActivity.java
const apkName = "arc_video_test.apk"

type params struct {
	videoName   string
	measurePerf bool
}

func init() {
	testing.AddTest(&testing.Test{
		Func:         ARCPlaybackPerf,
		Desc:         "Measures video playback performance on ARC++ for H.264/VP8/VP9 1080p@30fps video",
		Contacts:     []string{"johnylin@chromium.org", "chromeos-video-eng@google.com"},
		Attr:         []string{"group:crosbolt", "crosbolt_perbuild"},
		SoftwareDeps: []string{"android", "chrome"},
		Data:         []string{apkName},
		Pre:          arc.Booted(),
		Params: []testing.Param{{
			Name: "h264_1080p_30fps",
			Val: params{
				videoName: "1080p_30fps_600frames.h264.mp4",
			},
			// "chrome_internal" is needed because H.264 is a proprietary codec.
			ExtraSoftwareDeps: []string{"chrome_internal"},
			ExtraData:         []string{"1080p_30fps_600frames.h264.mp4"},
		}, {
			Name: "vp8_1080p_30fps",
			Val: params{
				videoName: "1080p_30fps_600frames.vp8.webm",
			},
			ExtraData: []string{"1080p_30fps_600frames.vp8.webm"},
		}, {
			Name: "vp9_1080p_30fps",
			Val: params{
				videoName: "1080p_30fps_600frames.vp9.webm",
			},
			ExtraData: []string{"1080p_30fps_600frames.vp9.webm"},
		}},
	})
}

// ARCPlaybackPerf plays H.264/VP8/VP9 1080P 30 FPS video by APK on ARC++ and measures CPU usage.
func ARCPlaybackPerf(ctx context.Context, s *testing.State) {
	const (
		pkg = "org.chromium.arc.testapp.video"
		cls = ".MainActivity"

		testLogID = pkg + ":id/test_log"

		keyEventPlay = "126"
		keyEventStop = "86"

		arcFilePath = "/sdcard/Download/"

		// time to wait for CPU to stabilize after launching proc.
		stabilize = 3 * time.Second
		// duration of the interval during which CPU usage will be measured.
		measureDuration = 15 * time.Second
		// the error tolerance on checking total played duration. The duration should be close to the consuming
		// time of stabilization and CPU measurement in normal case.
		durationTolerance  = 2 * time.Second
		durationLowerBound = stabilize + measureDuration - durationTolerance
		durationUpperBound = stabilize + measureDuration + durationTolerance
		// time reserved for cleanup.
		cleanupTime = 10 * time.Second
	)

	a := s.PreValue().(arc.PreData).ARC
	d, err := ui.NewDevice(ctx, a)
	if err != nil {
		s.Fatal("Failed to initialize UI Automator: ", err)
	}
	defer d.Close()

	cleanUpBenchmark, err := cpu.SetUpBenchmark(ctx)
	if err != nil {
		s.Fatal("Failed to set up benchmark mode: ", err)
	}
	defer cleanUpBenchmark(ctx)

	// Leave a bit of time to clean up benchmark mode.
	ctx, cancel := ctxutil.Shorten(ctx, cleanupTime)
	defer cancel()

	s.Log("Waiting for idle CPU")
	if err := cpu.WaitUntilIdle(ctx); err != nil {
		s.Fatal("Failed waiting for CPU to become idle: ", err)
	}

	// Note: we don't need to un-install APK by ourselves, it is done by arc's preImpl.
	s.Log("Installing APK ", apkName)
	if err := a.Install(ctx, s.DataPath(apkName)); err != nil {
		s.Fatal("Failed installing app: ", err)
	}

	s.Log("Granting storage permission")
	if err := a.Command(ctx, "pm", "grant", pkg, "android.permission.READ_EXTERNAL_STORAGE").Run(testexec.DumpLogOnError); err != nil {
		s.Fatal("Failed granting storage permission: ", err)
	}

	videoName := s.Param().(params).videoName
	s.Log("Pushing video file ", videoName)
	if err := a.PushFile(ctx, s.DataPath(videoName), arcFilePath); err != nil {
		s.Fatal("Failed pushing file: ", err)
	}
	videoPath := filepath.Join(arcFilePath, videoName)
	defer a.Command(ctx, "rm", videoPath).Run(testexec.DumpLogOnError)

	s.Log("Starting APK main activity")
	// Use argument "--es PATH <VideoPath>" to load video file.
	if err := a.Command(ctx, "am", "start", "--es", "PATH", videoPath, pkg+"/"+cls).Run(testexec.DumpLogOnError); err != nil {
		s.Fatal("Failed starting APK main activity: ", err)
	}

	s.Log("Playing video")
	if err := a.Command(ctx, "input", "keyevent", keyEventPlay).Run(testexec.DumpLogOnError); err != nil {
		s.Fatal("Failed playing video: ", err)
	}
	defer a.Command(ctx, "input", "keyevent", keyEventStop).Run(testexec.DumpLogOnError)

	if err := testing.Sleep(ctx, stabilize); err != nil {
		s.Fatal("Failed waiting for CPU usage to stabilize: ", err)
	}

	s.Log("Measuring CPU usage for ", measureDuration.Round(time.Second))
	cpuUsage, err := cpu.MeasureCPUUsage(ctx, measureDuration)
	if err != nil {
		s.Fatal("Failed measuring CPU usage: ", err)
	}

	// Get total played duration while stopping video. If video is played smoothly, we should get the expected duration close to
	// the consuming time of stabilization and CPU usage measurement.
	s.Log("Stopping video and checking duration")
	if err := a.Command(ctx, "input", "keyevent", keyEventStop).Run(testexec.DumpLogOnError); err != nil {
		s.Fatal("Failed stopping video: ", err)
	}
	text, err := d.Object(ui.ID(testLogID)).GetText(ctx)
	if err != nil {
		s.Fatal("Failed to get test log from UI Automator: ", err)
	}
	regexpDuration := regexp.MustCompile(`^stop playing at msec: (\-?[0-9]+)$`)
	matches := regexpDuration.FindAllStringSubmatch(text, -1)
	if len(matches) != 1 {
		s.Fatalf("Found %d duration matches in %q; want 1", len(matches), text)
	}
	// duration will be as milliseconds.
	duration, err := strconv.ParseInt(matches[0][1], 10, 64)
	if err != nil {
		s.Fatalf("Failed to parse duration value %q: %v", matches[0][1], err)
	}
	lowerBound := int64(durationLowerBound.Seconds() * 1000)
	upperBound := int64(durationUpperBound.Seconds() * 1000)
	if duration < lowerBound || duration > upperBound {
		s.Fatalf("Reported video played duration is %d ms, which is outside of expected range [%d, %d]",
			duration, lowerBound, upperBound)
	}

	s.Logf("CPU Usage = %.4f", cpuUsage)
	p := perf.NewValues()
	p.Set(perf.Metric{
		Name:      "cpu_usage",
		Unit:      "percent",
		Direction: perf.SmallerIsBetter,
	}, cpuUsage)
	if err := p.Save(s.OutDir()); err != nil {
		s.Fatal("Failed to save performance metrics: ", err)
	}
}
