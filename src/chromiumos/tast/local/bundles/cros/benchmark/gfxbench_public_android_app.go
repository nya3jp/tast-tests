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
	pollInterval         = 10 * time.Second
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

	cleanupCtx := ctx
	ctx, cancel := ctxutil.Shorten(ctx, 10*time.Second)
	defer cancel()

	defer func(ctx context.Context) {
		device.Close(ctx)
		faillog.SaveScreenshotOnError(ctx, cr, s.OutDir(), func() bool { return true })
		if err := ar.DumpUIHierarchyOnError(ctx, s.OutDir(), func() bool { return true }); err != nil {
			testing.ContextLog(ctx, "Failed to dump ARC UI hierarchy: ", err)
		}
	}(cleanupCtx)

	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to connect to test API connection: ", err)
	}

	if err := playstore.InstallApp(ctx, ar, device, gfxbenchPkgName, -1); err != nil {
		s.Fatalf("Failed to install %s: %v", gfxbenchPkgName, err)
	}
	if err := apps.Close(ctx, tconn, apps.PlayStore.ID); err != nil {
		s.Fatal("Failed to close Play Store: ", err)
	}

	boundsInPx, err := openGfxbench(ctx, s, tconn, device, ar)
	if err != nil {
		s.Fatal("Failed to launch GFXBench: ", err)
	}

	s.Log("Ready to start benchmark")

	const benchMarkResults = "Best results"

	startTime := time.Now() // GFXBench test start time.
	checkTime := startTime  // Last result checking time.
	startBenchmarkButton := device.Object(ui.ID(startBenchmark))

	// Retry for no response after clicked the start button.
	if err := testing.Poll(ctx, func(ctx context.Context) error {
		testing.ContextLog(ctx, "Click the start button")
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
	}, &testing.PollOptions{
		Timeout:  2 * time.Minute,
		Interval: 3 * time.Second,
	}); err != nil {
		s.Fatal("Something went wrong when clicking start button: ", err)
	}

	testing.ContextLog(ctx, "GFXBenchmark is running")

	// Wait for the GFXBench to produce test result.
	if err := testing.Poll(ctx, func(ctx context.Context) error {
		// Due to network errors, app might prompt a dialog to continue test.
		okButton := device.Object(ui.Text("OK"), ui.ClassName(buttonClass))
		if err := okButton.Exists(ctx); err == nil {
			s.Log("Found ok button to click")
			if err := okButton.Click(ctx); err != nil {
				return testing.PollBreak(errors.Wrap(err, "failed to click ok button when benchmark running"))
			}
		}

		resultLabel := device.Object(ui.TextContains(benchMarkResults))
		if err := resultLabel.WaitForExists(ctx, time.Second); err != nil {
			nt := time.Now()
			// Log every minute.
			if nt.Sub(checkTime) > time.Minute {
				checkTime = nt
				s.Logf("Result label not found - GFXBench test is still running. Elapsed time: %s", nt.Sub(startTime))
			}
			return errors.Wrap(err, "benchmark Result label not found")
		}
		return nil
	}, &testing.PollOptions{
		Timeout:  benchmarkTestingTime,
		Interval: pollInterval,
	}); err != nil {
		s.Fatal("Something went wrong when running GFXBench: ", err)
	}

	if err := screenshot.CaptureChrome(ctx, cr, filepath.Join(s.OutDir(), "result.png")); err != nil {
		s.Error("Failed to take screenshot: ", err)
	}

	parseBenchmarkOutput := func(ctx context.Context, item *ui.Object, logMessage string) (val string, err error) {
		const (
			wrapperObjectID  = "com.glbenchmark.glbenchmark27:id/updated_result_item_score_wrapper" // Second cell in item body, this contains result FPS.
			resultObjectID   = "com.glbenchmark.glbenchmark27:id/updated_result_item_subresult"     // This is FPS label and the result we wanted.
			fpsRegexpPattern = `([0-9]*[.])?[0-9]+`
		)
		wrapper := device.Object(ui.ID(wrapperObjectID))
		result := device.Object(ui.ID(resultObjectID))
		if err := item.GetChild(ctx, wrapper); err != nil {
			s.Log("Failed to find score wrapper element: ", err)
		}
		if err := wrapper.GetChild(ctx, result); err != nil {
			s.Log("Failed to find score result element: ", err)
		}
		// Get results from AztecRuinsOpenGLNormal(Aztec Ruins OpenGL (normal tier))/AztecRuinsOpenGLNormalOffscreen(1080p Aztec Ruins OpenGL (normal tier) Offscreen)"
		s.Logf("Get %s FPS", logMessage)
		text, err := result.GetText(ctx)
		if err != nil {
			return "", errors.Wrapf(err, "cannot find %s result", logMessage)
		}
		s.Logf("Found %s text label: %s", logMessage, text)
		var fpsRegexp = regexp.MustCompile(fpsRegexpPattern)
		val = fpsRegexp.FindString(text)
		return val, nil
	}

	const (
		resultListObjectID            = "com.glbenchmark.glbenchmark27:id/results_testList"                                 // This is a list that shows all benchmark results.
		layoutObjectClassname         = "android.widget.LinearLayout"                                                       // This is a row inside list.
		itemObjectID                  = "com.glbenchmark.glbenchmark27:id/updated_result_item_body"                         // This is an item body inside a row.
		relativeLayoutObjectClassname = "android.widget.RelativeLayout"                                                     // First cell in item body, this is used to get label text to get index of row we needed.
		linearLayoutObjectClassname   = "android.widget.LinearLayout"                                                       // A linear layout to wrap label texts.
		textObjectObjectID            = "com.glbenchmark.glbenchmark27:id/updated_result_item_name"                         // A Label indicates what result is this. We use this label to match finding.
		tierLogMessage                = "AztecRuinsOpenGLNormal(Aztec Ruins OpenGL (normal tier))"                          // Const string to match Tier result.
		offscreenLogMessage           = "AztecRuinsOpenGLNormalOffscreen(1080p Aztec Ruins OpenGL (normal tier) Offscreen)" // Const string to match Offscreen result.
	)
	var val, tier, offscreen string
	resultList := device.Object(ui.ID(resultListObjectID))
	if err := resultList.GetObject(ctx); err != nil {
		s.Fatal("Failed to locate GFXBench result list UI node: ", err)
	}
	if resultList.Exists(ctx); err == nil {
		// Loops through top 6 results to find Normal Tier and Offscreen FPS.
		for i := 1; i < 6; i++ {
			// List Row.
			layout := device.Object(ui.ClassName(layoutObjectClassname), ui.Index(i))
			// Item wrapped by row.
			item := device.Object(ui.ID(itemObjectID))
			// First cell.
			relativeLayout := device.Object(ui.ClassName(relativeLayoutObjectClassname))
			// Linear layout inside first cell.
			linearLayout := device.Object(ui.ClassName(linearLayoutObjectClassname), ui.Index(1))
			// Label indicates benchmark type.
			text := device.Object(ui.ID(textObjectObjectID))
			if err := resultList.GetChild(ctx, layout); err != nil {
				s.Logf("Failed to find benchmark row inside result list element: %s", err)
			}
			if err := layout.GetChild(ctx, item); err != nil {
				s.Logf("Failed to find item body in row element: %s", err)
			}
			if err := item.GetChild(ctx, relativeLayout); err != nil {
				s.Logf("Failed to find first cell element: %s", err)
			}
			if err := relativeLayout.GetChild(ctx, linearLayout); err != nil {
				s.Logf("Failed to find linear layout in cell element: %s", err)
			}
			if err := linearLayout.GetChild(ctx, text); err != nil {
				s.Logf("Failed to find label text element: %s", err)
			}
			// Find benchmark type text.
			if val, err = text.GetText(ctx); err != nil {
				s.Logf("Faild to find label text: %s", err)
			}

			s.Logf("Find result type: %s", val)

			if val == "Aztec Ruins OpenGL (Normal Tier)" {
				// Match Normal Tier.
				s.Logf("Get result for: %s", val)

				tier, err = parseBenchmarkOutput(ctx, item, tierLogMessage)
				if err != nil {
					s.Fatal("Get benchmark Aztec Ruins OpenGL (Normal Tier) result FPS failed: ", err)
				}
			} else if strings.Contains(val, "Aztec Ruins OpenGL (Normal Tier) Offscreen") {
				// Match Offscreens.
				s.Logf("Get result for: %s", val)

				offscreen, err = parseBenchmarkOutput(ctx, item, offscreenLogMessage)
				if err != nil {
					s.Fatal("Get benchmark Aztec Ruins OpenGL (Normal Tier) Offscreen result FPS failed: ", err)
				}
			}

			if tier != "" && offscreen != "" {
				break
			}
		}
	}

	var notFound string
	if tier == "" {
		notFound += " [Aztec Ruins OpenGL (Normal Tier)]"
	}
	if offscreen == "" {
		notFound += " [Aztec Ruins OpenGL (Normal Tier) Offscreen]"
	}
	if notFound != "" {
		s.Fatalf("Could not find test score for:%s", notFound)
	}

	s.Log("Converting scores to float")
	normalTierFPS, err := strconv.ParseFloat(tier, 64)
	if err != nil {
		s.Fatal("Failed to convert AztecRuinsOpenGLNormal FPS: ", err)
	}
	offscreenFPS, err := strconv.ParseFloat(offscreen, 64)
	if err != nil {
		s.Fatal("Failed to convert AztecRuinsOpenGLNormalOffscreen FPS: ", err)
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
func openGfxbench(ctx context.Context, s *testing.State, tconn *chrome.TestConn, device *ui.Device, ar *arc.ARC) (coords.Rect, error) {
	s.Log("Opening GFXBench APP")

	act, err := arc.NewActivity(ar, gfxbenchPkgName, gfxActivityName)
	defer act.Close()
	if err != nil {
		return coords.Rect{}, errors.Wrap(err, "failed to create new activity")
	}

	if err = act.Start(ctx, tconn); err != nil {
		return coords.Rect{}, errors.Wrap(err, "failed to start app")
	}

	if err := testing.Poll(ctx, func(ctx context.Context) error {
		return ash.WaitForVisible(ctx, tconn, gfxbenchPkgName)
	}, &testing.PollOptions{Timeout: time.Minute}); err != nil {
		return coords.Rect{}, errors.Wrap(err, "failed to wait for the GFXBench APP window")
	}

	activityBoundsInPx, err := act.WindowBounds(ctx)
	if err != nil {
		return activityBoundsInPx, errors.Wrap(err, "failed to get the window bound of the GFXBench APP")
	}

	windowState, err := act.GetWindowState(ctx)
	if err != nil {
		return activityBoundsInPx, errors.Wrap(err, "failed to get GFXBench APP's window state")
	}
	if windowState != arc.WindowStateMaximized {
		if err := act.SetWindowState(ctx, tconn, arc.WindowStateMaximized); err != nil {
			return activityBoundsInPx, errors.Wrap(err, "failed to set GFXBench APP to be maximized")
		}
		if err := ash.WaitForARCAppWindowState(ctx, tconn, gfxbenchPkgName, ash.WindowStateMaximized); err != nil {
			return activityBoundsInPx, errors.Wrap(err, "failed to wait for GFXBench APP to be maximized")
		}
	}
	testing.ContextLogf(ctx, "current window state: %s", windowState)

	// Click the "Accept" button if it shows up.
	acceptButton := device.Object(ui.Text("Accept"))
	if err := acceptButton.WaitForExists(ctx, 30*time.Second); err != nil {
		s.Log("License page was not found but try to continue: ", err)
	} else {
		s.Log("On license page")
		if err := acceptButton.Click(ctx); err != nil {
			return activityBoundsInPx, errors.Wrap(err, "failed to click Accept button")
		}
	}

	// Click the "OK" button to connect GFXBench server.
	info := device.Object(ui.Text("The benchmark only stores and displays results if you have an active internet connection."))
	if err := info.WaitForExists(ctx, 10*time.Second); err != nil {
		s.Log("Information page was not found but try to continue: ", err)
	} else {
		s.Log("On information page")
		okButton := device.Object(ui.Text("OK"), ui.ClassName(buttonClass))
		if err := okButton.Click(ctx); err != nil {
			return activityBoundsInPx, errors.Wrap(err, "failed to click OK button")
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
					s.Log("Click retry button to contine the test")
					if err := retryButton.Click(ctx); err != nil {
						// Continue and re-try next time.
						s.Log("Failed to click retry button: ", err)
					}
				}
			}
		}
	}()
	defer close(ch)

	startTime := time.Now() // GFXBench synchronization start time.
	checkTime := startTime  // Last result checking time.
	// Wait for start all button to show up.
	if err := testing.Poll(ctx, func(ctx context.Context) error {
		// Find and click the "OK" button to synchronize the application data from server.
		okButton := device.Object(ui.Text("OK"), ui.ClassName(buttonClass))
		if err := okButton.WaitForExists(ctx, 1*time.Second); err == nil {
			s.Log("On download page")
			if err := okButton.Click(ctx); err != nil {
				return testing.PollBreak(errors.Wrap(err, "failed to click OK button to download"))
			}
		}
		// In some situation APP will jump to results tab instead of home tab.
		// We click home tab and then look for benchmark start icon.
		tabs := device.Object(ui.ID("com.glbenchmark.glbenchmark27:id/tabbar_back"), ui.Index(3))
		if err := tabs.GetObject(ctx); err != nil {
			return errors.Wrap(err, "failed to call GetObject()")
		}
		startButton := device.Object(ui.ID(startBenchmark))
		if err := startButton.WaitForExists(ctx, 3*time.Second); err != nil {
			homeViewTab := device.Object(ui.ClassName("android.view.View"), ui.Index(0))
			// Log and continue if home tab clicking fails.
			if err := tabs.GetChild(ctx, homeViewTab); err != nil {
				s.Log("Failed to find home tab inside tabs element: ", err)
			} else {
				if err := homeViewTab.WaitForExists(ctx, 5*time.Second); err == nil {
					if err := homeViewTab.Click(ctx); err != nil {
						s.Log("Failed to click home tab: ", err)
					}
				}
			}
			nt := time.Now()
			// Log every minute.
			if nt.Sub(checkTime) > time.Minute {
				checkTime = nt
				s.Logf("Start benchmark button was not found - synchronization is still running. Elapsed time: %s", nt.Sub(startTime))
			}
			return errors.Wrap(err, "failed to wait for data download")
		}
		return nil
	}, &testing.PollOptions{
		Timeout:  benchmarkSyncTime,
		Interval: pollInterval,
	}); err != nil {
		return activityBoundsInPx, errors.Wrap(err, "failed to run GFXBench")
	}
	return activityBoundsInPx, nil
}
