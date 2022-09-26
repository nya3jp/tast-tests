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
	"chromiumos/tast/local/chrome"
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

	arecordArgs := []string{"-Dhw:0,1", "-d", "30", "-f", "dat", "-c", "2", recWavFile}
	cmd := testexec.CommandContext(ctx, "arecord", arecordArgs...)
	if err := cmd.Run(testexec.DumpLogOnError); err != nil {
		s.Fatalf("Failed to execute %q: %v", cmd, err)
	}

	files, err := filesapp.Launch(ctx, tconn)
	if err != nil {
		s.Fatal("Failed to launch the Files App: ", err)
	}
	defer func(ctx context.Context) {
		files.Close(ctx)
		if err := os.Remove(filepath.Join(downloadsPath, recWavFileName)); err != nil {
			s.Errorf("Failed to delete file %q: %v", recWavFileName, err)
		}
		if err = kb.Accel(ctx, "Ctrl+W"); err != nil {
			s.Error("Failed to close Audio player: ", err)
		}
	}(ctx)

	if err := files.OpenDownloads()(ctx); err != nil {
		s.Fatal("Failed to open Downloads folder in files app: ", err)
	}
	if err := files.OpenFile(recWavFileName)(ctx); err != nil {
		s.Fatalf("Failed to open the audio file %q: %v", recWavFileName, err)
	}

	// Sample time for the audio to play for 5 seconds.
	if err := testing.Sleep(ctx, 5*time.Second); err != nil {
		s.Fatal("Failed to sleep: ", err)
	}
}
