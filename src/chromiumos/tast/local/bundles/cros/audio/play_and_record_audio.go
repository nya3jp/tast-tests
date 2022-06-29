// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package audio

import (
	"context"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"time"

	"chromiumos/tast/common/testexec"
	"chromiumos/tast/local/audio"
	"chromiumos/tast/local/audio/crastestclient"
	"chromiumos/tast/local/bundles/cros/audio/audionode"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/uiauto/faillog"
	"chromiumos/tast/local/chrome/uiauto/filesapp"
	"chromiumos/tast/local/cryptohome"
	"chromiumos/tast/local/input"
	"chromiumos/tast/testing"
	"chromiumos/tast/testing/hwdep"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         PlayAndRecordAudio,
		LacrosStatus: testing.LacrosVariantUnneeded,
		Desc:         "Play local audio file and record it simultaneously. Then play the recorded audio and verify if it routes through expected audio node",
		Contacts:     []string{"ambalavanan.m.m@intel.com", "intel-chrome-system-automation-team@intel.com"},
		HardwareDeps: hwdep.D(hwdep.Speaker(), hwdep.Microphone()),
		SoftwareDeps: []string{"chrome"},
		Pre:          chrome.LoggedIn(),
	})
}

// PlayAndRecordAudio generates audio file, plays it and records it simultaneously. Then plays the recorded audio and verify if it routes through expected audio node.
func PlayAndRecordAudio(ctx context.Context, s *testing.State) {
	const (
		expectedAudioNode = "INTERNAL_SPEAKER"
		audioRate         = 48000
		audioChannel      = 2
		duration          = 30
	)

	cr := s.PreValue().(*chrome.Chrome)

	// Generate sine raw input file that lasts 30 seconds.
	rawTempFile, err := ioutil.TempFile("", "30SEC_*.raw")
	if err != nil {
		s.Error("Failed to create raw temp file: ", err)
	}
	if err := rawTempFile.Close(); err != nil {
		s.Error("Failed to close raw temp file: ", err)
	}
	rawFile := audio.TestRawData{
		Path:          rawTempFile.Name(),
		BitsPerSample: 16,
		Channels:      audioChannel,
		Rate:          audioRate,
		Frequencies:   []int{440, 440},
		Volume:        0.05,
		Duration:      duration,
	}
	if err := audio.GenerateTestRawData(ctx, rawFile); err != nil {
		s.Fatal("Failed to generate audio test data: ", err)
	}

	wavTempFile, err := ioutil.TempFile("", "30SEC_*.wav")
	if err != nil {
		s.Error("Failed to create wav temp file: ", err)
	}
	if err := wavTempFile.Close(); err != nil {
		s.Error("Failed to close wav temp file: ", err)
	}
	if err := audio.ConvertRawToWav(ctx, rawTempFile.Name(), wavTempFile.Name(), audioRate, audioChannel); err != nil {
		s.Fatal("Failed to convert raw to wav: ", err)
	}

	// Open the test API.
	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to create test API connection: ", err)
	}
	defer faillog.DumpUITreeOnError(ctx, s.OutDir(), s.HasError, tconn)
	files, err := filesapp.Launch(ctx, tconn)
	if err != nil {
		s.Fatal("Failed to launch the Files App: ", err)
	}
	defer files.Close(ctx)
	if err := files.OpenDownloads()(ctx); err != nil {
		s.Fatal("Failed to open Downloads folder in files app: ", err)
	}

	audioDeviceName, err := audionode.SetAudioNode(ctx, expectedAudioNode)
	if err != nil {
		s.Fatal("Failed to set the Audio node: ", err)
	}
	s.Logf("Selected audio device name: %s", audioDeviceName)

	recWavFileName := "30SEC_REC.wav"
	downloadsPath, err := cryptohome.DownloadsPath(ctx, cr.NormalizedUser())
	if err != nil {
		s.Fatal("Failed to get user's Download path: ", err)
	}
	recWavFile := filepath.Join(downloadsPath, recWavFileName)
	cmd := fmt.Sprintf("play -c %d -r %d %s & rec -r %d -c %d %s trim 0 30", audioChannel, audioRate, wavTempFile.Name(), audioRate, audioChannel, recWavFile)
	output := testexec.CommandContext(ctx, "sh", "-c", cmd)
	if err := output.Run(testexec.DumpLogOnError); err != nil {
		s.Fatalf("Failed to execute %q: %v", cmd, err)
	}

	kb, err := input.VirtualKeyboard(ctx)
	if err != nil {
		s.Fatal("Failed to open the keyboard: ", err)
	}
	defer kb.Close()

	if err := files.OpenFile(recWavFileName)(ctx); err != nil {
		s.Fatalf("Failed to open the audio file %q: %v", recWavFileName, err)
	}
	// Sample time for the audio to play for 5 seconds.
	testing.Sleep(ctx, 5*time.Second)

	// Deleting the files and closing the audio player.
	defer func() {
		os.Remove(rawTempFile.Name())
		os.Remove(wavTempFile.Name())
		os.Remove(recWavFile)
		if kb.Accel(ctx, "Ctrl+W"); err != nil {
			s.Error("Failed to close Audio player: ", err)
		}
	}()

	devName, err := crastestclient.FirstRunningDevice(ctx, audio.OutputStream)
	if err != nil {
		s.Fatal("Failed to detect running output device: ", err)
	}

	if audioDeviceName != devName {
		s.Fatalf("Failed to route the audio through expected audio node: got %q; want %q", devName, audioDeviceName)
	}
}
