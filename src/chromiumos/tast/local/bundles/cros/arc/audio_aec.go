// Copyright 2020 The ChromiumOS Authors
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
		Func:         AudioAEC,
		LacrosStatus: testing.LacrosVariantUnneeded,
		Desc:         "Audio AEC test for arc",
		Contacts: []string{
			"chromeos-audio-bugs@google.com", // Media team
			"cychiang@chromium.org",          // Media team
			"paulhsia@chromium.org",          // Media team
			"judyhsiao@chromium.org",         // Author
		},
		SoftwareDeps: []string{"chrome", "android_vm", "audio_stable"},
		// Disable this test case as AEC is currently disabled in ARCVM. (b/201378884)
		Attr:    []string{},
		Timeout: 2 * time.Minute,
		Fixture: "arcBooted",
	})
}

// AudioAEC runs audio AEC tests.
func AudioAEC(ctx context.Context, s *testing.State) {
	// This comes from APM_ECHO_CANCELLATION defined in cras_types.h
	const AEC = 1

	a := s.FixtValue().(*arc.PreData).ARC
	cr := s.FixtValue().(*arc.PreData).Chrome
	d := s.FixtValue().(*arc.PreData).UIDevice
	param := audio.TestParameters{
		Permission: "android.permission.RECORD_AUDIO",
		Class:      "org.chromium.arc.testapp.arcaudiotest.TestAECEffectActivity",
	}

	atast, err := audio.NewARCAudioTast(ctx, a, cr, d)
	if err != nil {
		s.Fatal("Failed to NewARCAudioTast: ", err)
	}
	streams, err := atast.RunAppAndPollStream(ctx, arc.APKPath(audio.Apk), param)
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
