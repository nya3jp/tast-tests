// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package arc

import (
	"context"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
	"time"

	"chromiumos/tast/common/android/adb"
	"chromiumos/tast/common/testexec"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/a11y"
	"chromiumos/tast/local/arc"
	"chromiumos/tast/local/cryptohome"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         TextToSpeech,
		Desc:         "Checks that Android TTS voice is used when chosen",
		Contacts:     []string{"sarakato@chromium.org", "sahok@chromium.org", "hirokisato@chromium.org", "arc-framework+tast@google.com"},
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
		apk           = "ArcTtsTest.apk"
		enginePackage = "org.chromium.arc.testapp.tts"
		rate          = 2.0
		speakText     = "hello world"
		voiceName     = "Android org.chromium.arc.testapp.tts.ArcTtsTestService en"
		volume        = 0.0
		debugFilePath = "files-under-cryptohome.txt"
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

	if err := a.Command(ctx, "settings", "put", "secure", "tts_default_synth", enginePackage).Run(testexec.DumpLogOnError); err != nil {
		s.Fatal("Failed to set TTS engine: ", err)
	}

	if err := testing.Poll(ctx, func(ctx context.Context) error {
		voices, err := a11y.GetVoices(ctx, tconn)
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
				if (event.type === chrome.tts.EventType.CANCELLED ||
				    event.type === chrome.tts.EventType.ERROR ||
				    event.type === chrome.tts.EventType.INTERRUPTED) {
					reject();
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

	cryptohomeUserPath, err := cryptohome.UserPath(ctx, cr.User())
	if err != nil {
		s.Fatalf("Failed to get the cryptohome user path for %s: %v", cr.User(), err)
	}
	targetPathInCros := filepath.Join(cryptohomeUserPath, "MyFiles", "Downloads", "ttsoutput.txt")

	actual, err := ioutil.ReadFile(targetPathInCros)
	if err != nil {
		var b strings.Builder
		if err := filepath.Walk(cryptohomeUserPath, func(path string, info os.FileInfo, err error) error {
			fmt.Fprintf(&b, "%s %+v\n", path, info)
			if err != nil {
				s.Error("Error on walking files: ", err)
			}
			return nil
		}); err != nil {
			s.Errorf("Failed to walk files under %q: %v", cryptohomeUserPath, err)
		}
		path := filepath.Join(s.OutDir(), debugFilePath)
		s.Logf("Writing a list of files under %q to %q", cryptohomeUserPath, debugFilePath)
		if writeErr := ioutil.WriteFile(path, []byte(b.String()), 0644); writeErr != nil {
			s.Error("Error on writing a file list: ", err)
		}
		s.Fatal("Failed to read TTS output file: ", err)
	}

	outputString := string(actual)
	if outputString != speakText {
		s.Fatalf("TTS output was incorrect; got %q, want %q", outputString, speakText)
	}
}
