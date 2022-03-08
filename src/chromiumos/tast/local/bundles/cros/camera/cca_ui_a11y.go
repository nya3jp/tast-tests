// Copyright 2022 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package camera

import (
	"context"
	"time"

	"chromiumos/tast/common/media/caps"
	"chromiumos/tast/ctxutil"
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
		Params: []testing.Param{{
			Name: "chromevox",
		}},
	})
}

func CCAUIA11y(ctx context.Context, s *testing.State) {
	cr := s.FixtValue().(cca.FixtureData).Chrome

	// Shorten deadline to leave time for cleanup
	ctxCleanup := ctx
	ctx, cancel := ctxutil.Shorten(ctx, time.Second)
	defer cancel()

	// Set Chromevox
	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to create Test API connection: ", err)
	}

	ed := a11y.TTSEngineData{ExtID: a11y.GoogleTTSExtensionID,
		UseOnSpeakWithAudioStream: false}

	// Set a speech monitor
	sm, err := a11y.RelevantSpeechMonitor(ctx, cr, tconn, ed)
	if err != nil {
		s.Fatal("Failed to connect to the TTS background page: ", err)
	}
	defer sm.Close()

	// Mute the device to avoid noisiness.
	if err := crastestclient.Mute(ctx); err != nil {
		s.Fatal("Failed to mute: ", err)
	}
	defer crastestclient.Unmute(ctxCleanup)

	// Enable Chromevox
	ctrlAltZ := []string{"Ctrl+Alt+z"}
	expectedSpeech := []a11y.SpeechExpectation{
		a11y.NewStringExpectation("ChromeVox spoken feedback is ready")}
	// Focused to other node can happen
	a11y.PressKeysAndConsumeExpectations(ctx, sm, ctrlAltZ, expectedSpeech)

	tab := []string{"Tab"}
	expectedSpeech = []a11y.SpeechExpectation{
		a11y.NewStringExpectation("Switch to take photo, radio button selected"),
		a11y.NewStringExpectation("2 of 4"),
		a11y.NewStringExpectation("Camera mode"),
		a11y.NewStringExpectation("Radio button group"),
		a11y.NewStringExpectation("Press Search plus Space to toggle")}
	if !moveAndCheckSpeech(ctx, sm, tab, expectedSpeech, s, 5) {
		s.Fatal("Failed to verify Chromevox keyboard manipulation")
	}
}

func moveAndCheckSpeech(ctx context.Context, sm *a11y.SpeechMonitor, key []string, expectedSpeech []a11y.SpeechExpectation, s *testing.State, try int) bool {
	for i := 1; i <= try; i++ {
		if err := a11y.PressKeysAndConsumeExpectations(ctx, sm, key, expectedSpeech); err == nil {
			return true
		}
	}
	return false
}
