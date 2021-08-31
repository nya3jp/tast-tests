// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package audio

import (
	"context"
	"io"
	"os"
	"path/filepath"
	"time"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/audio"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/ui/filesapp"
	"chromiumos/tast/local/chrome/uiauto/faillog"
	"chromiumos/tast/testing"
)

const (
	mp3AudioFile = "audio_file.mp3"
	storageType  = "Downloads"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         LocalAudioPlayback,
		Desc:         "Play local audio file through default app and check if the audio is routing through expected device",
		Contacts:     []string{"ambalavanan.m.m@intel.com", "intel-chrome-system-automation-team@intel.com"},
		SoftwareDeps: []string{"chrome"},
		Attr:         []string{"group:mainline", "informational"},
		Data:         []string{mp3AudioFile},
		Pre:          chrome.LoggedIn(),
		Params: []testing.Param{{
			Name: "internal_speaker",
			Val:  "INTERNAL_SPEAKER",
		}, {
			Name: "headphone",
			Val:  "HEADPHONE",
		}},
	})
}

// LocalAudioPlayback Copy audio file to Chromebook and play it through default audio player.
func LocalAudioPlayback(ctx context.Context, s *testing.State) {
	expectedAudioNode := s.Param().(string)

	cr := s.PreValue().(*chrome.Chrome)

	// Copy file from source to destination path.
	copy := func(src, dst string) error {
		sourceFileStat, err := os.Stat(src)
		if err != nil {
			return err
		}
		if !sourceFileStat.Mode().IsRegular() {
			return errors.Errorf("%s is not a regular file", src)
		}
		source, err := os.Open(src)
		if err != nil {
			return err
		}
		defer source.Close()

		destination, err := os.Create(dst)
		if err != nil {
			return err
		}
		defer destination.Close()
		_, err = io.Copy(destination, source)
		return err
	}

	// Copy mp3 audio file to Downloads folder.
	audioFileDownloadPath := filepath.Join(filesapp.DownloadPath, mp3AudioFile)
	if err := copy(s.DataPath(mp3AudioFile), audioFileDownloadPath); err != nil {
		s.Fatal("Failed to copy the Audio mp3 file failed to Downloads folder")
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
	defer files.Release(ctx)
	if err := files.OpenDir(ctx, storageType, "Files - "+storageType); err != nil {
		s.Fatalf("Failed to open %v folder in files app: %v", storageType, err)
	}
	if err := files.OpenFile(ctx, mp3AudioFile); err != nil {
		s.Fatalf("Failed to open the audio file %q: %v", mp3AudioFile, err)
	}

	// Sample time for the audio to play for 5 seconds.
	testing.Sleep(ctx, 5*time.Second)

	cras, err := audio.NewCras(ctx)
	if err != nil {
		s.Fatal("Failed to create Cras object: ", err)
	}
	// Set the expected audio node for audio routing.
	setAudioNode := func() (string, string) {
		deviceName, deviceType, err := cras.GetSelectedOutputDeviceNameAndType(ctx)
		if err != nil {
			s.Fatal("Failed to get the selected audio device")
		}

		if deviceType != expectedAudioNode {
			s.Logf("%s is not selected, selecting %s", expectedAudioNode, expectedAudioNode)
			if err := cras.SetActiveNodeByType(ctx, expectedAudioNode); err != nil {
				s.Fatalf("Failed to select active device %s: %v", expectedAudioNode, err)
			}
			deviceName, deviceType, err = cras.GetSelectedOutputDeviceNameAndType(ctx)
			if err != nil {
				s.Fatal("Failed to get the selected audio device: ", err)
			}
			if deviceType != expectedAudioNode {
				s.Fatalf("Failed to select the active device %s", expectedAudioNode)
			}
		}
		return deviceName, deviceType
	}
	audioDeviceName, audioDeviceType := setAudioNode()
	s.Logf("Audio device name:%s Audio device type:%s", audioDeviceName, audioDeviceType)

	// Verify if audio is routing through expected node.
	err = audio.CheckAudioStreamAtSelectedDevice(ctx, audioDeviceName, audioDeviceType)
	if err != nil {
		s.Fatal("Failed to route the audio through expected audio node: ", err)
	}
}
