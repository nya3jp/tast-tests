// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package ui

import (
	"context"
	"time"

	"chromiumos/tast/common/perf"
	"chromiumos/tast/ctxutil"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/audio"
	"chromiumos/tast/local/audio/crastestclient"
	"chromiumos/tast/local/bundles/cros/ui/cuj"
	"chromiumos/tast/local/bundles/cros/ui/videocuj"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/input"
	"chromiumos/tast/local/power"
	"chromiumos/tast/local/ui/cujrecorder"
	"chromiumos/tast/testing"
	"chromiumos/tast/testing/hwdep"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         BasicYoutubeCUJ,
		LacrosStatus: testing.LacrosVariantUnknown,
		Desc:         "Plays YouTube video and performs basic user actions. Also checks for significant video frame drops and if the audio is being routed through expected device",
		Contacts: []string{
			"ambalavanan.m.m@intel.com",
			"andrescj@google.com",
			"intel-chrome-system-automation-team@intel.com",
			"chromeos-gfx-video@google.com",
		},
		Attr:         []string{"group:mainline", "informational"},
		HardwareDeps: hwdep.D(hwdep.InternalDisplay(), hwdep.Speaker()),
		SoftwareDeps: []string{"chrome"},
		Fixture:      "loggedInAndKeepState",
	})
}

// BasicYoutubeCUJ plays YouTube video and performs basic user actions. Also checks for significant video frame drops and if the audio is being routed through expected device.
func BasicYoutubeCUJ(ctx context.Context, s *testing.State) {
	// Ensure display on to record ui performance correctly.
	if err := power.TurnOnDisplay(ctx); err != nil {
		s.Fatal("Failed to turn on display: ", err)
	}
	var videoSource = videocuj.VideoSrc{
		URL:     "https://www.youtube.com/watch?v=LXb3EKWsInQ",
		Title:   "COSTA RICA IN 4K 60fps HDR (ULTRA HD)",
		Quality: "1080p60",
	}

	// Give 5 seconds to cleanup other resources.
	cleanupCtx := ctx
	ctx, cancel := ctxutil.Shorten(ctx, 5*time.Second)
	defer cancel()

	// Setting the volume to low level.
	cras, err := audio.NewCras(ctx)
	if err != nil {
		s.Fatal("Failed to create Cras object: ", err)
	}

	vh, err := audio.NewVolumeHelper(ctx)
	if err != nil {
		s.Fatal("Failed to create the volumeHelper: ", err)
	}
	originalVolume, err := vh.GetVolume(ctx)
	if err != nil {
		s.Fatal("Failed to get volume: ", err)
	}
	testVol := 10
	s.Logf("Setting Output node volume to %d", testVol)
	if err := vh.SetVolume(ctx, testVol); err != nil {
		s.Errorf("Failed to set output node volume to %d: %v", testVol, err)
	}
	defer vh.SetVolume(cleanupCtx, originalVolume)

	cr := s.FixtValue().(chrome.HasChrome).Chrome()
	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Creating test API connection failed: ", err)
	}

	kb, err := input.VirtualKeyboard(ctx)
	if err != nil {
		s.Fatal("Failed to open the keyboard: ", err)
	}
	defer kb.Close()

	ui := uiauto.New(tconn)

	var uiHandler cuj.UIActionHandler
	if uiHandler, err = cuj.NewClamshellActionHandler(ctx, tconn); err != nil {
		s.Fatal("Failed to create clamshell action handler: ", err)
	}
	defer uiHandler.Close()

	recorder, err := cujrecorder.NewRecorder(ctx, cr, nil, cujrecorder.RecorderOptions{})
	if err != nil {
		s.Fatal("Failed to create a recorder: ", err)
	}
	defer recorder.Close(cleanupCtx)

	if err := recorder.AddCollectedMetrics(tconn, cujrecorder.NewSmoothnessMetricConfig(
		"Ash.WindowCycleView.AnimationSmoothness.Container")); err != nil {
		s.Fatal("Failed to add recorded metrics: ", err)
	}

	extendedDisplay := false
	videoApp := videocuj.NewYtWeb(cr.Browser(), tconn, kb, videoSource, extendedDisplay, ui, uiHandler)

	if err := videoApp.OpenAndPlayVideo(ctx); err != nil {
		s.Fatalf("Failed to open %s: %v", videoSource.URL, err)
	}
	defer videoApp.Close(cleanupCtx)

	if err := videoApp.PerformFrameDropsTest(ctx); err != nil {
		s.Fatal("Failed to play video without frame drops: ", err)
	}

	if err = recorder.Run(ctx, func(ctx context.Context) error {

		if err := videoApp.EnterFullscreen(ctx); err != nil {
			return errors.Wrap(err, "failed to enter full screen")
		}

		if err := videoApp.PauseAndPlayVideo(ctx); err != nil {
			return errors.Wrap(err, "failed to pause and play video")
		}

		if err := videoApp.RestoreWindow(ctx); err != nil {
			return errors.Wrap(err, "failed to restore Youtube window")
		}

		if err := videoApp.MaximizeWindow(ctx); err != nil {
			return errors.Wrap(err, "failed to maximize Youtube window")
		}

		if err := videoApp.MinimizeWindow(ctx); err != nil {
			return errors.Wrap(err, "failed to minimize Youtube window")
		}
		return nil
	}); err != nil {
		s.Fatal("Failed to run recorder: ", err)
	}

	expectedAudioNode := "INTERNAL_SPEAKER"
	// Setting the active node to INTERNAL_SPEAKER if default node is set to some other node.
	if err := cras.SetActiveNodeByType(ctx, expectedAudioNode); err != nil {
		s.Fatalf("Failed to select active device %q: %v", expectedAudioNode, err)
	}
	deviceName, deviceType, err := cras.SelectedOutputDevice(ctx)
	if err != nil {
		s.Fatal("Failed to get the selected audio device: ", err)
	}
	if deviceType != expectedAudioNode {
		s.Fatalf("Failed to set the audio node type: got %q; want %q", deviceType, expectedAudioNode)
	}

	devName, err := crastestclient.FirstRunningDevice(ctx, audio.OutputStream)
	if err != nil {
		s.Fatal("Failed to detect running output device: ", err)
	}

	if deviceName != devName {
		s.Fatalf("Failed to route the audio through expected audio node: got %q; want %q", devName, deviceName)
	}

	pv := perf.NewValues()
	if err = recorder.Record(ctx, pv); err != nil {
		s.Fatal("Failed to report: ", err)
	}
	if err = pv.Save(s.OutDir()); err != nil {
		s.Error("Failed to store values: ", err)
	}
}
