// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package arc

import (
	"context"
	"fmt"
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
		Timeout:      4 * time.Minute,
		Params: []testing.Param{{
			ExtraSoftwareDeps: []string{"android_p"},
		}, {
			Name:              "vm",
			ExtraSoftwareDeps: []string{"android_vm"},
		}},
	})
}

func AccessibilityTts(ctx context.Context, s *testing.State) {
	const (
		apk           = "ArcTtsTest.apk"
		enginePath    = "org.chromium.arc.ttsengine"
		text          = "hello world"
		ttsOutputPath = "/home/chronos/user/Downloads/ttsoutput.txt"
		voiceName     = "Android org.chromium.arc.ttsengine.ArcTtsService en"
	)

	// ttsVoice corresponds to the ttsVoice definition in chrome/common/extension/api/tts.json.
	type ttsVoice struct {
		VoiceName string `json:"voiceName"`
	}

	cr := s.FixtValue().(*arc.PreData).Chrome
	a := s.FixtValue().(*arc.PreData).ARC
	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed creating test API connection: ", err)
	}

	s.Logf("Installing %s", apk)
	if err := a.Install(ctx, arc.APKPath(apk)); err != nil {
		s.Fatalf("Failed to install %s: %v", apk, err)
	}

	if err := a.Command(ctx, "settings", "put", "secure", "tts_default_synth", enginePath).Run(testexec.DumpLogOnError); err != nil {
		s.Fatal("Failed to enable showing ANRs: ", err)
	}

	var ret []*ttsVoice
	const getVoices = `new Promise((resolve, reject) => {
	chrome.tts.getVoices(
		(voices)=> {
			resolve(voices);}
		)
	})`

	if err := testing.Poll(ctx, func(ctx context.Context) error {
		if err := tconn.Eval(ctx, getVoices, &ret); err != nil {
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

	speakScript := fmt.Sprintf("tast.promisify(chrome.tts.speak(%q, {'voiceName': %q, 'rate': 2.0, 'volume': 0}))", text, voiceName)
	if err := tconn.Eval(ctx, speakScript, nil); err != nil {
		s.Fatal("Failed to speak: ", err)
	}

	if err := testing.Poll(ctx, func(ctx context.Context) error {
		actual, err := ioutil.ReadFile(ttsOutputPath)
		if err != nil {
			return errors.Wrap(err, "failed to read TTS output file")
		}
		outputString := strings.TrimSuffix(string(actual[:]), "\n")
		if outputString != text {
			return errors.Errorf("TTS output was incorrect; got %q, want %q", outputString, text)
		}
		return nil
	}, &testing.PollOptions{
		Timeout: 15 * time.Second,
	}); err != nil {
		s.Fatal("Failed waiting for TTS output: ", err)
	}

}
