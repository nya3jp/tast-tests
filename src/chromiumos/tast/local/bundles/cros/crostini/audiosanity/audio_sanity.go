// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package audiosanity provides methods to test the basic audio functionality
// of a container, using alsa.
package audiosanity

import (
	"context"

	"chromiumos/tast/local/testexec"
	"chromiumos/tast/local/vm"
	"chromiumos/tast/testing"
)

// RunTest tests the container's audio subsystem. Specifically it uses
// alsa's aplay and arecord to list and use the audio devices.
func RunTest(ctx context.Context, s *testing.State, cont *vm.Container) {
	s.Log("List alsa output devices")
	if err := cont.Command(ctx, "aplay", "-l").Run(testexec.DumpLogOnError); err != nil {
		s.Fatal("Failed to list alsa output devices: ", err)
	}

	s.Log("Play zeros with alsa device")
	if err := cont.Command(ctx, "aplay", "-r", "48000", "-c", "2", "-d", "3", "-f", "dat", "/dev/zero").Run(testexec.DumpLogOnError); err != nil {
		s.Fatal("Failed to playback with alsa devices: ", err)
	}

	s.Log("List alsa input devices")
	if err := cont.Command(ctx, "arecord", "-l").Run(testexec.DumpLogOnError); err != nil {
		s.Fatal("Failed to list alsa input devices: ", err)
	}

	s.Log("Capture with alsa device")
	if err := cont.Command(ctx, "arecord", "-r", "48000", "-c", "2", "-d", "3", "-f", "dat", "/dev/null").Run(testexec.DumpLogOnError); err != nil {
		s.Fatal("Failed to capture with alsa devices: ", err)
	}
}
