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
	"chromiumos/tast/local/chrome/ui"
	chromeui "chromiumos/tast/local/chrome/ui"
	"chromiumos/tast/local/input"
	"chromiumos/tast/testing"
	"chromiumos/tast/testing/hwdep"
)

const (
	benchMarkTesting = 30 * time.Minute
	geekbenchPkgName = "com.primatelabs.geekbench5"
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
			hwdep.Model("barla", "bluebird", "eve", "krane", "liara", "maple", "pyke", "kohaku"),
		),
		Timeout: benchMarkTesting,
		Fixture: setup.BenchmarkARCFixture,
	})
}

func GeekbenchPublicAndroidApp(ctx context.Context, s *testing.State) {
	cr := s.FixtValue().(*arc.PreData).Chrome
	ar := s.FixtValue().(*arc.PreData).ARC

	device, err := ar.NewUIDevice(ctx)
	if err != nil {
		s.Fatal("Failed to setup ARC device: ", err)
	}
	defer device.Close(ctx)

	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to connect to test API connection: ", err)
	}

	if err := playstore.InstallApp(ctx, ar, device, geekbenchPkgName, -1); err != nil {
		s.Fatalf("Failed to install %s: %v", geekbenchPkgName, err)
	}
	if err := apps.Close(ctx, tconn, apps.PlayStore.ID); err != nil {
		s.Fatal("Failed to close Play Store: ", err)
	}

	keyboard, err := input.Keyboard(ctx)
	if err != nil {
		s.Fatal("Failed to open the keyboard: ", err)
	}
	defer keyboard.Close()

	if err := openGeekbench(ctx, tconn, keyboard, device); err != nil {
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

	// At this time, we view the test result online in the browser and obtain the test score.
	// TODO(tim.chang@cienet.com): find a way to extract the test score from Android UI.
	if err := findUIObjAndClick(ctx, device.Object(androidui.Description(moreOptions)), true); err != nil {
		s.Fatalf("Failed to click %q: %v", moreOptions, err)
	}
	if err := findUIObjAndClick(ctx, device.Object(androidui.TextContains(viewOnline)), true); err != nil {
		s.Fatalf("Failed to click %q: %v", viewOnline, err)
	}
	tab, err := getGeekbenchBrowserTab(ctx, cr, tconn)
	if err != nil {
		s.Fatal("Failed to get Geekbench result tab in the browser: ", err)
	}

	var singleCoreScore, multiCoreScore string
	expr1 := `(() => {
		let collection = document.getElementsByClassName('score-container desktop');
		return collection[0].firstElementChild.innerText
	})()`
	if err := tab.EvalPromise(ctx, expr1, &singleCoreScore); err != nil {
		s.Fatal("Failed to get single core score: ", err)
	}
	expr2 := `(() => {
		let collection = document.getElementsByClassName('score-container desktop');
		return collection[1].firstElementChild.innerText
	})()`
	if err := tab.EvalPromise(ctx, expr2, &multiCoreScore); err != nil {
		s.Fatal("Failed to get multi core score: ", err)
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

func getGeekbenchBrowserTab(ctx context.Context, cr *chrome.Chrome, tconn *chrome.TestConn) (*chrome.Conn, error) {
	query := map[string]interface{}{
		"url": "*://*.geekbench.com/*",
	}
	var tabs []struct {
		URL string `json:"url"`
	}
	l := 0
	// Query browser tabs for geekbench.com.
	if err := testing.Poll(ctx, func(ctx context.Context) error {

		if err := tconn.Call(ctx, &tabs, `tast.promisify(chrome.tabs.query)`, query); err != nil {
			return err
		}

		l = len(tabs)
		if l == 0 {
			return errors.New("Geekbench web page is not found")
		}

		return nil
	}, &testing.PollOptions{Interval: 15 * time.Second}); err != nil {
		return nil, errors.New("cannot find Geekbench test result web page")
	}

	// Assume the last Geekbench tab is the one that was just opened and has the test result.
	conn, err := cr.NewConnForTarget(ctx, chrome.MatchTargetURL(tabs[l-1].URL))
	if err != nil {
		return nil, err
	}

	return conn, nil
}

func openGeekbench(ctx context.Context, tconn *chrome.TestConn, keyboard *input.KeyboardEventWriter, device *androidui.Device) error {
	const queryStr = "geekbench"

	keyboard.Accel(ctx, "search")

	if err := keyboard.Type(ctx, queryStr); err != nil {
		return errors.Wrapf(err, "failed to type %q in search bar", queryStr)
	}
	node, err := ui.FindWithTimeout(ctx, tconn, chromeui.FindParams{ClassName: "SearchResultTileItemView"}, 10*time.Second)
	if err != nil {
		return errors.Wrapf(err, "failed to find Geekbench with search test %q", queryStr)
	}
	defer node.Release(ctx)
	if err := node.StableLeftClick(ctx, &testing.PollOptions{
		Interval: time.Second,
		Timeout:  time.Second * 10,
	}); err != nil {
		return errors.Wrap(err, "failed to click Geekbench APP to launch it")
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
