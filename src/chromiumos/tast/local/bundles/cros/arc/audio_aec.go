// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package arc

import (
	"context"
	"time"

	"chromiumos/tast/local/arc"
	audio "chromiumos/tast/local/bundles/cros/arc/audio"
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
		SoftwareDeps: []string{"chrome"},
		Data:         []string{"ArcAudioTest.apk"},
		Attr:         []string{"group:mainline", "informational"},
		Timeout:      2 * time.Minute,
		Params: []testing.Param{
			{
				Name: "vm",
				Val: audio.TestParameters{
					Permission: "android.permission.RECORD_AUDIO",
					Class:      "org.chromium.arc.testapp.arcaudiotestapp.TestAECEffectActivity",
				},
				ExtraSoftwareDeps: []string{"android_vm"},
				Pre:               arc.VMBooted(),
			},
		},
	})
}

// AudioAEC runs audio AEC tests.
func AudioAEC(ctx context.Context, s *testing.State) {
	audio.Run01ResultTest(ctx, s)
}
