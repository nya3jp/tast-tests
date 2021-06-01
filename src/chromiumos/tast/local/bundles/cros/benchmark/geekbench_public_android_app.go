// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package benchmark

import (
	"context"
	"path/filepath"
	"strconv"
	"time"

	"chromiumos/tast/common/perf"
	"chromiumos/tast/errors"
	androidui "chromiumos/tast/local/android/ui"
	"chromiumos/tast/local/apps"
	"chromiumos/tast/local/arc"
	"chromiumos/tast/local/arc/playstore"
	"chromiumos/tast/local/bundles/cros/benchmark/setup"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/ash"
	"chromiumos/tast/local/screenshot"
	"chromiumos/tast/testing"
	"chromiumos/tast/testing/hwdep"
)

const (
	benchMarkTesting = 30 * time.Minute
	geekbenchPkgName = "com.primatelabs.geekbench5"
	activityName     = "com.primatelabs.geekbench.HomeActivity"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:     GeekbenchPublicAndroidApp,
		Desc:     "Execute Geekbench public Android App to do benchmark testing and retrieve the results",
		Contacts: []string{"phuang@cienet.com", "cienet-development@googlegroups.com"},
		// Purposely leave the empty Attr here. Public benchmark tests are not included in crosbolt group for now.
		Attr:         []string{},
		SoftwareDeps: []string{"chrome", "arc"},
		HardwareDeps: hwdep.D(
			hwdep.InternalDisplay(),
			// Since the public benchmark will publish data online, run it only on certain approved models.
			setup.PublicBenchmarkAllowed(),
		),
		Timeout: benchMarkTesting + 5*time.Minute,
		Fixture: setup.BenchmarkARCFixture,
	})
}

func GeekbenchPublicAndroidApp(ctx context.Context, s *testing.State) {
	cr := s.FixtValue().(*arc.PreData).Chrome
	a := s.FixtValue().(*arc.PreData).ARC

	device, err := a.NewUIDevice(ctx)
	if err != nil {
		s.Fatal("Failed to setup ARC device: ", err)
	}
	defer device.Close(ctx)

	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to connect to test API connection: ", err)
	}

	if err := installGeekbench(ctx, tconn, device, a); err != nil {
		s.Fatal("Failed to install Geekbench: ", err)
	}

	if err := openGeekbench(ctx, tconn, device, a); err != nil {
		s.Fatal("Failed to launch Geekbench: ", err)
	}

	const (
		benchMarkRun     = "RUN CPU BENCHMARK"
		benchMarkResults = "Benchmark Results"
		moreOptions      = "More options"
		viewOnline       = "View Online"

		resultPollInterval = 10 * time.Second
	)
	startTime := time.Now() // Geekbench test start time.
	if err := findUIObjAndClick(ctx, device.Object(androidui.TextContains(benchMarkRun)), true); err != nil {
		s.Fatalf("Failed to click %q: %v", benchMarkRun, err)
	}
	// Wait for the geekbench to produce test result.
	if err := testing.Poll(ctx, func(ctx context.Context) error {
		resultLabel := device.Object(androidui.TextContains(benchMarkResults))
		if err := resultLabel.WaitForExists(ctx, time.Second); err != nil {
			s.Logf("Result label not found - geekbench test is still running. Elapsed time: %s", time.Now().Sub(startTime))
			return errors.Wrap(err, "failed to find benchmark result label")
		}
		return nil
	}, &testing.PollOptions{
		Timeout:  benchMarkTesting,
		Interval: resultPollInterval,
	}); err != nil {
		s.Fatal("Failed to run Geekbench: ", err)
	}

	if err := screenshot.CaptureChrome(ctx, cr, filepath.Join(s.OutDir(), "result.png")); err != nil {
		s.Error("Failed to take screenshot: ", err)
	}

	if err := readAndSaveResult(ctx, device, s.OutDir()); err != nil {
		s.Fatal("Failed to reand and save result: ", err)
	}
}

// readAndSaveResult locates the test score from GeekBench UI and saves it as performance value.
//
// It locates the score with the following node hierarchy:
// <node index="0" text="Geekbench Score" resource-id="" class="android.webkit.WebView" ...>
//     <node index="0" text="" resource-id="" ...>
//         <node index="0" text="Geekbench Score" resource-id="" ... />
//         <node index="1" text="" resource-id="" class="android.view.View" ...>
//             <node index="0" text="990" resource-id="" .../>
//             <node index="1" text="Single-Core Score" resource-id="" .../>
//             <node index="2" text="3972" resource-id="" .../>
//             <node index="3" text="Multi-Core Score" resource-id="" .../>
//             ...
//         </node>
//     </node>
// </node>
func readAndSaveResult(ctx context.Context, device *androidui.Device, outputDir string) error {
	root := device.Object(androidui.Text("Geekbench Score"), androidui.ClassName("android.webkit.WebView"))
	if err := root.GetObject(ctx); err != nil {
		return errors.Wrap(err, "failed to locate Geekbench Score web view element")
	}
	nodeIndex0 := device.Object(androidui.Index(0))
	if err := root.GetChild(ctx, nodeIndex0); err != nil {
		return errors.Wrap(err, "failed to locate Geekbench Score second layer child element")
	}
	nodeIndex1 := device.Object(androidui.Index(1))
	if err := nodeIndex0.GetChild(ctx, nodeIndex1); err != nil {
		return errors.Wrap(err, "failed to locate Geekbench Score third layer child element")
	}
	// Next, make sure the score labels are correct.
	nSingleLabel := device.Object(androidui.Index(1), androidui.Text("Single-Core Score"))
	if err := nodeIndex1.GetChild(ctx, nSingleLabel); err != nil {
		return errors.Wrap(err, "failed to locate Geekbench Single-Core Score label")
	}
	nMultiLabel := device.Object(androidui.Index(3), androidui.Text("Multi-Core Score"))
	if err := nodeIndex1.GetChild(ctx, nMultiLabel); err != nil {
		return errors.Wrap(err, "failed to locate Geekbench Multi-Core Score label")
	}
	// Get the score element.
	nSingleScore := device.Object(androidui.Index(0))
	if err := nodeIndex1.GetChild(ctx, nSingleScore); err != nil {
		return errors.Wrap(err, "failed to locate Geekbench Single-Core Score number")
	}
	nMultiScore := device.Object(androidui.Index(2))
	if err := nodeIndex1.GetChild(ctx, nMultiScore); err != nil {
		return errors.Wrap(err, "failed to locate Geekbench Multi-Core Score number")
	}
	singleCoreScore, err := nSingleScore.GetText(ctx)
	if err != nil {
		return errors.Wrap(err, "failed to obtain Geekbench Single-Core Score number")
	}
	multiCoreScore, err := nMultiScore.GetText(ctx)
	if err != nil {
		return errors.Wrap(err, "failed to obtain Geekbench Multi-Core Score number")
	}

	testing.ContextLogf(ctx, "Single-Core Score: %s; Multi-Core Score: %s", singleCoreScore, multiCoreScore)

	sScore, err := strconv.ParseFloat(singleCoreScore, 64)
	if err != nil {
		return errors.Wrap(err, "failed to parse Geekbench single core score")
	}
	mScore, err := strconv.ParseFloat(multiCoreScore, 64)
	if err != nil {
		return errors.Wrap(err, "failed to parse Geekbench multi core score")
	}
	pv := perf.NewValues()
	pv.Set(perf.Metric{
		Name:      "Benchmark.GeekBench.SingleCore",
		Unit:      "score",
		Direction: perf.BiggerIsBetter,
	}, sScore)
	pv.Set(perf.Metric{
		Name:      "Benchmark.GeekBench.MultiCore",
		Unit:      "score",
		Direction: perf.BiggerIsBetter,
	}, mScore)

	if err := pv.Save(outputDir); err != nil {
		return errors.Wrap(err, "failed to store performance values")
	}
	return nil
}

func openGeekbench(ctx context.Context, tconn *chrome.TestConn, device *androidui.Device, ar *arc.ARC) error {
	act, err := arc.NewActivity(ar, geekbenchPkgName, activityName)
	if err != nil {
		return errors.Wrap(err, "failed to create new activity")
	}
	defer act.Close()

	if err = act.Start(ctx, tconn); err != nil {
		return errors.Wrap(err, "failed to start Geekbench")
	}

	if err := ash.WaitForCondition(ctx, tconn, func(w *ash.Window) bool {
		return w.IsActive && w.ARCPackageName == geekbenchPkgName
	}, &testing.PollOptions{Timeout: time.Minute}); err != nil {
		return errors.Wrap(err, "failed to wait for the Geekbench APP window")
	}

	// Click the "ACCEPT" button if it shows up.
	if err := findUIObjAndClick(ctx, device.Object(androidui.TextContains("ACCEPT")), false); err != nil {
		return errors.Wrap(err, "failed to find button ACCEPT and click it")
	}

	return nil
}

func installGeekbench(ctx context.Context, tconn *chrome.TestConn, device *androidui.Device, ar *arc.ARC) error {
	if err := apps.Launch(ctx, tconn, apps.PlayStore.ID); err != nil {
		return errors.Wrap(err, "failed to launch Play Store")
	}

	// Ignore APP close error because it doesn't impact the test logic.
	defer apps.Close(ctx, tconn, apps.PlayStore.ID)

	if err := playstore.InstallApp(ctx, ar, device, geekbenchPkgName, -1); err != nil {
		return errors.Wrapf(err, "failed to install %s", geekbenchPkgName)
	}

	return nil
}

func findUIObjAndClick(ctx context.Context, uiObj *androidui.Object, mandatory bool) error {
	if err := uiObj.WaitForExists(ctx, 5*time.Second); err != nil {
		if !mandatory {
			// If object is not found, just return.
			return nil
		}
		return errors.Wrap(err, "failed to find ui object")
	}
	if err := uiObj.Click(ctx); err != nil {
		return errors.Wrap(err, "failed to click ui object")
	}
	return nil
}
