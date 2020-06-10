// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package arc

import (
	"context"
	"path/filepath"
	"strings"
	"time"

	"chromiumos/tast/common/perf"
	"chromiumos/tast/ctxutil"
	"chromiumos/tast/local/arc"
	"chromiumos/tast/local/arc/c2e2etest"
	"chromiumos/tast/local/media/caps"
	"chromiumos/tast/local/media/cpu"
	"chromiumos/tast/local/power"
	"chromiumos/tast/local/power/setup"
	"chromiumos/tast/testing"
)

const (
	testVideoFile = "1080p_60fps_600frames.vp8.ivf"

	// arcFilePath must be on the sdcard because of android permissions
	arcFilePath = "/sdcard/Download/c2_e2e_test/"

	iterationCount    = 30
	iterationDuration = 10 * time.Second
	warmupDuration    = 10 * time.Second

	logFileName = "gtest_logs.txt"
)

func init() {
	testing.AddTest(&testing.Test{
		Func: PowerVideoPerf,
		Desc: "Measures the battery drain during hardware accelerated 1080p@60fps vp8 video playback",
		Contacts: []string{
			"stevensd@chromium.org",
			"arcvm-eng@google.com",
		},
		SoftwareDeps: []string{"chrome", caps.HWDecodeVP8_60},
		Data:         []string{c2e2etest.X86ApkName, c2e2etest.ArmApkName, testVideoFile, testVideoFile + ".json"},
		Params: []testing.Param{{
			ExtraAttr:         []string{"group:crosbolt", "crosbolt_nightly"},
			ExtraSoftwareDeps: []string{"android_p"},
			Pre:               arc.Booted(),
		}},
		Timeout: 15 * time.Minute,
	})
}

func PowerVideoPerf(ctx context.Context, s *testing.State) {
	// Give cleanup actions a minute to run, even if we fail by exceeding our
	// deadline.
	cleanupCtx := ctx
	ctx, cancel := ctxutil.Shorten(ctx, time.Minute)
	defer cancel()

	cr := s.PreValue().(arc.PreData).Chrome

	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to create Test API connection: ", err)
	}

	a := s.PreValue().(arc.PreData).ARC

	// Parse JSON metadata.
	md, err := c2e2etest.LoadMetadata(s.DataPath(testVideoFile) + ".json")
	if err != nil {
		s.Fatal("Failed to get metadata: ", err)
	}

	apkName, err := c2e2etest.ApkNameForArch(ctx, a)
	if err != nil {
		s.Fatal("Failed to get apk: ", err)
	}

	testVideoDataArg, err := md.StreamDataArg(filepath.Join(arcFilePath, testVideoFile))
	if err != nil {
		s.Fatal("Failed to construct --test_video_data: ", err)
	}

	testArgs := []string{
		testVideoDataArg,
		"--loop",
		"--gtest_filter=C2VideoDecoderSurfaceE2ETest.TestFPS",
	}
	intentExtras := []string{
		"--esa", "test-args", strings.Join(testArgs, ","),
		"--es", "log-file", filepath.Join(arcFilePath, logFileName)}

	sup, cleanup := setup.New("video power")
	defer func() {
		if err := cleanup(cleanupCtx); err != nil {
			s.Error("Cleanup failed: ", err)
		}
	}()

	sup.Add(setup.PowerTest(ctx, tconn, setup.ForceBatteryDischarge))
	sup.Add(setup.InstallApp(ctx, a, s.DataPath(apkName), c2e2etest.Pkg))
	for _, p := range c2e2etest.RequiredPermissions() {
		sup.Add(setup.GrantAndroidPermission(ctx, a, c2e2etest.Pkg, p))
	}

	sup.Add(setup.AdbMkdir(ctx, a, arcFilePath))
	if err := a.PushFile(ctx, s.DataPath(testVideoFile), arcFilePath); err != nil {
		s.Fatal("Failed to push video stream to ARC: ", err)
	}

	s.Log("Waiting until CPU is idle")
	if err := cpu.WaitUntilIdle(ctx); err != nil {
		s.Fatal("Failed to wait until CPU is idle: ", err)
	}

	sup.Add(setup.StartActivity(ctx, tconn, a, c2e2etest.Pkg, c2e2etest.ActivityName, setup.Prefixes("-n"), setup.Suffixes(intentExtras...)))

	if err := sup.Check(ctx); err != nil {
		s.Fatal("Setup failed: ", err)
	}

	metrics, err := perf.NewTimeline(ctx, power.TestMetrics(), perf.Interval(iterationDuration))
	if err != nil {
		s.Fatal("Failed to build metrics: ", err)
	}
	s.Log("Finished setup")

	if err := metrics.Start(ctx); err != nil {
		s.Fatal("Failed to start metrics: ", err)
	}

	s.Log("Warmup: waiting a bit before starting the measurement")
	if err := testing.Sleep(ctx, warmupDuration); err != nil {
		s.Fatal("Failed to sleep: ", err)
	}

	s.Log("Starting measurement")
	if err := metrics.StartRecording(ctx); err != nil {
		s.Fatal("Failed to start recording: ", err)
	}

	if err := testing.Sleep(ctx, iterationCount*iterationDuration); err != nil {
		s.Fatal("Failed to sleep: ", err)
	}

	p, err := metrics.StopRecording()
	if err != nil {
		s.Fatal("Error while recording power metrics: ", err)
	}

	if err := p.Save(s.OutDir()); err != nil {
		s.Error("Failed saving perf data: ", err)
	}

	// TODO(b/152183347): cleanly shut down the test app and parse the logs, to validate that video
	// actually played. For now, failure will just show up as suspiciously low power consumption.
	if err := a.PullFile(ctx, filepath.Join(arcFilePath, logFileName), s.OutDir()); err != nil {
		s.Errorf("Failed fo pull %s: %v", logFileName, err)
	}
}
