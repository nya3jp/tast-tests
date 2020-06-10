// Copyright 2019 The Chromium OS Authors. All rights reserved.
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
		Func: AudioSanity,
		Desc: "Audio sanity test for arc",
		Contacts: []string{
			"chromeos-audio-bugs@google.com", // Media team
			"cychiang@chromium.org",          // Media team
			"paulhsia@chromium.org",          // Media team
			"judyhsiao@chromium.org",         // Author
		},
		SoftwareDeps: []string{"chrome"},
		Pre:          arc.Booted(),
		Data:         []string{"ArcAudioTest.apk"},
		Attr:         []string{"group:mainline", "informational"},
		Timeout:      3 * time.Minute,
		Params: []testing.Param{
			{
				Name: "playback",
				Val: audio.TestParameters{
					Class: "org.chromium.arc.testapp.arcaudiotestapp.TestOutputActivity",
				},
				ExtraSoftwareDeps: []string{"android_p"},
			},
			{
				Name: "playback_vm",
				Val: audio.TestParameters{
					Class: "org.chromium.arc.testapp.arcaudiotestapp.TestOutputActivity",
				},
				ExtraSoftwareDeps: []string{"android_vm"},
			},
			{
				Name: "record",
				Val: audio.TestParameters{
					Permission: "android.permission.RECORD_AUDIO",
					Class:      "org.chromium.arc.testapp.arcaudiotestapp.TestInputActivity",
				},
				ExtraSoftwareDeps: []string{"android_p"},
			},
			{
				Name: "record_vm",
				Val: audio.TestParameters{
					Permission: "android.permission.RECORD_AUDIO",
					Class:      "org.chromium.arc.testapp.arcaudiotestapp.TestInputActivity",
				},
				ExtraSoftwareDeps: []string{"android_vm"},
			},
		},
	})
}

// AudioSanity runs audio sanity tests.
func AudioSanity(ctx context.Context, s *testing.State) {
	a := s.PreValue().(arc.PreData).ARC
	cr := s.PreValue().(arc.PreData).Chrome
	param := s.Param().(audio.TestParameters)
	atast, err := audio.NewArcAudioTast(ctx, a, cr)
	if err != nil {
		s.Fatal("Failed to NewArcAudioTast: ", err)
	}
	if err := atast.RunAppTest(ctx, s.DataPath(audio.Apk), param); err != nil {
		s.Error("Test failed: ", err)
	}
}
