// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package arc

import (
	"context"
	"io/ioutil"
	"strings"
	"time"

	"chromiumos/tast/common/testexec"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/arc"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         AccessibilityTts,
		Desc:         "Checks that Android TTS voice is used when chosen",
		Contacts:     []string{"sarakato@chromium.org", "dtseng@chromium.org", "hirokisato@chromium.org", "arc-framework+tast@google.com"},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"chrome"},
		Fixture:      "arcBooted",
		Timeout:      6 * time.Minute,
		Data:         []string{"TtsEngine.apk"}, // see if we need to prefix this with data
		Params: []testing.Param{{
			ExtraSoftwareDeps: []string{"android_p"},
		}, {
			Name:              "vm",
			ExtraSoftwareDeps: []string{"android_vm"},
		}},
	})
}

// TtsVoice corresponds to the ttsVoice definition in chrome/common/extension/api/tts.json.
type TtsVoice struct {
	VoiceName string `json:"voiceName"`
}

func AccessibilityTts(ctx context.Context, s *testing.State) {
	const (
		text       = "hello world"
		voiceName  = "Android Example TTS Engine en"
		enginePath = "com.example.android.ttsengine"
	)

	cr := s.FixtValue().(*arc.PreData).Chrome
	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed creating test API connection: ", err)
	}
	//p := s.FixtValue().(*arc.PreData)
	a := s.FixtValue().(*arc.PreData).ARC

	//a := p.ARC

	//arc.APKPath("ArcTtsTest.apk")
	//a.Install(ctx, s.DataPath(apkName)); err != nil {
	if err := a.Install(ctx, s.DataPath("TtsEngine.apk")); err != nil {
		s.Fatal("Failed to install the APK: ", err)
	}

	if err := a.Command(ctx, "settings", "put", "secure", "tts_default_synth", "com.example.android.ttsengine").Run(testexec.DumpLogOnError); err != nil {
		s.Fatal("Failed to enable showing ANRs: ", err)
	}

	var ret []*TtsVoice
	const code = `new Promise((resolve, reject) => {
	chrome.tts.getVoices(
		(voices)=> {
			resolve(voices);}
		)
	})`

	if err := testing.Poll(ctx, func(ctx context.Context) error {
		if err := tconn.Eval(ctx, code, &ret); err != nil {
			// Do error handling here.
			return err
		}

		for _, voice := range ret {
			if voice.VoiceName == voiceName {
				return nil
			}
		}
		return errors.New("TTS engine is not loaded")
	}, &testing.PollOptions{
		Timeout: 15 * time.Second,
	}); err != nil {
		s.Fatal("Failed waiting for TTS engine to load: ", err)
	}

	if err := tconn.Eval(ctx, `tast.promisify(chrome.tts.speak("hello world", {"voiceName": "Android Example TTS Engine en", "rate": 2.0, "volume": 0}))`, nil); err != nil {
		s.Fatal("Failed to speak: ", err)
	}

	if err := testing.Poll(ctx, func(ctx context.Context) error {
		crosPath := "/home/chronos/user/Downloads/ttsoutput.txt"
		actual, err := ioutil.ReadFile(crosPath)
		if err != nil {
			return errors.Wrap(err, "failed to read TTS output file")
		}
		outputString := strings.TrimSuffix(string(actual[:]), "\n")
		if outputString != "hello world" {
			return errors.Errorf("TTS output was incorrect; got %q, want %q", outputString, "hello world")
		}
		return nil
	}, &testing.PollOptions{
		Timeout: 15 * time.Second,
	}); err != nil {
		s.Fatal("Failed waiting for TTS output: ", err)
	}

}
