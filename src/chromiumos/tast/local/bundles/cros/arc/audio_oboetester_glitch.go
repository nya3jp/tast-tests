// Copyright 2022 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package arc

import (
	"context"
	"regexp"
	"strconv"
	"time"

	"chromiumos/tast/common/android/ui"
	"chromiumos/tast/common/perf"
	"chromiumos/tast/common/testexec"
	"chromiumos/tast/ctxutil"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/arc"
	arcaudio "chromiumos/tast/local/bundles/cros/arc/audio"
	"chromiumos/tast/local/chrome/ash"
	"chromiumos/tast/testing"
)

type audioOboetesterGlitchStressLoad int

const (
	// audioOboetesterGlitchStressLoadNone run the test without any stress test
	audioOboetesterGlitchStressLoadNone audioOboetesterGlitchStressLoad = iota

	// audioOboetesterGlitchStressLoadFull run the test with stressapptest to simulate full load
	audioOboetesterGlitchStressLoadFull
)

type audioOboetesterGlitchParam struct {
	stressMode audioOboetesterGlitchStressLoad
	options    []arc.ActivityStartOption
}

func init() {
	testing.AddTest(&testing.Test{
		Func:         AudioOboetesterGlitch,
		LacrosStatus: testing.LacrosVariantUnneeded,
		Desc:         "Runs Oboetester glitch test for 60 seconds and stores the result",
		Contacts: []string{
			"chromeos-audio-bugs@google.com", // Media team
			"pteerapong@chromium.org",        // Author
		},
		SoftwareDeps: []string{"chrome", "arc"},
		Fixture:      "arcBooted",
		Data:         []string{"oboetester_debug.apk"},
		Attr:         []string{"group:crosbolt", "crosbolt_perbuild"},
		Timeout:      7 * time.Minute,
		Params: []testing.Param{
			{
				Name: "aaudio_noload",
				Val: audioOboetesterGlitchParam{
					stressMode: audioOboetesterGlitchStressLoadNone,
					options: []arc.ActivityStartOption{
						arc.WithExtraString("in_api", "aaudio"),
						arc.WithExtraString("out_api", "aaudio"),
					},
				},
			},
			{
				Name: "aaudio_fullload",
				Val: audioOboetesterGlitchParam{
					stressMode: audioOboetesterGlitchStressLoadFull,
					options: []arc.ActivityStartOption{
						arc.WithExtraString("in_api", "aaudio"),
						arc.WithExtraString("out_api", "aaudio"),
					},
				},
			},
			{
				Name: "opensles_noload",
				Val: audioOboetesterGlitchParam{
					stressMode: audioOboetesterGlitchStressLoadNone,
					options: []arc.ActivityStartOption{
						arc.WithExtraString("in_api", "opensles"),
						arc.WithExtraString("out_api", "opensles"),
					},
				},
			},
			{
				Name: "opensles_fullload",
				Val: audioOboetesterGlitchParam{
					stressMode: audioOboetesterGlitchStressLoadFull,
					options: []arc.ActivityStartOption{
						arc.WithExtraString("in_api", "opensles"),
						arc.WithExtraString("out_api", "opensles"),
					},
				},
			},
		},
	})
}

// AudioOboetesterGlitch runs Oboetester glitch test for 30 seconds and stores the result.
func AudioOboetesterGlitch(ctx context.Context, s *testing.State) {
	const (
		cleanupTime  = 30 * time.Second
		testDuration = 60 // test duration in seconds.

		apkName      = "oboetester_debug.apk"
		pkg          = "com.mobileer.oboetester"
		activityName = ".MainActivity"
	)

	param := s.Param().(audioOboetesterGlitchParam)
	a := s.FixtValue().(*arc.PreData).ARC
	cr := s.FixtValue().(*arc.PreData).Chrome
	d := s.FixtValue().(*arc.PreData).UIDevice

	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to create Test API connection: ", err)
	}

	// Reserve time to remove input file and unload ALSA loopback at the end of the test.
	cleanupCtx := ctx
	ctx, cancel := ctxutil.Shorten(ctx, cleanupTime)
	defer cancel()

	cleanup, err := arcaudio.SetupLoopbackDevice(ctx, cr)
	if err != nil {
		s.Fatal("Failed to setup loopback device: ", err)
	}
	defer cleanup(cleanupCtx)

	testing.ContextLog(ctx, "Install app")
	if err := a.Install(ctx, s.DataPath(apkName)); err != nil {
		s.Fatal("Failed to install app: ", err)
	}
	defer a.Uninstall(cleanupCtx, pkg)

	// Grant permissions and create activity
	if err := a.GrantPermission(ctx, pkg, "android.permission.RECORD_AUDIO"); err != nil {
		s.Fatal("Failed to grant RECORD_AUDIO permission: ", err)
	}
	if err := a.GrantPermission(ctx, pkg, "android.permission.WRITE_EXTERNAL_STORAGE"); err != nil {
		s.Fatal("Failed to grant WRITE_EXTERNAL_STORAGE permission: ", err)
	}
	activity, err := arc.NewActivity(a, pkg, activityName)
	if err != nil {
		s.Fatalf("Failed to create activity %q in package %q: %v", activityName, pkg, err)
	}
	defer activity.Close()

	// Start stress test
	switch param.stressMode {
	case audioOboetesterGlitchStressLoadFull:
		// Run stressapptest for testDuration+3 seconds
		stress := testexec.CommandContext(ctx, "stressapptest", "-s", strconv.Itoa(testDuration+3))
		stress.Start()
		defer stress.Wait()
	}

	// Launch app
	launchParams := param.options
	launchParams = append(launchParams, arc.WithExtraString("test", "glitch"))
	launchParams = append(launchParams, arc.WithExtraInt("duration", testDuration))
	if err := activity.Start(ctx, tconn, launchParams...); err != nil {
		s.Fatalf("Failed to start activity %q in package %q: %v", activityName, pkg, err)
	}
	defer func(ctx context.Context) error {
		// Check that app is still running
		if _, err := ash.GetARCAppWindowInfo(ctx, tconn, activity.PackageName()); err != nil {
			return err
		}
		testing.ContextLogf(ctx, "Stopping activities in package %s", pkg)
		return activity.Stop(ctx, tconn)
	}(cleanupCtx)

	// The test takes at least `testDuration` seconds, so we can sleep and start polling later
	testing.ContextLogf(ctx, "Sleeping for %v seconds to wait for the test to finish", testDuration)
	testing.Sleep(ctx, testDuration*time.Second)

	// Polling until the time total in `time.total = xx.xx seconds` is more than testDuration.
	testing.ContextLog(ctx, "Polling for the test to finish")
	var resultText string
	timeTotalRegex := regexp.MustCompile(`time.total = (\d+\.\d+) seconds`)
	if err := testing.Poll(ctx, func(ctx context.Context) (err error) {
		resultText, err = d.Object(ui.ID("com.mobileer.oboetester:id/text_status")).GetText(ctx)
		if err != nil {
			return err
		}
		match := timeTotalRegex.FindStringSubmatch(resultText)
		if match == nil {
			s.Fatalf("Failed to find time total in result text. Result text = %q", resultText)
		}
		timeTotal, err := strconv.ParseFloat(match[1], 64)
		if err != nil {
			s.Fatalf("Failed to parse time total %q to float: %v", match[1], err)
		}
		if timeTotal < testDuration {
			return errors.Errorf("time total %.2f is less than test duration", timeTotal)
		}
		return nil
	}, &testing.PollOptions{
		Timeout:  1 * time.Minute,
		Interval: 1 * time.Second,
	}); err != nil {
		s.Fatal("Failed to wait for result OK text: ", err)
	}

	// Parse `glitch.count = <number>`
	glitchCountRegex := regexp.MustCompile(`glitch.count = (\d+)`)
	match := glitchCountRegex.FindStringSubmatch(resultText)
	if match == nil {
		s.Fatalf("Failed to find glitch count in result text. Result text = %q", resultText)
	}
	glitchCount, err := strconv.Atoi(match[1])
	if err != nil {
		s.Fatalf("Failed to parse glitch count %q to int: %v", match[1], err)
	}

	// Parse `glitch.frames = <number>`
	glitchFramesRegex := regexp.MustCompile(`glitch.frames = (\d+)`)
	match = glitchFramesRegex.FindStringSubmatch(resultText)
	if match == nil {
		s.Fatalf("Failed to find glitch frames in result text. Result text = %q", resultText)
	}
	glitchFrames, err := strconv.Atoi(match[1])
	if err != nil {
		s.Fatalf("Failed to parse glitch frames %q to int: %v", match[1], err)
	}

	// Stores test result
	perfValues := perf.NewValues()
	defer func() {
		if err := perfValues.Save(s.OutDir()); err != nil {
			s.Error("Cannot save perf data: ", err)
		}
	}()

	perfValues.Set(perf.Metric{
		Name:      "glitch_count",
		Unit:      "times",
		Direction: perf.SmallerIsBetter,
	}, float64(glitchCount))

	perfValues.Set(perf.Metric{
		Name:      "glitch_frames",
		Unit:      "frames",
		Direction: perf.SmallerIsBetter,
	}, float64(glitchFrames))
}
