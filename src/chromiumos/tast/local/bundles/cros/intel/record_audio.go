// Copyright 2022 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package intel

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"time"

	"chromiumos/tast/common/testexec"
	"chromiumos/tast/ctxutil"
	"chromiumos/tast/local/audio"
	"chromiumos/tast/local/audio/crastestclient"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/chrome/uiauto/filesapp"
	"chromiumos/tast/local/cryptohome"
	"chromiumos/tast/local/input"
	"chromiumos/tast/testing"
	"chromiumos/tast/testing/hwdep"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         RecordAudio,
		LacrosStatus: testing.LacrosVariantUnneeded,
		Desc:         "Test audio record file",
		Contacts:     []string{"ambalavanan.m.m@intel.com", "intel-chrome-system-automation-team@intel.com"},
		SoftwareDeps: []string{"chrome"},
		HardwareDeps: hwdep.D(hwdep.Microphone()),
		Timeout:      10 * time.Minute,
		Fixture:      "chromeLoggedIn",
	})
}

func RecordAudio(ctx context.Context, s *testing.State) {
	cr := s.FixtValue().(*chrome.Chrome)

	cleanupCtx := ctx
	ctx, cancel := ctxutil.Shorten(ctx, 10*time.Second)
	defer cancel()

	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to create test API connection: ", err)
	}
	kb, err := input.VirtualKeyboard(ctx)
	if err != nil {
		s.Fatal("Failed to open the keyboard: ", err)
	}
	defer kb.Close()

	out, err := testexec.CommandContext(ctx, "arecord", "-l").CombinedOutput(testexec.DumpLogOnError)
	if err != nil {
		s.Fatal("Failed to check arecord command: ", err)
	}
	if strings.Contains(string(out), "no soundcards found") {
		s.Fatal("Failed to recognize sound cards")
	}

	recWavFileName := "30SEC_REC.wav"
	downloadsPath, err := cryptohome.DownloadsPath(ctx, cr.NormalizedUser())
	if err != nil {
		s.Fatal("Failed to get user's Download path: ", err)
	}
	recWavFile := filepath.Join(downloadsPath, recWavFileName)

	arecordArgs := []string{"-Dhw:0,1", // device name.
		"-d", "30", // duration.
		"-f", "dat", // format.
		"-c", "2", //  number of channels.
		recWavFile, // output file.
	}
	cmd := testexec.CommandContext(ctx, "arecord", arecordArgs...)
	if err := cmd.Run(testexec.DumpLogOnError); err != nil {
		s.Fatalf("Failed to execute %q: %v", cmd, err)
	}

	files, err := filesapp.Launch(ctx, tconn)
	if err != nil {
		s.Fatal("Failed to launch the Files App: ", err)
	}
	defer func(ctx context.Context) {
		if err := files.Close(ctx); err != nil {
			s.Error("Failed to close the Files App: ", err)
		}
		if err := os.Remove(filepath.Join(downloadsPath, recWavFileName)); err != nil {
			s.Errorf("Failed to delete file %q: %v", recWavFileName, err)
		}
		if err = kb.Accel(ctx, "Ctrl+W"); err != nil {
			s.Error("Failed to close Audio player: ", err)
		}
	}(cleanupCtx)

	if err := uiauto.Combine("open wav file in downloads",
		files.OpenDownloads(),
		files.OpenFile(recWavFileName),
	)(ctx); err != nil {
		s.Fatal("Failed to open file in downloads: ", err)
	}

	// Sample time for the audio to play for 5 seconds.
	if err := testing.Sleep(ctx, 5*time.Second); err != nil {
		s.Fatal("Failed to sleep: ", err)
	}

	// Get Current active node.
	cras, err := audio.NewCras(ctx)
	if err != nil {
		s.Fatal("Failed to create Cras object")
	}
	audioDeviceName, _, err := cras.SelectedOutputDevice(ctx)
	if err != nil {
		s.Fatal("Failed to get the selected audio device: ", err)
	}

	devName, err := crastestclient.FirstRunningDevice(ctx, audio.OutputStream)
	if err != nil {
		s.Fatal("Failed to detect running output device: ", err)
	}

	if audioDeviceName != devName {
		s.Fatalf("Failed to route the audio through expected audio node: got %q; want %q", devName, audioDeviceName)
	}
}
