// Copyright 2022 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package spera

import (
	"context"
	"strings"
	"time"

	"chromiumos/tast/common/android/adb"
	"chromiumos/tast/common/android/ui"
	androidui "chromiumos/tast/common/android/ui"
	"chromiumos/tast/common/perf"
	"chromiumos/tast/ctxutil"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/apps"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/ash"
	"chromiumos/tast/local/chrome/browser"
	"chromiumos/tast/local/chrome/cuj"
	"chromiumos/tast/local/chrome/display"
	"chromiumos/tast/local/chrome/googleapps"
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/chrome/uiauto/faillog"
	"chromiumos/tast/local/input"
	"chromiumos/tast/local/mtbf/youtube"
	"chromiumos/tast/local/ui/cujrecorder"
	"chromiumos/tast/testing"
	"chromiumos/tast/testing/hwdep"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         EDUCastToClass,
		LacrosStatus: testing.LacrosVariantExists,
		Desc:         "Measure the performance of casting to a screen connected to ADT-3. Additional chromecast hardware needs to be prepared before running this test",
		Contacts:     []string{"xliu@cienet.com", "alston.huang@cienet.com"},
		SoftwareDeps: []string{"chrome", "arc"},
		HardwareDeps: hwdep.D(hwdep.InternalDisplay()),
		Vars: []string{
			"spera.cuj_mode",     // Optional. Expecting "tablet" or "clamshell". Other values will be be taken as "clamshell".
			"spera.collectTrace", // Optional. Expecting "enable" or "disable", default is "disable".
		},
		Data: []string{cujrecorder.SystemTraceConfigFile},
		Params: []testing.Param{
			{
				Name:    "plus_cast",
				Timeout: 10 * time.Minute,
				Fixture: "enrolledLoggedInToCUJUser",
				Val:     browser.TypeAsh,
			},
			{
				Name:              "plus_lacros_cast",
				Timeout:           10 * time.Minute,
				Fixture:           "enrolledLoggedInToCUJUserLacros",
				ExtraSoftwareDeps: []string{"lacros"},
				Val:               browser.TypeLacros,
			},
		},
	})
}

const (
	// targetResolution specifies the resolution to used for the YouTube video.
	targetResolution = "1080p"
	// accessCodeLength specifies the length of ADT-3 access code.
	accessCodeLength = 6

	// slideTab specifies the tab name for the new Google Slides.
	slideTab = "Google Slides"
	// title specifies the title to use for the new Google Slides.
	title = "Hello class"
	// subtitle specifies the subtitle to use for the new Google Slides.
	subtitle = "Welcome back"
)

var videoSrc = youtube.VideoSrc{
	URL:     cuj.YoutubeDeveloperKeynoteVideoURL,
	Title:   "Developer Keynote (Google I/O '21) - American Sign Language",
	Quality: targetResolution,
}

// EDUCastToClass measures the system performance by casting to a screen connected to ADT-3.
func EDUCastToClass(ctx context.Context, s *testing.State) {
	cr := s.FixtValue().(chrome.HasChrome).Chrome()
	bt := s.Param().(browser.Type)
	outDir := s.OutDir()

	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to connect to test API: ", err)
	}

	// Shorten context a bit to allow for cleanup if Run fails.
	cleanupCtx := ctx
	ctx, cancel := ctxutil.Shorten(ctx, 5*time.Second)
	defer cancel()

	var tabletMode bool
	if mode, ok := s.Var("spera.cuj_mode"); ok {
		tabletMode = mode == "tablet"
		cleanup, err := ash.EnsureTabletModeEnabled(ctx, tconn, tabletMode)
		if err != nil {
			s.Fatalf("Failed to enable tablet mode to %v: %v", tabletMode, err)
		}
		defer cleanup(cleanupCtx)
	} else {
		// Use default screen mode of the DUT.
		tabletMode, err = ash.TabletModeEnabled(ctx, tconn)
		if err != nil {
			s.Fatal("Failed to get DUT default screen mode: ", err)
		}
	}
	s.Log("Running test with tablet mode: ", tabletMode)
	var uiHandler cuj.UIActionHandler
	if tabletMode {
		cleanup, err := display.RotateToLandscape(ctx, tconn)
		if err != nil {
			s.Fatal("Failed to rotate display to landscape: ", err)
		}
		defer cleanup(cleanupCtx)
		if uiHandler, err = cuj.NewTabletActionHandler(ctx, tconn); err != nil {
			s.Fatal("Failed to create tablet action handler: ", err)
		}
	} else {
		if uiHandler, err = cuj.NewClamshellActionHandler(ctx, tconn); err != nil {
			s.Fatal("Failed to create clamshell action handler: ", err)
		}
	}
	defer uiHandler.Close()

	kb, err := input.Keyboard(ctx)
	if err != nil {
		s.Fatal("Failed to initialize keyboard input: ", err)
	}
	defer kb.Close()

	// Give 10 seconds to set initial settings. It is critical to ensure
	// cleanupSetting can be executed with a valid context so it has its
	// own cleanup context from other cleanup functions. This is to avoid
	// other cleanup functions executed earlier to use up the context time.
	cleanupSettingsCtx := ctx
	ctx, cancel = ctxutil.Shorten(ctx, 10*time.Second)
	defer cancel()

	cleanupSetting, err := cuj.InitializeSetting(ctx, tconn)
	if err != nil {
		s.Fatal("Failed to set initial settings: ", err)
	}
	defer cleanupSetting(cleanupSettingsCtx)

	adbDevice, err := adb.WaitForDevice(ctx, func(device *adb.Device) bool { return !strings.HasPrefix(device.Serial, "ACHE-") }, 10*time.Second)
	if err != nil {
		s.Fatal("Failed to list adb devices: ", err)
	}

	device, err := ui.NewDevice(ctx, adbDevice)
	if err != nil {
		s.Fatal("Failed to setup device: ", err)
	}
	defer device.Close(ctx)

	accessCode, err := getAccessCode(ctx, device)
	if err != nil {
		s.Fatal("Failed to get the access code: ", err)
	}
	if len(accessCode) != accessCodeLength {
		s.Fatalf("Length of access code is incorrect; expected: %d; get: %d", accessCodeLength, len(accessCode))
	}

	testing.ContextLog(ctx, "Start to get browser start time")
	l, browserStartTime, err := cuj.GetBrowserStartTime(ctx, tconn, true, tabletMode, bt)
	if err != nil {
		s.Fatal("Failed to get browser start time: ", err)
	}
	br := cr.Browser()
	var bTconn *chrome.TestConn
	if l != nil {
		bTconn, err = l.TestAPIConn(ctx)
		if err != nil {
			s.Fatal("Failed to get lacros test API conn: ", err)
		}
		br = l.Browser()
	}
	ac := uiauto.New(tconn)

	browserApp, err := apps.PrimaryBrowser(ctx, tconn)
	if err != nil {
		s.Fatal("Failed to find the primary browser: ", err)
	}

	youtubeWeb := youtube.NewYtWeb(br, tconn, kb, videoSrc, false, ac, uiHandler)
	defer youtubeWeb.Close(ctx)

	// Shorten the context to clean up the Google Slides created in the test case.
	cleanUpResourceCtx := ctx
	ctx, cancel = ctxutil.Shorten(ctx, 10*time.Second)
	defer cancel()

	defer uiauto.Combine("remove the slide",
		uiHandler.SwitchToAppWindowByName(browserApp.Name, slideTab),
		googleapps.DeleteSlide(tconn),
	)(cleanUpResourceCtx)

	defer faillog.DumpUITreeWithScreenshotOnError(ctx, outDir, s.HasError, cr, "ui_dump")

	// Shorten the context to cleanup cast setting.
	cleanupCastCtx := ctx
	ctx, cancel = ctxutil.Shorten(ctx, 5*time.Second)
	defer cancel()

	defer uiauto.NamedAction("reset cast status", youtubeWeb.ResetCastStatus())(cleanupCastCtx)

	// Shorten the context to cleanup recorder.
	cleanupRecorderCtx := ctx
	ctx, cancel = ctxutil.Shorten(ctx, 5*time.Second)
	defer cancel()

	options := cujrecorder.NewPerformanceCUJOptions()
	recorder, err := cujrecorder.NewRecorder(ctx, cr, bTconn, nil, options)
	if err != nil {
		s.Fatal("Failed to create the recorder: ", err)
	}
	defer recorder.Close(cleanupRecorderCtx)
	if err := cuj.AddPerformanceCUJMetrics(tconn, bTconn, recorder); err != nil {
		s.Fatal("Failed to add metrics to recorder: ", err)
	}
	if collect, ok := s.Var("spera.collectTrace"); ok && collect == "enable" {
		recorder.EnableTracing(s.OutDir(), s.DataPath(cujrecorder.SystemTraceConfigFile))
	}
	pv := perf.NewValues()
	if err = recorder.Run(ctx, func(ctx context.Context) error {
		if err := googleapps.NewGoogleSlides(ctx, tconn, br, uiHandler, false); err != nil {
			return err
		}
		castYoutubeVideo := uiauto.NamedCombine("cast youtube video",
			youtubeWeb.OpenAndPlayVideo,
			youtubeWeb.SwitchQuality(targetResolution),
			youtubeWeb.StartCast(accessCode),
		)
		editSlide := uiauto.NamedCombine("switch back to slide and edit",
			uiHandler.SwitchToAppWindowByName(browserApp.Name, slideTab),
			googleapps.EditSlideTitle(tconn, kb, title, subtitle),
		)
		return uiauto.NamedCombine("cast to class",
			castYoutubeVideo,
			editSlide,
			youtubeWeb.StopCast(),
		)(ctx)
	}); err != nil {
		s.Fatal("Failed to conduct the recorder task: ", err)
	}

	if err := recorder.Record(ctx, pv); err != nil {
		s.Fatal("Failed to record the data: ", err)
	}

	pv.Set(perf.Metric{
		Name:      "Browser.StartTime",
		Unit:      "ms",
		Direction: perf.SmallerIsBetter,
	}, float64(browserStartTime.Milliseconds()))

	if err := pv.Save(outDir); err != nil {
		s.Fatal("Failed to save perf data: ", err)
	}

	if err := recorder.SaveHistograms(outDir); err != nil {
		s.Fatal("Failed to save histogram raw data: ", err)
	}
}

// getAccessCode get the access code from ADT-3 UI.
//
// It locates the access code with the following node hierarchy:
// <node index="0" text="" resource-id="com.google.android.apps.education.cast2class:id/access_code_container_left" class="android.widget.LinearLayout" ...>
//     <node index="0" text="P" resource-id="" class="android.widget.TextView" ...>
//     <node index="1" text="T" resource-id="" class="android.widget.TextView" ...>
//     <node index="2" text="V" resource-id="" class="android.widget.TextView" ...>
// </node>
// <node index="1" text="" resource-id="com.google.android.apps.education.cast2class:id/access_code_container_right" class="android.widget.LinearLayout" ...>
//     <node index="0" text="Q" resource-id="" class="android.widget.TextView" ...>
//     <node index="1" text="G" resource-id="" class="android.widget.TextView" ...>
//     <node index="2" text="R" resource-id="" class="android.widget.TextView" ...>
// </node>

func getAccessCode(ctx context.Context, device *androidui.Device) (accessCode string, err error) {
	var (
		packageName                = "com.google.android.apps.education.cast2class"
		showAccessCodeText         = "Show access code"
		linearLayoutClass          = "android.widget.LinearLayout"
		textViewClass              = "android.widget.TextView"
		accessCodeLeftContainerID  = packageName + ":id/access_code_container_left"
		accessCodeRightContainerID = packageName + ":id/access_code_container_right"
	)

	showAccessCode := device.Object(androidui.Text(showAccessCodeText), androidui.PackageName(packageName))
	if err := cuj.ClickIfExist(showAccessCode, 15*time.Second)(ctx); err != nil {
		return "", errors.Wrap(err, "failed to click Show access code")
	}

	accessCodeLeftContainer := device.Object(androidui.ID(accessCodeLeftContainerID), androidui.ClassName(linearLayoutClass))
	accessCodeRightContainer := device.Object(androidui.ID(accessCodeRightContainerID), androidui.ClassName(linearLayoutClass))
	containers := []*androidui.Object{accessCodeLeftContainer, accessCodeRightContainer}

	testing.ContextLog(ctx, "Get the access code through ADT-3 UI")
	// The access code has two parts, each containing three letters.
	for _, parent := range containers {
		for j := 0; j < 3; j++ {
			letter := device.Object(androidui.Index(j), androidui.ClassName(textViewClass), androidui.PackageName(packageName))
			if err := parent.GetChild(ctx, letter); err != nil {
				return "", errors.Wrapf(err, "failed to get %+v child", parent)
			}
			l, err := letter.GetText(ctx)
			if err != nil {
				return "", errors.Wrap(err, "failed to get the letter")
			}
			accessCode += l
		}
	}

	return accessCode, nil
}
