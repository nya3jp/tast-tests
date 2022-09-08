// Copyright 2022 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package camera

import (
	"context"
	"strings"
	"time"

	"chromiumos/tast/common/media/caps"
	"chromiumos/tast/ctxutil"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/a11y"
	"chromiumos/tast/local/audio/crastestclient"
	"chromiumos/tast/local/camera/cca"
	"chromiumos/tast/local/chrome/uiauto/nodewith"
	"chromiumos/tast/local/input"
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

	ew, err := input.Keyboard(ctx)
	if err != nil {
		s.Fatal("Failed to create EventWriter from keyboard")
	}
	defer ew.Close()

	visited := make(map[string]bool)
	tab := "Tab"

	// Turning on ChromeVox by keyboard is in a11y/chromevox_toggle_on_shortcut.go
	// Connect to ChromeVox using NewChromeVoxConn.
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

	finder := nodewith.HasClass("shutter")
	if err = cvconn.WaitForFocusedNode(ctx, tconn, finder); err != nil {
		s.Error("Failed to wait for initial ChromeVox focus: ", err)
	}

	for true {
		arialabel, err := app.ReturnFocusedElementAriaLabel(ctx)
		if err != nil {
			s.Fatal("Failed to get a focused node: ", err)
		}

		if visited[arialabel] {
			break
		}

		// There is a case of speaking "+" as "plus" like below.
		// expected: Document scanning now available. Search + Left arrow to access.
		// spoken: Document scanning now available. Search plus Left arrow to access.
		arialabel = strings.Replace(arialabel, "+", "plus", -1)

		if err = sm.Consume(ctx, []a11y.SpeechExpectation{a11y.NewRegexExpectation(arialabel)}); err != nil {
			s.Fatal("Failed to match speeches: ", err)
		}

		visited[arialabel] = true

		if arialabel == "Take photo" {
			if err := takePictureByKeyboard(ctx, sm, app); err != nil {
				s.Fatal("Failed to take a picture: ", err)
			}
		}

		if err = ew.Accel(ctx, tab); err != nil {
			s.Fatal("Failed to press tab key")
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
