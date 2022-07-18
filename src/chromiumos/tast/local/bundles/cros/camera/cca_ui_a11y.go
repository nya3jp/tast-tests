// Copyright 2022 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package camera

import (
	"context"
	"regexp"
	"time"

	"chromiumos/tast/common/media/caps"
	"chromiumos/tast/ctxutil"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/a11y"
	"chromiumos/tast/local/audio/crastestclient"
	"chromiumos/tast/local/camera/cca"
	"chromiumos/tast/local/chrome/uiauto"
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

	ctrlAltZ := []string{"Ctrl+Alt+z"}
	expectedSpeech := []a11y.SpeechExpectation{a11y.NewRegexExpectation("ChromeVox spoken feedback is ready")}
	// Use the speech monitor to ensure that the spoken announcement was given.
	if err := a11y.PressKeysAndConsumeExpectations(ctx, sm, ctrlAltZ, expectedSpeech); err != nil {
		s.Fatal("Failed to verify Chromevox toggled on: ", err)
	}

	cvconn, err := a11y.NewChromeVoxConn(ctx, cr)
	if err != nil {
		s.Fatal("Failed to connect to the ChromeVox background page: ", err)
	}
	defer cvconn.Close()

	visited := make(map[string]bool)
	tab := []string{"Tab"}

	for true {

		exp, err := a11y.PressKeysAndReturnSpeeches(ctx, sm, tab)
		if err != nil {
			s.Fatal("Failed to consume: ", err)
		}

		fnode, err := cvconn.FocusedNode(ctx, tconn)
		if err != nil {
			s.Fatal("Failed to get a focused node: ", err)
		}

		fnodename := (uiauto.NodeInfo)(*fnode).Name
		matched, err := regexp.MatchString(fnodename, exp)
		if err != nil {
			s.Fatal("Failed to match strings: ", err)
		}

		if visited[fnodename] {
			break
		} else if !matched {
			s.Fatal("Failed to speak expected speeches")
		} else {
			visited[fnodename] = true
		}

		if fnodename == "Take photo" {
			if err := takePictureByKeyboard(ctx, sm, app); err != nil {
				s.Fatal("Failed to take a picture: ", err)
			}
		}
	}
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
