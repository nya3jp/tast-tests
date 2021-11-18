// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package audio

import (
	"context"
	"regexp"
	"time"

	"chromiumos/tast/common/testexec"
	"chromiumos/tast/local/audio"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/input"
	"chromiumos/tast/testing"
	"chromiumos/tast/testing/hwdep"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         MicrophoneMuteKeyboardKey,
		LacrosStatus: testing.LacrosVariantUnknown,
		Desc:         "Tests to ensure that the dedicated keyboard key for microphone mute toggle properly mutes/unmutes the microphone",
		Contacts: []string{
			"chromeos-audio-sw@google.com",
			"chromeos-sw-engprod@google.com",
			"rtinkoff@chromium.org",
		},
		// TODO(https://crbug.com/1266507): Remove "informational" once stable.
		// TODO(https://crbug.com/1271209): Add a formal HW dependency for devices with KEY_MICMUTE.
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"chrome"},
		HardwareDeps: hwdep.D(hwdep.Model("mrbland", "homestar")),
	})
}

func MicrophoneMuteKeyboardKey(ctx context.Context, s *testing.State) {
	cr, err := chrome.New(ctx)
	if err != nil {
		s.Fatal("Failed to connect to Chrome: ", err)
	}
	defer cr.Close(ctx)

	kb, err := input.Keyboard(ctx)
	if err != nil {
		s.Fatal("Failed to find keyboard: ", err)
	}
	defer kb.Close()

	cras, err := audio.NewCras(ctx)
	if err != nil {
		s.Fatal("Failed to connect to cras: ", err)
	}

	// There is no way to query which device is used by CRAS now. However, the
	// PCM name of internal mic is still correct, we can always run a test on the
	// internal mic until there is a method to get the correct device name.
	// See b/142910355 for more details.
	if err := cras.SetActiveNodeByType(ctx, "INTERNAL_MIC"); err != nil {
		s.Fatal("Failed to set internal mic active: ", err)
	}

	// Test mute via the keyboard key.
	s.Log("Muting the microphone via keyboard key")
	if err := kb.Accel(ctx, "micmute"); err != nil {
		s.Fatal("Failed to mute the microphone: ", err)
	}

	// Set timeout to duration + 1s, which is the time buffer to complete the
	// normal execution.
	const duration = 5 // second
	runCtx, cancel := context.WithTimeout(ctx, (duration+1)*time.Second)
	defer cancel()

	// If the output of cras_test_client reports that "capture" is muted, then we've
	// successfully muted the mic.
	outMuted, err := testexec.CommandContext(
		runCtx, "cras_test_client").Output()
	matchedMuted, err := regexp.Match("Capture Muted : Muted", outMuted)
	if !matchedMuted {
		s.Fatal("Failed to mute microphone")
	}
	s.Log("Mic successfully muted")

	// Test un-mute via the keyboard key.
	s.Log("Un-muting the microphone via keyboard key")
	if err := kb.Accel(ctx, "micmute"); err != nil {
		s.Fatal("Failed to un-mute the microphone: ", err)
	}

	// If the output of cras_test_client reports that "capture" is not muted, then we've
	// successfully muted the mic.
	outUnmuted, err := testexec.CommandContext(
		runCtx, "cras_test_client").Output()
	matchedUnmuted, err := regexp.Match("Capture Muted : Not muted", outUnmuted)
	if !matchedUnmuted {
		s.Fatal("Failed to un-mute microphone")
	}
	s.Log("Mic successfully un-muted")
}
