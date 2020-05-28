// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package arc

import (
	"context"
	"time"

	"chromiumos/tast/errors"
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

	// Gets the active streams by parsing audio thread dumps.
	getActiveStream := func(ctx context.Context) error {
		s.Log("Dump audio thread to check active streams")
		streams, log, err := audio.DumpActiveStreams(ctx)
		if err != nil {
			s.Log("Failed to parse audio dumps: ", log)
			return testing.PollBreak(errors.Errorf("failed to parse audio dumps: %s", err))
		}

		// No active stream. Return an error to keep polling.
		if len(streams) == 0 {
			return errors.New("fail to detect active stream")
		}

		if len(streams) > 1 {
			s.Log("Too many active streams: ", streams)
			return testing.PollBreak(errors.New("too many active streams"))
		}

		// Verifies that there is an input stream and it is opened with AEC effect.
		s.Log("len(streams) = 1")
		dir, ok := streams[0]["direction"]
		if !ok || dir != "Input" {
			return testing.PollBreak(errors.New("An output stream is detected"))
		}

		effects, ok := streams[0]["effects"]
		if !ok || effects != "0x0001" {
			return testing.PollBreak(errors.New("the opened input stream is not opened with AEC"))
		}

		// We successfully find an input stream with AEC effect, and return nil to stop polling.
		s.Log("effects:", streams[0]["effects"])
		return nil

	}
	a := s.PreValue().(arc.PreData).ARC
	cr := s.PreValue().(arc.PreData).Chrome
	param := s.Param().(audio.TestParameters)
	if err := audio.RunAppAndPollingTest(ctx, a, cr, s.DataPath(audio.Apk), param, getActiveStream, getStreamTimeout); err != nil {
		s.Error("Test failed: ", err)
	}

}
