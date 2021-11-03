// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package launcher

import (
	"context"
	"regexp"
	"time"

	"chromiumos/tast/common/testexec"
	"chromiumos/tast/local/audio"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/input"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func: MicMuteKey,
		Desc: "Tests to ensure that both the microphone mute key and debug accelerator properly mutes/unmutes the microphone",
		Contacts: []string{
			"cros-system-ui-eng@google.com",
			"chromeos-sw-engprod@google.com",
			"rtinkoff@chromium.org",
		},
		// TODO(https://crbug.com/1266507): Remove "informational" once stable.
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"chrome"},
	})
}

func MicMuteKey(ctx context.Context, s *testing.State) {
	cr, err := chrome.New(ctx, chrome.EnableFeatures("ProductivityLauncher"))
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

	// Test mute via the debug accelerator.
	s.Log("Muting the microphone via debug accelerator")
	if err := kb.Accel(ctx, "Shift+Ctrl+Alt+M"); err != nil {
		s.Fatal("Failed to mute the microphone: ", err)
	}

	// Set timeout to duration + 1s, which is the time buffer to complete the
	// normal execution.
	const duration = 5 // second
	runCtx, cancel := context.WithTimeout(ctx, (duration+1)*time.Second)
	defer cancel()

	// If the output of cras_test_client reports that "capture" is muted, then we've
	// successfully muted the mic.
	out_muted, err := testexec.CommandContext(
		runCtx, "cras_test_client").Output()
	matched_muted, err := regexp.Match("Capture Muted : Muted", out_muted)
	if matched_muted {
		s.Log("Mic successfully muted")
	} else {
		s.Fatal("Failed to mute microphone")
	}

	// Test un-mute via the debug accelerator.
	s.Log("Un-muting the microphone via debug accelerator")
	if err := kb.Accel(ctx, "Shift+Ctrl+Alt+M"); err != nil {
		s.Fatal("Failed to un-mute the microphone: ", err)
	}

	// If the output of cras_test_client reports that "capture" is not muted, then we've
	// successfully muted the mic.
	out_unmuted, err := testexec.CommandContext(
		runCtx, "cras_test_client").Output()
	matched_unmuted, err := regexp.Match("Capture Muted : Not muted", out_unmuted)
	if matched_unmuted {
		s.Log("Mic successfully un-muted")
	} else {
		s.Fatal("Failed to un-mute microphone")
	}
}
