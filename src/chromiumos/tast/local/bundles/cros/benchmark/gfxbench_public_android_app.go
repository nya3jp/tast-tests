// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package benchmark

import (
	"context"
	"regexp"
	"strconv"
	"time"

	"chromiumos/tast/common/perf"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/android/ui"
	"chromiumos/tast/local/apps"
	"chromiumos/tast/local/arc"
	"chromiumos/tast/local/arc/playstore"
	"chromiumos/tast/local/bundles/cros/benchmark/setup"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/ash"
	"chromiumos/tast/testing"
)

const (
	pollInterval         = 60 * time.Second
	benchmarkTestingTime = 50 * time.Minute
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
		Timeout:      benchmarkTestingTime,
		Fixture:      setup.BenchmarkARCFixture,
	})
}

func GFXBenchPublicAndroidApp(ctx context.Context, s *testing.State) {
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

	if err := playstore.InstallApp(ctx, ar, device, gfxbenchPkgName, -1); err != nil {
		s.Fatalf("Failed to install %s: %v", gfxbenchPkgName, err)
	}
	if err := apps.Close(ctx, tconn, apps.PlayStore.ID); err != nil {
		s.Fatal("Failed to close Play Store: ", err)
	}

	if err := openGfxbench(ctx, tconn, device, ar); err != nil {
		s.Fatal("Something went wrong when launching GFXBench: ", err)
	}

	s.Log("Ready to start benchmark")

	const benchMarkResults = "Best results"

	startTime := time.Now() // GFXBench test start time.
	startBenchmarkButton := device.Object(ui.ID(startBenchmark))
	if err := startBenchmarkButton.WaitForExists(ctx, 5*time.Second); err != nil {
		s.Fatalf("Failed to click %q: %v", startBenchmarkButton, err)
	}
	if err := startBenchmarkButton.Click(ctx); err != nil {
		s.Fatal(err, "Failed to click start benchmark button")
	}

	// Wait for the GFXBench to produce test result.
	if err := testing.Poll(ctx, func(ctx context.Context) error {
		// Due to network errors, app might prompt a dialog to continue download.
		okButton := device.Object(ui.TextContains("OK"))
		if err := okButton.Exists(ctx); err == nil {
			testing.ContextLog(ctx, "Found ok button to click")
			if err := okButton.Click(ctx); err != nil {
				return testing.PollBreak(errors.Wrap(err, "failed to click ok button when benchmark running"))
			}
		}

		resultLabel := device.Object(ui.TextContains(benchMarkResults))
		if err := resultLabel.WaitForExists(ctx, time.Second); err != nil {
			s.Logf("Result label not found - GFXBench test is still running. Elapsed time: %s", time.Now().Sub(startTime))
			return errors.Wrap(err, "benchmark Result label not found")
		}
		return nil
	}, &testing.PollOptions{
		Timeout:  benchmarkTestingTime,
		Interval: pollInterval,
	}); err != nil {
		s.Fatal(err, "Something went wrong when running GFXBench")
	}

	parseBenchmarkOutput := func(ctx context.Context, item *ui.Object, logMessage string) (val string, err error) {
		const (
			wrapperObjectID  = "com.glbenchmark.glbenchmark27:id/updated_result_item_score_wrapper"
			resultObjectID   = "com.glbenchmark.glbenchmark27:id/updated_result_item_subresult"
			fpsRegexpPattern = `\([0-9]+.[\d]+\ Fps\)`
		)
		wrapper := device.Object(ui.ID(wrapperObjectID))
		result := device.Object(ui.ID(resultObjectID))
		if err := item.GetChild(ctx, wrapper); err != nil {
			testing.ContextLogf(ctx, "Failed to find child element: %s", err)
		}
		if err := wrapper.GetChild(ctx, result); err != nil {
			testing.ContextLogf(ctx, "Failed to find child element: %s", err)
		}
		// Get results from AztecRuinsOpenGLNormal(Aztec Ruins OpenGL (normal tier))/AztecRuinsOpenGLNormalOffscreen(1080p Aztec Ruins OpenGL (normal tier) Offscreen)"
		testing.ContextLogf(ctx, "Get %s FPS", logMessage)
		text, err := result.GetText(ctx)
		if err != nil {
			return "", errors.Wrapf(err, "cannot find %s result", logMessage)
		}
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
	if resultList.Exists(ctx); err == nil {
		// Loops through top 6 results to find Normal Tier and Offscreen FPS.
		for i := 1; i < 6; i++ {
			layout := device.Object(ui.ClassName(layoutObjectClassname), ui.Index(i))             // Row.
			item := device.Object(ui.ID(itemObjectID))                                            // Item wrapped by row.
			relativeLayout := device.Object(ui.ClassName(relativeLayoutObjectClassname))          // First cell.
			linearLayout := device.Object(ui.ClassName(linearLayoutObjectClassname), ui.Index(1)) // Linear layout inside first cell.
			text := device.Object(ui.ID(textObjectObjectID))                                      // Label indicates benchmark type.
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
			// Fine benchmark type text.
			if val, err = text.GetText(ctx); err != nil {
				s.Logf("Faild to find label text: %s", err)
			}

			if val == "Aztec Ruins OpenGL (Normal Tier)" {
				// Match Normal Tier.
				tier, err = parseBenchmarkOutput(ctx, item, tierLogMessage)
				if err != nil {
					s.Fatal(err, "Get benchmark Aztec Ruins OpenGL (Normal Tier) result FPS failed")
				}
			} else if val == "1080p Aztec Ruins OpenGL (Normal Tier) Offscreen" {
				// Match Offscreens.
				offscreen, err = parseBenchmarkOutput(ctx, item, offscreenLogMessage)
				if err != nil {
					s.Fatal(err, "Get benchmark 1080p Aztec Ruins OpenGL (Normal Tier) Offscreen result FPS failed")
				}
			}

			if tier != "" && offscreen != "" {
				break
			}
		}
	}

	s.Log("Converting to float")
	normalTierFPS, err := strconv.ParseFloat(tier, 64)
	if err != nil {
		s.Fatal(err, "failed to convert AztecRuinsOpenGLNormal FPS")
	}
	offscreenFPS, err := strconv.ParseFloat(offscreen, 64)
	if err != nil {
		s.Fatal(err, "failed to convert AztecRuinsOpenGLNormalOffscreen FPS")
	}

	pv := perf.NewValues()
	pv.Set(perf.Metric{
		Name:      "Benchmar.GfxBench.AztecRuinsOpenGLNormal",
		Unit:      "FPS",
		Direction: perf.BiggerIsBetter,
	}, normalTierFPS)
	pv.Set(perf.Metric{
		Name:      "Benchmar.GfxBench.AztecRuinsOpenGLNormalOffscreen",
		Unit:      "FPS",
		Direction: perf.BiggerIsBetter,
	}, offscreenFPS)

	if err := pv.Save(s.OutDir()); err != nil {
		s.Error("Failed to store performance values: ", err)
	}
}

func openGfxbench(ctx context.Context, tconn *chrome.TestConn, device *ui.Device, ar *arc.ARC) error {
	act, err := arc.NewActivity(ar, gfxbenchPkgName, gfxActivityName)
	defer act.Close()
	if err != nil {
		return errors.Wrap(err, "failed to create new activity")
	}

	if err = act.Start(ctx, tconn); err != nil {
		return errors.Wrap(err, "failed to start app")
	}

	if err := testing.Poll(ctx, func(ctx context.Context) error {
		return ash.WaitForVisible(ctx, tconn, gfxbenchPkgName)
	}, &testing.PollOptions{Timeout: time.Minute}); err != nil {
		return errors.Wrap(err, "failed to wait for the GFXBench APP window")
	}

	// Click the "Accept" button if it shows up.
	acceptButton := device.Object(ui.TextContains("Accept"))
	if err := acceptButton.WaitForExists(ctx, 30*time.Second); err != nil {
		testing.ContextLog(ctx, "Failed to find Accept button")
	}
	if err := acceptButton.Click(ctx); err != nil {
		testing.ContextLog(ctx, "Failed to click Accept button")
	}

	// Click the "OK" button to connecting GFXBench server.
	okButton := device.Object(ui.TextContains("OK"))
	if err := okButton.WaitForExists(ctx, time.Minute); err != nil {
		testing.ContextLog(ctx, "Failed to find ok button")
	}
	if err := okButton.Click(ctx); err != nil {
		testing.ContextLog(ctx, "Failed to click ok button")
	}

	startTime := time.Now() // GFXBench synchronization start time.
	// Click the "OK" button to synchronize the application data from server.
	okButton = device.Object(ui.TextContains("OK"))
	if err := okButton.WaitForExists(ctx, 30*time.Second); err != nil {
		testing.ContextLog(ctx, "Failed to find download button")
	}
	if err := okButton.Click(ctx); err != nil {
		testing.ContextLog(ctx, "Failed to click download button")
	}

	// Wait for start all button to show up.
	if err := testing.Poll(ctx, func(ctx context.Context) error {
		retryButton := device.Object(ui.TextContains("Retry"))
		if err := retryButton.Exists(ctx); err == nil {
			testing.ContextLog(ctx, "Found retry button to click")
			if err := retryButton.Click(ctx); err != nil {
				return testing.PollBreak(errors.Wrap(err, "failed to click retry button"))
			}
		}

		startButton := device.Object(ui.ID(startBenchmark))
		if err := startButton.WaitForExists(ctx, time.Second); err != nil {
			testing.ContextLogf(ctx, "Start benchmark button has not found - synchronization is still running. Elapsed time: %s", time.Now().Sub(startTime))
			return errors.Wrap(err, "failed to wait for the download data")
		}
		return nil
	}, &testing.PollOptions{
		Timeout:  benchmarkTestingTime,
		Interval: pollInterval,
	}); err != nil {
		return errors.Wrap(err, "something went wrong when running GFXBench")
	}

	return nil
}
