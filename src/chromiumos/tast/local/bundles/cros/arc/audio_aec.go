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
		Pre:          arc.Booted(),
	})
}

// AudioAEC runs audio AEC tests.
func AudioAEC(ctx context.Context, s *testing.State) {
	// This comes from APM_ECHO_CANCELLATION defined in cras_types.h
	const AEC = 1

	a := s.PreValue().(arc.PreData).ARC
	cr := s.PreValue().(arc.PreData).Chrome
	param := audio.TestParameters{
		Permission: "android.permission.RECORD_AUDIO",
		Class:      "org.chromium.arc.testapp.arcaudiotestapp.TestAECEffectActivity",
	}

	atast, err := audio.NewARCAudioTast(ctx, a, cr)
	if err != nil {
		s.Fatal("Failed to NewARCAudioTast: ", err)
	}
	streams, err := atast.RunAppAndPollStream(ctx, s.DataPath(audio.Apk), param)
	if err != nil {
		s.Fatal("Test failed: ", err)
	}
	if len(streams) == 0 {
		s.Fatal("Failed to detect an active stream")
	}
	if len(streams) > 1 {
		s.Fatalf("Too many active streams, found %d streams", len(streams))
	}
	// Verifies that there is an input stream.
	if streams[0].Direction != "Input" {
		s.Fatalf("Unexpected stream direction: got %s, want Input", streams[0].Direction)
	}
	// Verifies that the input stream is opened with AEC effect.
	if streams[0].Effects != AEC {
		s.Fatalf("Unexpected stream effects: got %#x, want %#x", streams[0].Effects, AEC)
	}
	s.Logf("Found effects: %#x", streams[0].Effects)
}
