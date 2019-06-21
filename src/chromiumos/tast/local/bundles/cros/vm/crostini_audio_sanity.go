// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package vm

import (
	"context"
	"time"

	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/testexec"
	"chromiumos/tast/local/vm"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         CrostiniAudioSanity,
		Desc:         "Tests basic Crostini audio functions through alsa",
		Contacts:     []string{"paulhsia@chromium.org", "cros-containers-dev@google.com", "chromeos-audio-bugs@google.com"},
		Attr:         []string{"informational"},
		Timeout:      7 * time.Minute, // the longest part is to setup the container
		Data:         []string{"crostini_start_basic_guest_images.tar"},
		SoftwareDeps: []string{"chrome", "vm_host"},
		Pre:          chrome.LoggedIn(),
	})
}

func CrostiniAudioSanity(ctx context.Context, s *testing.State) {
	// TODO(paulhsia): Remove the container setup when the refactor is done.
	s.Log("Enabling Crostini preference setting")
	cr := s.PreValue().(*chrome.Chrome)
	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to create test API connection: ", err)
	}
	if err = vm.EnableCrostini(ctx, tconn); err != nil {
		s.Fatal("Failed to enable Crostini preference setting: ", err)
	}

	s.Log("Setting up component")
	artifactPath := s.DataPath("crostini_start_basic_guest_images.tar")
	if err := vm.MountArtifactComponent(ctx, artifactPath); err != nil {
		s.Fatal("Failed to set up component: ", err)
	}
	defer vm.UnmountComponent(ctx)

	s.Log("Creating default container")
	cont, err := vm.CreateDefaultContainer(ctx, s.OutDir(), cr.User(), vm.Tarball, artifactPath)
	if err != nil {
		s.Fatal("Failed to set up default container: ", err)
	}
	defer vm.StopConcierge(ctx)
	defer func() {
		if err := cont.DumpLog(ctx, s.OutDir()); err != nil {
			s.Error("Failed to dump container log: ", err)
		}
	}()

	s.Log("List alsa output devices")
	if err = cont.Command(ctx, "aplay", "-l").Run(testexec.DumpLogOnError); err != nil {
		s.Fatal("Failed to list alsa output devices: ", err)
	}

	s.Log("Play zeros with alsa device")
	if err = cont.Command(ctx, "aplay", "-r", "48000", "-c", "2", "-d", "3", "-f", "dat", "/dev/zero").Run(testexec.DumpLogOnError); err != nil {
		s.Fatal("Failed to playback with alsa devices: ", err)
	}

	s.Log("List alsa input devices")
	if err = cont.Command(ctx, "arecord", "-l").Run(testexec.DumpLogOnError); err != nil {
		s.Fatal("Failed to list alsa input devices: ", err)
	}

	s.Log("Capture with alsa device")
	if err = cont.Command(ctx, "arecord", "-r", "48000", "-c", "2", "-d", "3", "-f", "dat", "/dev/null").Run(testexec.DumpLogOnError); err != nil {
		s.Fatal("Failed to capture with alsa devices: ", err)
	}
}
