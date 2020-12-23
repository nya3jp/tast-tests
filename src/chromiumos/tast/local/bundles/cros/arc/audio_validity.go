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
		Func: AudioValidity,
		Desc: "Audio validity test for arc",
		Contacts: []string{
			"chromeos-audio-bugs@google.com", // Media team
			"cychiang@chromium.org",          // Media team
			"paulhsia@chromium.org",          // Media team
			"judyhsiao@chromium.org",         // Author
		},
		SoftwareDeps: []string{"chrome"},
		Fixture:      "arcBooted",
		Attr:         []string{"group:mainline", "informational"},
		Timeout:      3 * time.Minute,
		Params: []testing.Param{
			{
				Name: "playback",
				Val: audio.TestParameters{
					Class: "org.chromium.arc.testapp.arcaudiotest.TestOutputActivity",
				},
				ExtraSoftwareDeps: []string{"android_p"},
			},
			{
				Name: "playback_vm",
				Val: audio.TestParameters{
					Class: "org.chromium.arc.testapp.arcaudiotest.TestOutputActivity",
				},
				ExtraSoftwareDeps: []string{"android_vm"},
			},
			{
				Name: "record",
				Val: audio.TestParameters{
					Permission: "android.permission.RECORD_AUDIO",
					Class:      "org.chromium.arc.testapp.arcaudiotest.TestInputActivity",
				},
				ExtraSoftwareDeps: []string{"android_p"},
			},
			{
				Name: "record_vm",
				Val: audio.TestParameters{
					Permission: "android.permission.RECORD_AUDIO",
					Class:      "org.chromium.arc.testapp.arcaudiotest.TestInputActivity",
				},
				ExtraSoftwareDeps: []string{"android_vm"},
			},
		},
	})
}

// AudioValidity runs audio validity tests.
func AudioValidity(ctx context.Context, s *testing.State) {
	a := s.FixtValue().(*arc.PreData).ARC
	cr := s.FixtValue().(*arc.PreData).Chrome
	param := s.Param().(audio.TestParameters)
	atast, err := audio.NewARCAudioTast(ctx, a, cr)
	if err != nil {
		s.Fatal("Failed to NewARCAudioTast: ", err)
	}
	if err := atast.RunAppTest(ctx, arc.APKPath(audio.Apk), param); err != nil {
		s.Error("Test failed: ", err)
	}
}
