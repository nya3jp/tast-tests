// Copyright 2022 The ChromiumOS Authors.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package assistant

import (
	"context"
	"time"

	"chromiumos/tast/local/apps"
	"chromiumos/tast/local/assistant"
	"chromiumos/tast/local/chrome/ash"
	"chromiumos/tast/local/input/voice"
	"chromiumos/tast/testing"
)

const soundFile2 = "open_settings.wav"

func init() {
	testing.AddTest(&testing.Test{
		Func:         OpenSettingsAudioLocal,
		LacrosStatus: testing.LacrosVariantUnneeded,
		Desc:         "Tests opening the Settings app using an Assistant query with hotword",
		Contacts: []string{
			"chromeos-sw-engprod@google.com",
			"assistive-eng@google.com",
		},
		Data:         []string{soundFile2},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"chrome", "chrome_internal"},
		Fixture:      "assistant",
	})
}

// OpenSettingsAudioLocal tests that the Settings app can be opened by the Assistant
func OpenSettingsAudioLocal(ctx context.Context, s *testing.State) {
	fixtData := s.FixtValue().(*assistant.FixtData)
	cr := fixtData.Chrome

	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Creating test API connection failed: ", err)
	}

	if err := assistant.SetHotwordEnabled(ctx, tconn, true); err != nil {
		s.Fatal("Failed enable Hotword in assistant: ", err)
	}

	// The sound playing from the DUT's speak to trigger assistant only works when DUT is
	// using speakers as the audio output. When a headphone is plugged in, it does not work.
	if err := voice.AudioFromFile(ctx, s.DataPath(soundFile2)); err != nil {
		s.Fatal("Failed to play audio file: ", err)
	}

	s.Log("Launching Settings app with Assistant query and waiting for it to open")

	if err := ash.WaitForApp(ctx, tconn, apps.Settings.ID, time.Minute); err != nil {
		s.Fatalf("Settings app did not appear in the shelf: %v. Last assistant.SendTextQuery error: %v", err, "My Error")
	}
}
