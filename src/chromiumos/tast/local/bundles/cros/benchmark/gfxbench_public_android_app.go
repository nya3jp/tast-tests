// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package benchmark

import (
	"context"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"

	"chromiumos/tast/common/perf"
	"chromiumos/tast/ctxutil"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/android/ui"
	"chromiumos/tast/local/apps"
	"chromiumos/tast/local/arc"
	"chromiumos/tast/local/arc/playstore"
	"chromiumos/tast/local/bundles/cros/benchmark/setup"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/ash"
	"chromiumos/tast/local/chrome/uiauto/faillog"
	"chromiumos/tast/local/coords"
	"chromiumos/tast/local/screenshot"
	"chromiumos/tast/testing"
	"chromiumos/tast/testing/hwdep"
)

const (
	benchmarkSyncTime    = 30 * time.Minute
	benchmarkTestingTime = 60 * time.Minute
	buttonClass          = "android.widget.Button"
	gfxbenchPkgName      = "com.glbenchmark.glbenchmark27"
	gfxActivityName      = "net.kishonti.app.MainActivity"
	startBenchmark       = "com.glbenchmark.glbenchmark27:id/main_circleControl"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:     GFXBenchPublicAndroidApp,
		Desc:     "Execute GFXBench public Android App to do benchmark testing and retrieve the results",
		Contacts: []string{"phuang@cienet.com", "xliu@cienet.com"},
		// Purposely leave the empty Attr here. Public benchmark tests are not included in crosbolt group for now.
		Attr:         []string{},
		SoftwareDeps: []string{"chrome", "arc"},
		HardwareDeps: hwdep.D(
			hwdep.InternalDisplay(),
			// Since the public benchmark will publish data online, run it only on certain approved models.
			setup.PublicBenchmarkAllowed(),
		),
		Timeout: benchmarkTestingTime,
		Fixture: setup.BenchmarkARCFixture,
	})
}

func GFXBenchPublicAndroidApp(ctx context.Context, s *testing.State) {
	cr := s.FixtValue().(*arc.PreData).Chrome
	ar := s.FixtValue().(*arc.PreData).ARC

	device, err := ar.NewUIDevice(ctx)
	if err != nil {
		s.Fatal("Failed to setup ARC device: ", err)
	}

	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to connect to test API connection: ", err)
	}

	cleanupCtx := ctx
	ctx, cancel := ctxutil.Shorten(ctx, 10*time.Second)
	defer cancel()

	defer func(ctx context.Context) {
		device.Close(ctx)
		faillog.DumpUITreeWithScreenshotOnError(ctx, s.OutDir(), s.HasError, cr, "gfx_benchmark_ui_tree")
		ar.DumpUIHierarchyOnError(ctx, s.OutDir(), s.HasError)
		if w, err := ash.GetARCAppWindowInfo(ctx, tconn, gfxbenchPkgName); err == nil {
			w.CloseWindow(ctx, tconn)
		}
	}(cleanupCtx)

	s.Log("Installing app from play store")
	if err := playstore.InstallApp(ctx, ar, device, gfxbenchPkgName, -1); err != nil {
		s.Fatalf("Failed to install %s: %v", gfxbenchPkgName, err)
	}
	if err := apps.Close(ctx, tconn, apps.PlayStore.ID); err != nil {
		s.Fatal("Failed to close Play Store: ", err)
	}

	boundsInPx, err := openGfxbench(ctx, tconn, device, ar)
	if err != nil {
		s.Fatal("Failed to launch GFXBench: ", err)
	}

	s.Log("Ready to start benchmark")
	if err := startGfxbench(ctx, device, boundsInPx); err != nil {
		s.Fatal("Failed to start the GFXBench test: ", err)
	}

	testing.ContextLog(ctx, "GFXBenchmark is running")
	if err := waitForResult(ctx, device); err != nil {
		s.Fatal("Failed to wait for GFXBench test result: ", err)
	}
	// Take result screenshot which can be checked manually if needed.
	if err := screenshot.CaptureChrome(ctx, cr, filepath.Join(s.OutDir(), "gfx_benchmark_result.png")); err != nil {
		s.Error("Failed to take screenshot: ", err)
	}

	normalTierFPS, offscreenFPS, err := parseBenchmarkResult(ctx, device)
	if err != nil {
		s.Fatal("Failed to parse GFXBench test result: ", err)
	}

	pv := perf.NewValues()
	pv.Set(perf.Metric{
		Name:      "Benchmark.GfxBench.AztecRuinsOpenGLNormal",
		Unit:      "FPS",
		Direction: perf.BiggerIsBetter,
	}, normalTierFPS)
	pv.Set(perf.Metric{
		Name:      "Benchmark.GfxBench.AztecRuinsOpenGLNormalOffscreen",
		Unit:      "FPS",
		Direction: perf.BiggerIsBetter,
	}, offscreenFPS)

	if err := pv.Save(s.OutDir()); err != nil {
		s.Error("Failed to store performance values: ", err)
	}
}

// openGfxbench launches GFXBench app and does data sync with GFXBench server.
// coords.Rect needs to be returned because sometimes it doesn't work when clicking start button
// by UiObject.click() due to NAF Android UI elements.
func openGfxbench(ctx context.Context, tconn *chrome.TestConn, device *ui.Device, ar *arc.ARC) (coords.Rect, error) {
	testing.ContextLog(ctx, "Opening GFXBench APP")

	act, err := arc.NewActivity(ar, gfxbenchPkgName, gfxActivityName)
	defer act.Close()
	if err != nil {
		return coords.Rect{}, errors.Wrap(err, "failed to create new activity")
	}

	if err = act.Start(ctx, tconn); err != nil {
		return coords.Rect{}, errors.Wrap(err, "failed to start app")
	}

	if err := ash.WaitForVisible(ctx, tconn, gfxbenchPkgName); err != nil {
		return coords.Rect{}, errors.Wrap(err, "failed to wait for the GFXBench APP window")
	}

	if err := setup.DismissMobilePrompt(ctx, tconn); err != nil {
		return coords.Rect{}, errors.Wrap(err, "failed to dismiss 'designed for mobile' prompt")
	}
	// Set ARC window to be resizable so it can be maximized.
	if err := setup.SetResizable(ctx, tconn); err != nil {
		return coords.Rect{}, errors.Wrap(err, "failed to set ARC window to be resizable")
	}

	activityBoundsInPx, err := act.WindowBounds(ctx)
	if err != nil {
		return coords.Rect{}, errors.Wrap(err, "failed to get the window bound of the GFXBench APP")
	}
	windowState, err := act.GetWindowState(ctx)
	if err != nil {
		return coords.Rect{}, errors.Wrap(err, "failed to get GFXBench APP's window state")
	}
	// UI elements sometimes could not be found on normal window state.
	// Set window state to maximized to ensure the UI actions works well.
	if windowState != arc.WindowStateMaximized {
		testing.ContextLogf(ctx, "Change window state from %s to maximized", windowState)
		if err := act.SetWindowState(ctx, tconn, arc.WindowStateMaximized); err != nil {
			return coords.Rect{}, errors.Wrap(err, "failed to set GFXBench APP to be maximized")
		}
		if err := ash.WaitForARCAppWindowState(ctx, tconn, gfxbenchPkgName, ash.WindowStateMaximized); err != nil {
			return coords.Rect{}, errors.Wrap(err, "failed to wait for GFXBench APP to be maximized")
		}
	}

	// Click the "Accept" button if it shows up.
	acceptButton := device.Object(ui.Text("Accept"))
	if err := acceptButton.WaitForExists(ctx, 30*time.Second); err != nil {
		testing.ContextLog(ctx, "License page was not found but try to continue: ", err)
	} else {
		testing.ContextLog(ctx, "On license page")
		if err := acceptButton.Click(ctx); err != nil {
			return coords.Rect{}, errors.Wrap(err, "failed to click Accept button")
		}
	}

	// Click the "OK" button to connect GFXBench server.
	info := device.Object(ui.Text("The benchmark only stores and displays results if you have an active internet connection."))
	if err := info.WaitForExists(ctx, 10*time.Second); err != nil {
		testing.ContextLog(ctx, "Information page was not found but try to continue: ", err)
	} else {
		testing.ContextLog(ctx, "On information page")
		okButton := device.Object(ui.Text("OK"), ui.ClassName(buttonClass))
		if err := okButton.Click(ctx); err != nil {
			return coords.Rect{}, errors.Wrap(err, "failed to click OK button")
		}
	}

	// At this time, the APP should be connecting the server, or downloading data. Handle
	// the retry in a go routine.
	ch := make(chan bool)
	go func() {
		for {
			select {
			case <-ch:
				return
			case <-time.After(10 * time.Second):
				retryButton := device.Object(ui.Text("Retry"), ui.ClassName(buttonClass))
				if err := retryButton.Exists(ctx); err == nil {
					testing.ContextLog(ctx, "Click retry button to continue the test")
					if err := retryButton.Click(ctx); err != nil {
						// Continue and re-try next time.
						testing.ContextLog(ctx, "Failed to click retry button: ", err)
					}
				}
			}
		}
	}()
	defer close(ch)

	startTime := time.Now() // GFXBench synchronization start time.
	checkTime := startTime  // Last result checking time.
	// Wait for "start all" button to show up.
	if err := testing.Poll(ctx, func(ctx context.Context) error {
		// Find and click the "OK" button to synchronize the application data from server.
		okButton := device.Object(ui.Text("OK"), ui.ClassName(buttonClass))
		if err := okButton.WaitForExists(ctx, 1*time.Second); err == nil {
			testing.ContextLog(ctx, "On download page")
			if err := okButton.Click(ctx); err != nil {
				return testing.PollBreak(errors.Wrap(err, "failed to click OK button to download"))
			}
		}
		// In some situations APP will jump to results tab instead of home tab.
		// Clicks home tab and then looks for benchmark start icon.
		tabs := device.Object(ui.ID("com.glbenchmark.glbenchmark27:id/tabbar_back"), ui.Index(3))
		if err := tabs.GetObject(ctx); err != nil {
			return errors.Wrap(err, "failed to call GetObject()")
		}
		startButton := device.Object(ui.ID(startBenchmark))
		if err := startButton.WaitForExists(ctx, 3*time.Second); err != nil {
			homeViewTab := device.Object(ui.ClassName("android.view.View"), ui.Index(0))
			// Log and continue if home tab clicking fails.
			if err := tabs.GetChild(ctx, homeViewTab); err != nil {
				testing.ContextLog(ctx, "Failed to find home tab inside tabs element: ", err)
			} else {
				if err := homeViewTab.WaitForExists(ctx, 5*time.Second); err == nil {
					if err := homeViewTab.Click(ctx); err != nil {
						testing.ContextLog(ctx, "Failed to click home tab: ", err)
					}
				}
			}
			nt := time.Now()
			// Log every minute.
			if nt.Sub(checkTime) > time.Minute {
				checkTime = nt
				testing.ContextLog(ctx, "Start benchmark button was not found - synchronization is still running. Elapsed time: ", nt.Sub(startTime))
			}
			return errors.Wrap(err, "failed to wait for data download")
		}
		return nil
	}, &testing.PollOptions{Timeout: benchmarkSyncTime, Interval: time.Second}); err != nil {
		return coords.Rect{}, errors.Wrap(err, "failed to run GFXBench")
	}
	return activityBoundsInPx, nil
}

func startGfxbench(ctx context.Context, device *ui.Device, boundsInPx coords.Rect) error {
	startBenchmarkButton := device.Object(ui.ID(startBenchmark))

	testing.ContextLog(ctx, "Click the start button")
	// Because the start button is rendered by customized code in GFXBench (NAF Android UI),
	// retry with UIObject.click and UIDevice.click to increase the success rate of the clicking.
	// Return with errors directly when start button is not found or fails to be clicked.
	return testing.Poll(ctx, func(ctx context.Context) error {
		if err := startBenchmarkButton.WaitForExists(ctx, 3*time.Second); err != nil {
			return errors.Wrap(err, "failed to find the start button")
		}
		if err := startBenchmarkButton.Click(ctx); err != nil {
			return errors.Wrap(err, "failed to click the start button")
		}
		if err := startBenchmarkButton.WaitUntilGone(ctx, 3*time.Second); err != nil {
			testing.ContextLog(ctx, "Start button was clicked but no response")
			// Try to click on the center point of bounds of GFXBench app
			// if there's no response from UI after the start button was clicked.
			if err := device.Click(ctx, boundsInPx.Width/2, boundsInPx.Height/2); err != nil {
				return errors.Wrap(err, "failed to click start button")
			}
			if err := startBenchmarkButton.WaitUntilGone(ctx, 3*time.Second); err != nil {
				return errors.Wrap(err, "there's no response from UI after the start button was clicked")
			}
		}
		return nil
	}, &testing.PollOptions{Timeout: 2 * time.Minute, Interval: 3 * time.Second})
}

func waitForResult(ctx context.Context, device *ui.Device) error {
	startTime := time.Now() // GFXBench test start time.
	checkTime := startTime  // Last result checking time.
	// Wait for the GFXBench to produce test result.
	return testing.Poll(ctx, func(ctx context.Context) error {
		// Due to network errors, app might prompt a dialog to continue test.
		okButton := device.Object(ui.Text("OK"), ui.ClassName(buttonClass))
		if err := okButton.Exists(ctx); err == nil {
			testing.ContextLog(ctx, "Found ok button to click")
			if err := okButton.Click(ctx); err != nil {
				return testing.PollBreak(errors.Wrap(err, "failed to click ok button when benchmark running"))
			}
		}

		resultLabel := device.Object(ui.TextContains("Best results"))
		if err := resultLabel.WaitForExists(ctx, time.Second); err != nil {
			nt := time.Now()
			// Log every minute.
			if nt.Sub(checkTime) > time.Minute {
				checkTime = nt
				testing.ContextLog(ctx, "Result label not found - GFXBench test is still running. Elapsed time: ", nt.Sub(startTime))
			}
			return errors.Wrap(err, "benchmark Result label not found")
		}
		return nil
	}, &testing.PollOptions{Timeout: benchmarkTestingTime, Interval: time.Second})
}

// parseBenchmarkResultItem parses a particular result UI item.
func parseBenchmarkResultItem(ctx context.Context, device *ui.Device, item *ui.Object, logMessage string) (val string, err error) {
	const (
		wrapperObjectID  = "com.glbenchmark.glbenchmark27:id/updated_result_item_score_wrapper" // Second cell in item body, this contains result FPS.
		resultObjectID   = "com.glbenchmark.glbenchmark27:id/updated_result_item_subresult"     // This is FPS label and the result we wanted.
		fpsRegexpPattern = `([0-9]*[.])?[0-9]+`
	)
	wrapper := device.Object(ui.ID(wrapperObjectID))
	result := device.Object(ui.ID(resultObjectID))
	if err := item.GetChild(ctx, wrapper); err != nil {
		return "", errors.Wrap(err, "failed to find score wrapper element")
	}
	if err := wrapper.GetChild(ctx, result); err != nil {
		return "", errors.Wrap(err, "failed to find score result element")
	}
	// Get results for AztecRuinsOpenGLNormal(Aztec Ruins OpenGL (normal tier)) and AztecRuinsOpenGLNormalOffscreen(1080p Aztec Ruins OpenGL (normal tier) Offscreen).
	testing.ContextLogf(ctx, "Get %s FPS", logMessage)
	text, err := result.GetText(ctx)
	if err != nil {
		return "", errors.Wrapf(err, "cannot find %s result", logMessage)
	}
	testing.ContextLogf(ctx, "Found %s text label: %s", logMessage, text)
	val = regexp.MustCompile(fpsRegexpPattern).FindString(text)
	return val, nil
}

// parseBenchmarkResult parses the benchmark result by analyzing the UI items.
func parseBenchmarkResult(ctx context.Context, device *ui.Device) (float64, float64, error) {
	const (
		resultListObjectID            = "com.glbenchmark.glbenchmark27:id/results_testList"         // This is a list that shows all benchmark results.
		layoutObjectClassname         = "android.widget.LinearLayout"                               // This is a row inside list.
		itemObjectID                  = "com.glbenchmark.glbenchmark27:id/updated_result_item_body" // This is an item body inside a row.
		relativeLayoutObjectClassname = "android.widget.RelativeLayout"                             // First cell in item body, this is used to get label text to get index of row we needed.
		linearLayoutObjectClassname   = "android.widget.LinearLayout"                               // A linear layout to wrap label texts.
		textObjectObjectID            = "com.glbenchmark.glbenchmark27:id/updated_result_item_name" // A Label indicates what result is this. We use this label to match finding.
	)

	var normalTier, offscreen string // The two FPS scores.
	resultList := device.Object(ui.ID(resultListObjectID))
	if err := resultList.GetObject(ctx); err != nil {
		return 0.0, 0.0, errors.Wrap(err, "failed to locate GFXBench result list UI node")
	}

	resultCount, err := resultList.GetChildCount(ctx)
	if err != nil {
		return 0.0, 0.0, errors.Wrap(err, "failed to get child count of result list")
	}

	// Loops through result list to find Normal Tier and Offscreen FPS.
	// Loop terminates if (1) no further row can be found, or (2) the two interested results have been found.
	for i := 1; i <= resultCount; i++ {
		// List Row.
		listRow := device.Object(ui.ClassName(layoutObjectClassname), ui.Index(i))
		if err := resultList.GetChild(ctx, listRow); err != nil {
			testing.ContextLogf(ctx, "Failed to find result row %d inside result list: %v", i, err)
			continue
		}
		// Item wrapped by row.
		item := device.Object(ui.ID(itemObjectID))
		// First cell.
		relativeLayout := device.Object(ui.ClassName(relativeLayoutObjectClassname))
		// Linear layout inside first cell.
		linearLayout := device.Object(ui.ClassName(linearLayoutObjectClassname), ui.Index(1))
		// Label indicates benchmark type.
		text := device.Object(ui.ID(textObjectObjectID))

		if err := listRow.GetChild(ctx, item); err != nil {
			testing.ContextLog(ctx, "Failed to find item body in row element: ", err)
			continue
		}
		if err := item.GetChild(ctx, relativeLayout); err != nil {
			testing.ContextLog(ctx, "Failed to find first cell element: ", err)
			continue
		}
		if err := relativeLayout.GetChild(ctx, linearLayout); err != nil {
			testing.ContextLog(ctx, "Failed to find linear layout in cell element: ", err)
			continue
		}
		if err := linearLayout.GetChild(ctx, text); err != nil {
			testing.ContextLog(ctx, "Failed to find label text element: ", err)
			continue
		}
		// Get benchmark type text.
		val, err := text.GetText(ctx)
		if err != nil {
			testing.ContextLog(ctx, "Faild to get label text: ", err)
			continue
		}

		testing.ContextLogf(ctx, "Result for %s is found", val)
		if val == "Aztec Ruins OpenGL (Normal Tier)" {
			// Match Normal Tier.
			testing.ContextLog(ctx, "Get result for: ", val)

			logMessage := "AztecRuinsOpenGLNormal(Aztec Ruins OpenGL (normal tier))"
			normalTier, err = parseBenchmarkResultItem(ctx, device, item, logMessage)
			if err != nil {
				return 0.0, 0.0, errors.Wrap(err, "failed to get benchmark Aztec Ruins OpenGL (Normal Tier) result")
			}
		} else if strings.Contains(val, "Aztec Ruins OpenGL (Normal Tier) Offscreen") {
			// Match Offscreens.
			testing.ContextLog(ctx, "Get result for: ", val)

			logMessage := "AztecRuinsOpenGLNormalOffscreen(1080p Aztec Ruins OpenGL (normal tier) Offscreen)"
			offscreen, err = parseBenchmarkResultItem(ctx, device, item, logMessage)
			if err != nil {
				return 0.0, 0.0, errors.Wrap(err, "failed to get Aztec Ruins OpenGL (Normal Tier) Offscreen result")
			}
		}

		// Both of the two results have been found.
		if normalTier != "" && offscreen != "" {
			break
		}
	}

	var notFound string
	if normalTier == "" {
		notFound += " [Aztec Ruins OpenGL (Normal Tier)]"
	}
	if offscreen == "" {
		notFound += " [Aztec Ruins OpenGL (Normal Tier) Offscreen]"
	}
	if notFound != "" {
		return 0.0, 0.0, errors.Errorf("failed to find test score for %s", notFound)
	}

	normalTierFPS, err := strconv.ParseFloat(normalTier, 64)
	if err != nil {
		return 0.0, 0.0, errors.Wrap(err, "failed to convert AztecRuinsOpenGLNormal FPS")
	}
	offscreenFPS, err := strconv.ParseFloat(offscreen, 64)
	if err != nil {
		return 0.0, 0.0, errors.Wrap(err, "failed to convert AztecRuinsOpenGLNormalOffscreen FPS")
	}
	return normalTierFPS, offscreenFPS, nil
}
