// Copyright 2022 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package camera

import (
	"context"
	"time"

	"chromiumos/tast/common/media/caps"
	"chromiumos/tast/ctxutil"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/a11y"
	"chromiumos/tast/local/audio/crastestclient"
	"chromiumos/tast/local/camera/cca"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         CCAUIA11y,
		LacrosStatus: testing.LacrosVariantUnneeded,
		Desc:         "Checks Chromevox reads Chrome Camera App elements as expected",
		Contacts:     []string{"dorahkim@chromium.org", "chromeos-camera-eng@google.com"},
		Attr:         []string{"group:mainline", "informational", "group:camera-libcamera"},
		SoftwareDeps: []string{"camera_app", "chrome", caps.BuiltinOrVividCamera},
		Fixture:      "ccaLaunched",
	})
}

func CCAUIA11y(ctx context.Context, s *testing.State) {
	app := s.FixtValue().(cca.FixtureData).App()
	cr := s.FixtValue().(cca.FixtureData).Chrome

	// Shorten deadline to leave time for cleanup.
	ctxCleanup := ctx
	ctx, cancel := ctxutil.Shorten(ctx, 3*time.Second)
	defer cancel()

	// Mute the device to avoid noisiness.
	if err := crastestclient.Mute(ctx); err != nil {
		s.Fatal("Failed to mute: ", err)
	}
	defer crastestclient.Unmute(ctxCleanup)

	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to create Test API connection: ", err)
	}

	// Get a speech monitor for the Google TTS engine.
	ed := a11y.TTSEngineData{
		ExtID:                     a11y.GoogleTTSExtensionID,
		UseOnSpeakWithAudioStream: false,
	}
	sm, err := a11y.RelevantSpeechMonitor(ctx, cr, tconn, ed)
	if err != nil {
		s.Fatal("Failed to connect to the TTS background page: ", err)
	}
	defer sm.Close()

	// Connect to ChromeVox.
	if err := a11y.SetFeatureEnabled(ctx, tconn, a11y.SpokenFeedback, true); err != nil {
		s.Fatal("Failed to enable spoken feedback: ", err)
	}
	defer func() {
		if err := a11y.SetFeatureEnabled(ctxCleanup, tconn, a11y.SpokenFeedback, false); err != nil {
			s.Error("Failed to disable spoken feedback: ", err)
		}
	}()

	cvconn, err := a11y.NewChromeVoxConn(ctx, cr)
	if err != nil {
		s.Fatal("Failed to connect to the ChromeVox background page: ", err)
	}
	defer cvconn.Close()

	expectedSpeeches1 := []a11y.SpeechExpectation{
		a11y.NewRegexExpectation("ChromeVox spoken feedback is ready"),
		a11y.NewRegexExpectation("Mirror preview"),
		a11y.NewRegexExpectation("Grid"),
		a11y.NewRegexExpectation("Timer duration"),
		a11y.NewRegexExpectation("Take photo"),
	}

	for i := 0; i < len(expectedSpeeches1); i++ {
		if err := moveAroundByKeyboard(ctx, sm, []a11y.SpeechExpectation{expectedSpeeches1[i]}); err != nil {
			s.Fatal("Failed to check all functions: ", err)
		}
	}

	if err := takePictureByKeyboard(ctx, sm, app); err != nil {
		s.Fatal("Failed to take a picture: ", err)
	}

	expectedSpeeches2 := []a11y.SpeechExpectation{
		a11y.NewRegexExpectation("Switch to next camera"),
		a11y.NewRegexExpectation("Switch to take photo"),
		a11y.NewRegexExpectation("Go to Gallery"),
		a11y.NewRegexExpectation("Settings"),
	}

	for i := 0; i < len(expectedSpeeches2); i++ {
		if err := moveAroundByKeyboard(ctx, sm, []a11y.SpeechExpectation{expectedSpeeches2[i]}); err != nil {
			s.Fatal("Failed to check all functions: ", err)
		}
	}
}

func moveAroundByKeyboard(ctx context.Context, sm *a11y.SpeechMonitor, expectedSpeech []a11y.SpeechExpectation) error {
	tab := []string{"Tab"}
	if err := a11y.PressKeysAndConsumeExpectations(ctx, sm, tab, expectedSpeech); err != nil {
		return errors.Wrap(err, "failed to speak expected speech")
	}
	return nil
}

func takePictureByKeyboard(ctx context.Context, sm *a11y.SpeechMonitor, app *cca.App) error {
	dir, err := app.SavedDir(ctx)
	if err != nil {
		return errors.Wrap(err, "failed to get result saved directory")
	}

	start := time.Now()

	space := []string{"Space"}
	if err := a11y.PressKeysAndConsumeExpectations(ctx, sm, space, []a11y.SpeechExpectation{}); err != nil {
		return errors.Wrap(err, "failed to press the shutter button")
	}

	if err := app.WaitForState(ctx, "taking", false); err != nil {
		return errors.Wrap(err, "shutter is not ended")
	}

	if _, err := app.WaitForFileSaved(ctx, dir, cca.PhotoPattern, start); err != nil {
		return errors.Wrap(err, "cannot find captured result file")
	}

	return nil
}
