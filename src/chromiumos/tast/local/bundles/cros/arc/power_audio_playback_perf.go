// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package arc

import (
	"context"
	"fmt"
	"strconv"
	"time"

	"chromiumos/tast/common/perf"
	"chromiumos/tast/ctxutil"
	"chromiumos/tast/local/arc"
	"chromiumos/tast/local/bundles/cros/arc/audio"
	"chromiumos/tast/local/power"
	"chromiumos/tast/local/power/setup"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func: PowerAudioPlaybackPerf,
		Desc: "Measures the battery drain during audio playback with different performance flags",
		Contacts: []string{
			"judyhsiao@chromium.org",         // Author
			"cychiang@chromium.org",          // Media team
			"paulhsia@chromium.org",          // Media team
			"chromeos-audio-bugs@google.com", // Media team
		},
		SoftwareDeps: []string{"chrome"},
		Pre:          arc.Booted(),
		Attr:         []string{"group:crosbolt", "crosbolt_nightly"},
		Params: []testing.Param{
			{
				Name: "default",
				Val: audio.TestParameters{
					PerformanceMode: audio.PerformanceModeNone,
				},
				ExtraSoftwareDeps: []string{"android_p"},
			},
			{
				Name: "default_vm",
				Val: audio.TestParameters{
					PerformanceMode: audio.PerformanceModeNone,
				},
				ExtraSoftwareDeps: []string{"android_vm"},
			},
			{
				Name: "low_latency",
				Val: audio.TestParameters{
					PerformanceMode: audio.PerformanceModeLowLatency,
				},
				ExtraSoftwareDeps: []string{"android_p"},
			},
			{
				Name: "low_latency_vm",
				Val: audio.TestParameters{
					PerformanceMode: audio.PerformanceModeLowLatency,
				},
				ExtraSoftwareDeps: []string{"android_vm"},
			},
			{
				Name: "power_saving",
				Val: audio.TestParameters{
					PerformanceMode: audio.PerformanceModePowerSaving,
				},
				ExtraSoftwareDeps: []string{"android_p"},
			},
			{
				Name: "power_saving_vm",
				Val: audio.TestParameters{
					PerformanceMode: audio.PerformanceModePowerSaving,
				},
				ExtraSoftwareDeps: []string{"android_vm"},
			},
		},
		Timeout: 10 * time.Minute,
	})
}

// PowerAudioPlaybackPerf measures the battery drain during audio playback with different performance flags.
func PowerAudioPlaybackPerf(ctx context.Context, s *testing.State) {
	const (
		testActivity           = "org.chromium.arc.testapp.arcaudiotest.PlaybackPerformanceActivity"
		audioWarmupDuration    = 10 * time.Second
		measureDuration        = 60 * time.Second
		keyPerformanceMode     = "perf_mode"
		keyDuration            = "duration"
		playbackDurationSecond = audioWarmupDuration + measureDuration + 10*time.Second // Add 10 seconds buffer.
	)

	param := s.Param().(audio.TestParameters)
	s.Logf("Measuing power consumption of audio playback with flag: %#x", param.PerformanceMode)
	intentExtras := []string{
		"--ei", keyPerformanceMode, strconv.FormatUint(uint64(param.PerformanceMode), 10),
		"--ei", keyDuration, strconv.FormatUint(uint64(playbackDurationSecond/time.Second), 10),
	}

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

	sup, cleanup := setup.New(fmt.Sprintf("audio playback with flag: %#x.", param.PerformanceMode))
	defer func(ctx context.Context) {
		if err := cleanup(ctx); err != nil {
			s.Error("Cleanup failed: ", err)
		}
	}(cleanupCtx)

	sup.Add(setup.PowerTest(ctx, tconn, setup.ForceBatteryDischarge))

	// Install testing app.
	a := s.PreValue().(arc.PreData).ARC
	sup.Add(setup.InstallApp(ctx, a, arc.APKPath(audio.Apk), audio.Pkg))

	// Wait until CPU is cooled down.
	if _, err := power.WaitUntilCPUCoolDown(ctx, power.CoolDownPreserveUI); err != nil {
		s.Fatal("CPU failed to cool down: ", err)
	}

	powerMetrics, err := perf.NewTimeline(ctx, power.TestMetrics(), perf.Interval(measureDuration))
	if err != nil {
		s.Fatal("Failed to build metrics: ", err)
	}

	if err := powerMetrics.Start(ctx); err != nil {
		s.Fatal("Failed to start metrics: ", err)
	}

	// Start testing activity.
	sup.Add(setup.StartActivity(ctx, tconn, a, audio.Pkg, testActivity, setup.Prefixes("-n"), setup.Suffixes(intentExtras...)))
	if err := sup.Check(ctx); err != nil {
		s.Fatal("Setup failed: ", err)
	}

	s.Log("Warmup: Waiting a bit before starting the measurement")
	if err := testing.Sleep(ctx, audioWarmupDuration); err != nil {
		s.Fatal("Failed to sleep: ", err)
	}

	// Keep audio playback and record power usage.
	s.Log("Starting measurement")
	if err := powerMetrics.StartRecording(ctx); err != nil {
		s.Fatal("Failed to start recording: ", err)
	}

	if err := testing.Sleep(ctx, measureDuration); err != nil {
		s.Fatal("Failed to sleep: ", err)
	}

	p, err := powerMetrics.StopRecording()
	if err != nil {
		s.Fatal("Error while recording power metrics: ", err)
	}

	if err := p.Save(s.OutDir()); err != nil {
		s.Error("Failed saving perf data: ", err)
	}
}
