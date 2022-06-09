// Copyright 2022 The ChromiumOS Authors.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package audio

import (
	"context"
	"time"

	"chromiumos/tast/ctxutil"
	"chromiumos/tast/local/audio"
	"chromiumos/tast/local/audio/crastestclient"
	"chromiumos/tast/local/bundles/cros/audio/audionode"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/chrome/uiauto/faillog"
	"chromiumos/tast/local/chrome/uiauto/nodewith"
	"chromiumos/tast/local/chrome/uiauto/role"
	"chromiumos/tast/local/input"
	"chromiumos/tast/testing"
	"chromiumos/tast/testing/hwdep"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         BrowserShellAudioToneCheck,
		LacrosStatus: testing.LacrosVariantUnneeded,
		Desc:         "Verifies system tones in browser shell",
		Contacts:     []string{"ambalavanan.m.m@intel.com", "intel-chrome-system-automation-team@intel.com"},
		SoftwareDeps: []string{"chrome"},
		Attr:         []string{"group:mainline", "informational"},
		HardwareDeps: hwdep.D(hwdep.Speaker()),
		Fixture:      "chromeLoggedIn",
	})
}

func BrowserShellAudioToneCheck(ctx context.Context, s *testing.State) {
	// Shorten deadline to leave time for cleanup.
	cleanupCtx := ctx
	ctx, cancel := ctxutil.Shorten(ctx, 20*time.Second)
	defer cancel()

	cr := s.FixtValue().(*chrome.Chrome)
	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to create test API connection: ", err)
	}
	defer faillog.DumpUITreeOnError(cleanupCtx, s.OutDir(), s.HasError, tconn)

	kb, err := input.VirtualKeyboard(ctx)
	if err != nil {
		s.Fatal("Failed to find keyboard: ", err)
	}
	defer kb.Close()

	expectedAudioNode := "INTERNAL_SPEAKER"
	audioDeviceName, err := audionode.SetAudioNode(ctx, expectedAudioNode)
	if err != nil {
		s.Fatal("Failed to set the Audio node: ", err)
	}

	vh, err := audionode.NewVolumeHelper(ctx)
	if err != nil {
		s.Fatal("Failed to create the volumeHelper: ", err)
	}
	originalVolume, err := vh.ActiveNodeVolume(ctx)
	defer vh.SetVolume(cleanupCtx, originalVolume)

	maxVolume := 100
	if err := vh.SetVolume(ctx, maxVolume); err != nil {
		s.Fatal("Failed to set volume to maximum: ", err)
	}

	// Launching browser shell.
	if err := kb.Accel(ctx, "ctrl+alt+t"); err != nil {
		s.Fatal("Failed to press Ctrl+Alt+t to launch browser shell: ", err)
	}

	ui := uiauto.New(tconn)
	// Close browser shell window as cleanup.
	defer func(ctx context.Context) {
		croshCloseButton := nodewith.Name("Close").ClassName("FrameCaptionButton").Role(role.Button)
		if err := ui.LeftClick(croshCloseButton)(ctx); err != nil {
			s.Error("Failed to close browser shell: ", err)
		}
		croshLeaveButton := nodewith.Name("Leave").Role(role.Button)
		if err := ui.WaitForLocation(croshLeaveButton)(ctx); err != nil {
			s.Error("Failed to wait for 'Leave' prompt button: ", err)
		}
		if err := ui.LeftClick(croshLeaveButton)(ctx); err != nil {
			s.Error("Failed to left click 'Leave' prompt button: ", err)
		}
	}(cleanupCtx)

	croshWindow := nodewith.Name("Chrome - crosh").Role(role.Window)
	if err := ui.WaitForLocation(croshWindow)(ctx); err != nil {
		s.Fatal("Failed to launch browser shell: ", err)
	}

	// Enter 'shell' text and go into chronos@localhost terminal.
	typeSequenceString := []string{"s", "h", "e", "l", "l"}
	if err := kb.TypeSequence(ctx, typeSequenceString); err != nil {
		s.Fatal("Failed to type 'shell' text: ", err)
	}

	if err := kb.Accel(ctx, "enter"); err != nil {
		s.Fatal("Failed to press Enter key: ", err)
	}

	// Press downward arror keyboard key.
	if err := kb.Accel(ctx, "down"); err != nil {
		s.Fatal("Failed to press 'down' key in browser shell: ", err)
	}

	// Bell sound should be emitted by DUT.
	devName, err := crastestclient.FirstRunningDevice(ctx, audio.OutputStream)
	if err != nil {
		s.Fatal("Failed to detect running output device: ", err)
	}

	if audioDeviceName != devName {
		s.Fatalf("Failed to route the audio tone through expected audio node: got %q; want %q", devName, audioDeviceName)
	}
}
