// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package arc

import (
	"context"
	"strings"
	"time"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/arc"
	audio "chromiumos/tast/local/bundles/cros/arc/audio"
	"chromiumos/tast/local/testexec"
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

	const (
		getStreamTimeout = 5 * time.Second
	)

	// Gets the active streams by parsing audio thread logs.
	getActiveStream := func(ctx context.Context) error {
		s.Log("Dump audio thread to check active streams")
		dump, err := testexec.CommandContext(ctx, "cras_test_client", "--dump_audio_thread").Output()
		if err != nil {
			return errors.Errorf("failed to dump audio thread: %s", err)
		}

		// Verifies that stream is opened with AEC effect.
		if strings.Contains(string(dump), "effects: 0x0001") {

			return nil
		}
		return errors.New("no stream opened with AEC effect")
	}

	// Starts a goroutine to verify that there is an active stream with AEC effect in CRAS.
	pollActiveStream := func() {
		if err := testing.Poll(ctx, getActiveStream, &testing.PollOptions{Timeout: getStreamTimeout}); err != nil {
			s.Error("Failed to detect active stream with AEC effect: ", err)
		}
	}

	audio.Run01withPollResultTest(ctx, s, pollActiveStream)

}
