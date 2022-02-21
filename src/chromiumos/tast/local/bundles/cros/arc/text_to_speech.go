// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package arc

import (
	"context"
	"fmt"
	"time"

	"chromiumos/tast/common/android/adb"
	"chromiumos/tast/common/testexec"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/a11y"
	"chromiumos/tast/local/arc"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         TextToSpeech,
		LacrosStatus: testing.LacrosVariantUnneeded,
		Desc:         "Checks that Android TTS voice is used when chosen",
		Contacts:     []string{"hirokisato@chromium.org", "sahok@chromium.org", "arc-framework+tast@google.com"},
		Attr:         []string{"group:mainline"},
		SoftwareDeps: []string{"chrome"},
		Fixture:      "arcBooted",
		Params: []testing.Param{{
			ExtraSoftwareDeps: []string{"android_p"},
		}, {
			Name:              "vm",
			ExtraSoftwareDeps: []string{"android_vm"},
		}},
	})
}

func TextToSpeech(ctx context.Context, s *testing.State) {
	const (
		apk = "ArcTtsTest.apk"
		pkg = "org.chromium.arc.testapp.tts"

		rate      = 2.0
		volume    = 0.0
		voiceName = "Android org.chromium.arc.testapp.tts.ArcTtsTestService en"
		speakText = "hello world"

		resultFilePath = "/data/user/0/org.chromium.arc.testapp.tts/ttsoutput.txt"
	)

	cr := s.FixtValue().(*arc.PreData).Chrome
	a := s.FixtValue().(*arc.PreData).ARC
	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed creating test API connection: ", err)
	}

	s.Logf("Installing %s", apk)
	if err := a.Install(ctx, arc.APKPath(apk), adb.InstallOptionGrantPermissions); err != nil {
		s.Fatalf("Failed to install %s: %v", apk, err)
	}

	if err := a.Command(ctx, "settings", "put", "secure", "tts_default_synth", pkg).Run(testexec.DumpLogOnError); err != nil {
		s.Fatal("Failed to set TTS engine: ", err)
	}

	if err := testing.Poll(ctx, func(ctx context.Context) error {
		voices, err := a11y.Voices(ctx, tconn)
		if err != nil {
			return errors.Wrap(err, "failed to get voices")
		}

		for _, voice := range voices {
			if voice.Name == voiceName {
				return nil
			}
		}
		return errors.New("TTS engine is not loaded")
	}, &testing.PollOptions{
		Timeout: 15 * time.Second,
	}); err != nil {
		s.Fatal("Failed waiting for TTS engine to load: ", err)
	}

	speakScript := fmt.Sprintf(`new Promise((resolve, reject) => {
		chrome.tts.speak(%q, {
			voiceName: %q,
			volume: %f,
			rate: %f,
			onEvent: function(event) {
				if (event.type === chrome.tts.EventType.END) {
					resolve(event.charIndex);
				}
				if (event.type === chrome.tts.EventType.ERROR) {
					reject(new Error(event.errorMessage));
				}
				if (event.type === chrome.tts.EventType.CANCELLED ||
				    event.type === chrome.tts.EventType.INTERRUPTED) {
					reject(new Error("Unexpected event typpe: " + event.type));
				}
			}},
			function()  {
				if (chrome.runtime.lastError) {
					reject(new Error(chrome.runtime.lastError.message));
				}
			});
	})`, speakText, voiceName, volume, rate)

	var charIndex int
	if err := tconn.Eval(ctx, speakScript, &charIndex); err != nil {
		s.Fatal("Failed to speak: ", err)
	}

	// Validate that Chrome TTS was able to receive last event.
	if charIndex != len(speakText) {
		s.Fatal("Failed to verify all events were dispatched from Android TTS engine")
	}

	actual, err := a.Command(ctx, "run-as", pkg, "cat", resultFilePath).Output(testexec.DumpLogOnError)
	if err != nil {
		s.Fatal("Failed to read TTS output file: ", err)
	}

	outputString := string(actual)
	if outputString != speakText {
		s.Fatalf("TTS output was incorrect; got %q, want %q", outputString, speakText)
	}
}
