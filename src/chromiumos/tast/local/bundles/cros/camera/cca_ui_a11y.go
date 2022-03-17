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
	"chromiumos/tast/local/chrome/uiauto/faillog"
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
		Fixture:      "ccaTestBridgeReady",
	})
}

func CCAUIA11y(ctx context.Context, s *testing.State) {
	cr := s.FixtValue().(cca.FixtureData).Chrome

	// Shorten deadline to leave time for cleanup.
	ctxCleanup := ctx
	ctx, cancel := ctxutil.Shorten(ctx, time.Second)
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
	defer faillog.DumpUITreeOnError(ctx, s.OutDir(), s.HasError, tconn)

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

	// App launch tests.
	startApp := s.FixtValue().(cca.FixtureData).StartApp
	stopApp := s.FixtValue().(cca.FixtureData).StopApp
	app, err := startApp(ctx)
	if err != nil {
		s.Fatal("Failed to open CCA: ", err)
	}
	defer func(cleanupCtx context.Context) {
		if err := stopApp(cleanupCtx, s.HasError()); err != nil {
			s.Fatal("Failed to close CCA: ", err)
		}
	}(ctxCleanup)

	ctrlAltZ := []string{"Ctrl+Alt+z"}
	expectedSpeech := []a11y.SpeechExpectation{a11y.NewRegexExpectation("*Camera mode*")}
	if err := a11y.PressKeysAndConsumeExpectations(ctx, sm, ctrlAltZ, expectedSpeech); err != nil {
		s.Log("Failed to enable Chromevox or the different mention is spoken: ", err)
	}

	tab := []string{"Tab"}
	expectedSpeech = []a11y.SpeechExpectation{a11y.NewRegexExpectation("*Button*")}
	if err := a11y.PressKeysAndConsumeExpectations(ctx, sm, tab, expectedSpeech); err != nil {
		s.Fatal("Failed to focus on the shutter button: ", err)
	}

	if err := takePicture(ctx, sm, s, app); err != nil {
		s.Fatal("Failed to take a Picture: ", err)
	}

	defer func() {
		a11y.PressKeysAndConsumeExpectations(ctx, sm, ctrlAltZ, []a11y.SpeechExpectation{})
	}()
}

func takePicture(ctx context.Context, sm *a11y.SpeechMonitor, s *testing.State, app *cca.App) error {
	dir, err := app.SavedDir(ctx)
	if err != nil {
		return errors.Wrap(err, "failed to get result saved directory")
	}

	start := time.Now()

	space := []string{"Space"}
	if err := a11y.PressKeysAndConsumeExpectations(ctx, sm, space, []a11y.SpeechExpectation{}); err != nil {
		return errors.Wrap(err, "failed to press the shutter button")
	}

	if _, err := app.WaitForFileSaved(ctx, dir, cca.PhotoPattern, start); err != nil {
		return errors.Wrap(err, "cannot find captured result file")
	}
	if err := app.WaitForState(ctx, "taking", false); err != nil {
		return errors.Wrap(err, "shutter is not ended")
	}

	return nil
}
