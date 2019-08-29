// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package arcplayback provides common code for video.ARCPlayback* tests.
package arcplayback

import (
	"context"
	"time"

	"chromiumos/tast/ctxutil"
	"chromiumos/tast/local/arc"
	"chromiumos/tast/local/media/cpu"
	"chromiumos/tast/local/perf"
	"chromiumos/tast/testing"
)

// RunTest runs APK to play video on ARC++ for CPU measurement.
func RunTest(ctx context.Context, s *testing.State, a *arc.ARC, videoName, videoDesc string) {
	const (
		apk = "arc_video_test.apk"
		pkg = "org.chromium.arc.testapp.video"
		cls = ".MainActivity"

		keyEventPlay = "126"
		keyEventStop = "86"

		arcFilePath = "/sdcard/Download/"

		// duration of the interval during which CPU usage will be measured.
		measureDuration = 5 * time.Second
		// time reserved for cleanup.
		cleanupTime = 10 * time.Second
		// time to wait for CPU to stabilize after launching proc.
		stabilize = 1 * time.Second
	)

	cleanUpBenchmark, err := cpu.SetUpBenchmark(ctx)
	if err != nil {
		s.Fatal("Failed to set up benchmark mode: ", err)
	}
	defer cleanUpBenchmark(ctx)

	// Leave a bit of time to clean up benchmark mode.
	ctx, cancel := ctxutil.Shorten(ctx, cleanupTime)
	defer cancel()

	s.Log("Waiting for CPU idle")
	if err := cpu.WaitUntilIdle(ctx); err != nil {
		s.Fatal("Failed waiting for CPU to become idle: ", err)
	}

	s.Log("Installing APK ", apk)
	if err := a.Install(ctx, s.DataPath(apk)); err != nil {
		s.Fatal("Failed installing app: ", err)
	}
	// TODO(johnylin): should we un-install APK?

	s.Log("Granting storage permission")
	if err := a.Command(ctx, "pm", "grant", pkg, "android.permission.READ_EXTERNAL_STORAGE").Run(); err != nil {
		s.Fatal("Failed granting storage permission: ", err)
	}

	s.Log("Pushing video file ", videoName)
	if err := a.PushFile(ctx, s.DataPath(videoName), arcFilePath); err != nil {
		s.Fatal("Failed pushing file: ", err)
	}
	videoPath := arcFilePath + videoName
	defer a.Command(ctx, "rm", videoPath).Run()

	s.Log("Starting APK main activity")
	if err := a.Command(ctx, "am", "start", "--activity-clear-top", "--es", "PATH", videoPath, pkg+"/"+cls).Run(); err != nil {
		s.Fatal("Failed starting APK main activity: ", err)
	}

	// TODO(johnylin): how do we verify video is really played and keeps playing during the CPU measurement?
	s.Log("Playing video")
	if err := a.Command(ctx, "input", "keyevent", keyEventPlay).Run(); err != nil {
		s.Fatal("Failed playing video: ", err)
	}
	defer a.Command(ctx, "input", "keyevent", keyEventStop).Run()

	if err := testing.Sleep(ctx, stabilize); err != nil {
		s.Fatal("Failed waiting for CPU usage to stabilize: ", err)
	}

	s.Log("Measuring CPU usage for ", measureDuration.Round(time.Second))
	cpuUsage, err := cpu.MeasureUsage(ctx, measureDuration)
	if err != nil {
		s.Fatal("Failed measuring CPU usage: ", err)
	}

	s.Logf("CPU Usage of %q = %.4f", videoDesc, cpuUsage)
	p := perf.NewValues()
	p.Set(perf.Metric{
		Name:      "tast_cpu_usage_" + videoDesc,
		Unit:      "percent",
		Direction: perf.SmallerIsBetter,
	}, cpuUsage)
	p.Save(s.OutDir())
}
