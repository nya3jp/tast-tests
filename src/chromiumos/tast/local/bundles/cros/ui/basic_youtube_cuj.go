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
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/input"
	"chromiumos/tast/local/power"
	"chromiumos/tast/testing"
	"chromiumos/tast/testing/hwdep"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         BasicYoutubeCUJ,
		Desc:         "Plays YouTube video, performs basic user actions. Also checks for any frame drops and if the audio is routing through expected device",
		Contacts:     []string{"ambalavanan.m.m@intel.com", "intel-chrome-system-automation-team@intel.com"},
		Attr:         []string{"group:mainline", "informational"},
		HardwareDeps: hwdep.D(hwdep.InternalDisplay()),
		SoftwareDeps: []string{"chrome"},
		Fixture:      "loggedInAndKeepState",
	})
}

// BasicYoutubeCUJ plays YouTube video, performs basic user actions. Also checks for any frame drops and if the audio is routing through expected device.
func BasicYoutubeCUJ(ctx context.Context, s *testing.State) {
	// Ensure display on to record ui performance correctly.
	if err := power.TurnOnDisplay(ctx); err != nil {
		s.Fatal("Failed to turn on display: ", err)
	}
	expectedAudioNode := "INTERNAL_SPEAKER"
	var videoSource = videocuj.VideoSrc{
		"https://www.youtube.com/watch?v=LXb3EKWsInQ",
		"COSTA RICA IN 4K 60fps HDR (ULTRA HD)",
		"1440p60",
	}

	// Give 5 seconds to cleanup other resources.
	cleanupCtx := ctx
	ctx, cancel := ctxutil.Shorten(ctx, 5*time.Second)
	defer cancel()

	// Setting the volume to low level.
	cras, err := audio.NewCras(ctx)
	if err != nil {
		s.Error("Failed to create Cras object: ", err)
	}
	setVolume := func(cras *audio.Cras, vol int) error {
		var activeNode audio.CrasNode
		nodes, err := cras.GetNodes(ctx)
		if err != nil {
			return errors.Wrap(err, "failed to get nodes from cras")
		}
		for _, n := range nodes {
			if n.Active && !n.IsInput {
				activeNode = n
			}
		}
		s.Logf("Setting Output node volume to %d", vol)
		if err = cras.SetOutputNodeVolume(ctx, activeNode, vol); err != nil {
			return errors.Wrapf(err, "failed to set output node volume to %d", vol)
		}
		return nil
	}
	if err = setVolume(cras, 10); err != nil {
		s.Error("Failed to reduce output node volume: ", err)
	}
	// Setting back to default volume level.
	defer setVolume(cras, 70)

	cr := s.FixtValue().(cuj.FixtureData).Chrome
	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Creating test API connection failed: ", err)
	}

	kb, err := input.Keyboard(ctx)
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

	var configs []cuj.MetricConfig
	configs = append(configs,
		cuj.NewSmoothnessMetricConfig("Ash.WindowCycleView.AnimationSmoothness.Container"),
	)
	recorder, err := cuj.NewRecorder(ctx, cr, configs...)
	if err != nil {
		s.Fatal("Failed to create a recorder: ", err)
	}
	defer recorder.Close(cleanupCtx)

	extendedDisplay := false
	videoApp := videocuj.NewYtWeb(cr, tconn, kb, videoSource, extendedDisplay, ui, uiHandler)

	if err := videoApp.OpenAndPlayVideo(ctx); err != nil {
		s.Fatalf("Failed to open %s", videoSource)
	}
	defer videoApp.Close(cleanupCtx)

	if err := videoApp.PerformFrameDropsTest(ctx); err != nil {
		s.Error("Failed to play video without frame drops: ", err)
	}

	if err = recorder.Run(ctx, func(ctx context.Context) error {

		if err := videoApp.EnterFullscreen(ctx); err != nil {
			return errors.Errorf("failed to open %q", videoSource.URL)
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
		s.Fatal("Failed: ", err)
	}

	deviceName, deviceType, err := cras.SelectedOutputDevice(ctx)
	if err != nil {
		s.Error("Failed to get the selected audio device: ", err)
	}
	if deviceType != expectedAudioNode {
		s.Errorf("Failed to route the audio via expected node: want %q; got %q", expectedAudioNode, deviceType)
	}

	s.Logf("Selected audio device name: %s", deviceName)
	s.Logf("Selected audio device type: %s", deviceType)

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
