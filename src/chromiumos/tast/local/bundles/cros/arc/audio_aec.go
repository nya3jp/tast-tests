// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package arc

import (
	"context"
	"time"

	"chromiumos/tast/local/arc"
	"chromiumos/tast/local/bundles/cros/arc/audio"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func: AudioAEC,
		Desc: "Audio AEC test for arc",
		Contacts: []string{
			"chromeos-audio-bugs@google.com", // Media team
			"cychiang@chromium.org",          // Media team
			"paulhsia@chromium.org",          // Media team
			"judyhsiao@chromium.org",         // Author
		},
		SoftwareDeps: []string{"chrome", "android_vm"},
		Data:         []string{"ARCAudioTest.apk"},
		Attr:         []string{"group:mainline", "informational"},
		Timeout:      2 * time.Minute,
		Pre:          arc.VMBooted(),
	})
}

// AudioAEC runs audio AEC tests.
func AudioAEC(ctx context.Context, s *testing.State) {
	const AEC = "0x0001"

	a := s.PreValue().(arc.PreData).ARC
	cr := s.PreValue().(arc.PreData).Chrome
	param := audio.TestParameters{
		Permission: "android.permission.RECORD_AUDIO",
		Class:      "org.chromium.arc.testapp.arcaudiotestapp.TestAECEffectActivity",
	}

	atast, err := audio.NewArcAudioTast(ctx, a, cr)
	if err != nil {
		s.Fatal("Failed to NewArcAudioTast: ", err)
	}
	streams, err := atast.RunAppTestAndPollStream(ctx, s.DataPath(audio.Apk), param)
	if err != nil {
		s.Error("Test failed: ", err)
	}
	if len(streams) > 1 {
		s.Errorf("Too many active streams, found %d streams ", len(streams))
	}
	// Verifies that there is an input stream.
	dir, ok := streams[0]["direction"]
	if !ok || dir != "Input" {
		s.Error("An output stream is detected")
	}
	// Verifies that it is opened with AEC effect.
	effects, ok := streams[0]["effects"]
	if !ok || effects != AEC {
		s.Error("The opened input stream is not opened with AEC, effect")
	}
	// We successfully find an input stream with AEC effect, and return nil to stop polling.
	s.Log("Found effects: ", streams[0]["effects"])
}
