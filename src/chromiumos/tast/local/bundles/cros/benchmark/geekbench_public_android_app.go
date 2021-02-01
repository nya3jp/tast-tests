// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package benchmark

import (
	"context"
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
		Contacts: []string{"tim.chang@cienet.com", "xliu@cienet.com"},
		// Purposely leave the empty Attr here. Public benchmark tests are not included in crosbolt group for now.
		Attr:         []string{},
		SoftwareDeps: []string{"chrome", "arc"},
		HardwareDeps: hwdep.D(
			hwdep.InternalDisplay(),
			// Since the public benchmark will publish data online, run it only on certain approved models.
			setup.PublicBenchmarkAllowed(),
		),
		Timeout: benchMarkTesting,
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

	if err := playstore.InstallApp(ctx, a, device, geekbenchPkgName, -1); err != nil {
		s.Fatalf("Failed to install %s: %v", geekbenchPkgName, err)
	}
	if err := apps.Close(ctx, tconn, apps.PlayStore.ID); err != nil {
		s.Fatal("Failed to close Play Store: ", err)
	}

	if err := openGeekbench(ctx, tconn, device, a); err != nil {
		s.Fatal("Something went wrong when launching Geekbench: ", err)
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
			return errors.Wrap(err, "Benchmark Result label not found")
		}
		return nil
	}, &testing.PollOptions{
		Timeout:  benchMarkTesting,
		Interval: resultPollInterval,
	}); err != nil {
		s.Fatal(err, "Something went wrong when running Geekbench")
	}

	// Locate the test score with the following node hierarchy:
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

	n1 := device.Object(androidui.Text("Geekbench Score"), androidui.ClassName("android.webkit.WebView"))
	if err := n1.GetObject(ctx); err != nil {
		s.Fatal("Failed to locate Geekbench Score web view element: ", err)
	}
	n2 := device.Object(androidui.Index(0))
	if err := n1.GetChild(ctx, n2); err != nil {
		s.Fatal("Failed to locate Geekbench Score second layer child element: ", err)
	}
	n3 := device.Object(androidui.Index(1))
	if err := n2.GetChild(ctx, n3); err != nil {
		s.Fatal("Failed to locate Geekbench Score third layer child element: ", err)
	}
	// Next, make sure the score labels are correct.
	nSingleLabel := device.Object(androidui.Index(1), androidui.Text("Single-Core Score"))
	if err := n3.GetChild(ctx, nSingleLabel); err != nil {
		s.Fatal("Failed to locate Geekbench Single-Core Score label: ", err)
	}
	nMultiLabel := device.Object(androidui.Index(3), androidui.Text("Multi-Core Score"))
	if err := n3.GetChild(ctx, nMultiLabel); err != nil {
		s.Fatal("Failed to locate Geekbench Multi-Core Score label: ", err)
	}
	// Get the score element.
	nSingleScore := device.Object(androidui.Index(0))
	if err := n3.GetChild(ctx, nSingleScore); err != nil {
		s.Fatal("Failed to locate Geekbench Single-Core Score number: ", err)
	}
	nMultiScore := device.Object(androidui.Index(2))
	if err := n3.GetChild(ctx, nMultiScore); err != nil {
		s.Fatal("Failed to locate Geekbench Multi-Core Score number: ", err)
	}
	singleCoreScore, err := nSingleScore.GetText(ctx)
	if err != nil {
		s.Fatal("Failed to obtain Geekbench Single-Core Score number: ", err)
	}
	multiCoreScore, err := nMultiScore.GetText(ctx)
	if err != nil {
		s.Fatal("Failed to obtain Geekbench Multi-Core Score number: ", err)
	}

	s.Logf("Single-Core Score: %s; Multi-Core Score: %s", singleCoreScore, multiCoreScore)

	sScore, err := strconv.ParseFloat(singleCoreScore, 64)
	if err != nil {
		s.Fatal(err, "failed to convert Geekbench single core score")
	}
	mScore, err := strconv.ParseFloat(multiCoreScore, 64)
	if err != nil {
		s.Fatal(err, "failed to convert Geekbench multi core score")
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

	if err := pv.Save(s.OutDir()); err != nil {
		s.Error("Failed to store performance values: ", err)
	}
}

func openGeekbench(ctx context.Context, tconn *chrome.TestConn, device *androidui.Device, ar *arc.ARC) error {
	act, err := arc.NewActivity(ar, geekbenchPkgName, activityName)
	defer act.Close()
	if err != nil {
		return errors.Wrap(err, "failed to create new activity")
	}

	if err = act.Start(ctx, tconn); err != nil {
		return errors.Wrap(err, "failed to start app")
	}

	if err := testing.Poll(ctx, func(ctx context.Context) error {
		return ash.WaitForVisible(ctx, tconn, geekbenchPkgName)
	}, &testing.PollOptions{Timeout: time.Minute}); err != nil {
		return errors.Wrap(err, "failed to wait for the Geekbench APP window")
	}

	// Click the "ACCEPT" button if it shows up.
	if err := findUIObjAndClick(ctx, device.Object(androidui.TextContains("ACCEPT")), false); err != nil {
		return errors.Wrap(err, "failed to find button ACCEPT and click it")
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
