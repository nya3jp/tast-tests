// Copyright 2022 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package arc

import (
	"context"
	"regexp"
	"strconv"
	"strings"
	"time"

	"chromiumos/tast/common/android/ui"
	"chromiumos/tast/common/perf"
	"chromiumos/tast/ctxutil"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/arc"
	arcaudio "chromiumos/tast/local/bundles/cros/arc/audio"
	"chromiumos/tast/local/chrome/ash"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         AudioOboetesterLatency,
		LacrosStatus: testing.LacrosVariantUnneeded,
		Desc:         "Runs oboetester Round Trip Latency test and stores the result",
		Contacts: []string{
			"chromeos-audio-bugs@google.com", // Media team
			"pteerapong@chromium.org",        // Author
		},
		SoftwareDeps: []string{"chrome", "arc"},
		Fixture:      "arcBooted",
		Data:         []string{"oboetester_debug.apk"},
		Attr:         []string{"group:crosbolt", "crosbolt_perbuild"},
		Timeout:      15 * time.Minute,
		Params: []testing.Param{{
			Name: "aaudio",
			Val: []arc.ActivityStartOption{
				arc.WithExtraString("in_api", "aaudio"),
				arc.WithExtraString("out_api", "aaudio"),
			},
		}, {
			Name: "opensles",
			Val: []arc.ActivityStartOption{
				arc.WithExtraString("in_api", "opensles"),
				arc.WithExtraString("out_api", "opensles"),
			},
		}},
	})
}

// AudioOboetesterLatency runs oboetester Round Trip Latency test and stores the result.
func AudioOboetesterLatency(ctx context.Context, s *testing.State) {
	const (
		cleanupTime = 30 * time.Second

		apkName      = "oboetester_debug.apk"
		pkg          = "com.mobileer.oboetester"
		activityName = ".MainActivity"
	)

	param := s.Param().([]arc.ActivityStartOption)
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

	// Launch app
	param = append(param, arc.WithExtraString("test", "latency"))
	if err := activity.Start(ctx, tconn, param...); err != nil {
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

	// The test takes about 1-5 minutes to finish.
	testing.ContextLog(ctx, "Waiting for the test to finish")
	var resultText string
	if err := testing.Poll(ctx, func(ctx context.Context) (err error) {
		resultText, err = d.Object(ui.ID("com.mobileer.oboetester:id/text_status")).GetText(ctx)
		if err != nil {
			return err
		}
		if !strings.Contains(resultText, "result.text = OK") {
			return errors.New("result text not OK")
		}
		return nil
	}, &testing.PollOptions{
		Timeout:  10 * time.Minute, // Take a lot of time on slower machine
		Interval: 5 * time.Second,
	}); err != nil {
		s.Fatal("Failed to wait for result OK text: ", err)
	}

	// Parse `latency.msec = xx.xx`
	latencyRegex := regexp.MustCompile(`latency.msec = (\d+\.\d+)`)
	match := latencyRegex.FindStringSubmatch(resultText)
	if match == nil {
		s.Fatalf("Failed to find latency in result text. Result text = %q", resultText)
	}
	latency, err := strconv.ParseFloat(match[1], 64)
	if err != nil {
		s.Fatalf("Failed to parse latency text %q to float: %v", match[1], err)
	}

	perfValues := perf.NewValues()
	defer func() {
		if err := perfValues.Save(s.OutDir()); err != nil {
			s.Error("Cannot save perf data: ", err)
		}
	}()

	perfValues.Set(
		perf.Metric{
			Name:      "latency",
			Unit:      "ms",
			Direction: perf.SmallerIsBetter,
		}, latency)
}
