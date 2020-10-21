// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package inputs

import (
	"context"
	"os"
	"path/filepath"

	"chromiumos/tast/fsutil"
	"chromiumos/tast/local/bundles/cros/inputs/pre"
	"chromiumos/tast/local/bundles/cros/inputs/testserver"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/ui/faillog"
	"chromiumos/tast/local/chrome/ui/filesapp"
	"chromiumos/tast/local/chrome/vkb"
	"chromiumos/tast/local/input"
	"chromiumos/tast/testing"
)

const enTestFile = "voice_en_hello_20201021.wav"
const expectedText = "Hello"

func init() {
	testing.AddTest(&testing.Test{
		Func:         VirtualKeyboardSpeech,
		Desc:         "Tests that user can input in speech on virtual keyboard",
		Contacts:     []string{"shengjun@chromium.org", "essential-inputs-team@google.com"},
		SoftwareDeps: []string{"chrome"},
		Attr:         []string{"group:mainline", "informational"},
		Data:         []string{enTestFile},
		Pre:          pre.VKEnabled(),
	})
}

func VirtualKeyboardSpeech(ctx context.Context, s *testing.State) {
	cr := s.PreValue().(*chrome.Chrome)
	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to create Test API connection: ", err)
	}

	defer faillog.DumpUITreeOnError(ctx, s.OutDir(), s.HasError, tconn)

	cleanup, err := input.EnableALoopInput(ctx, tconn)
	if err != nil {
		s.Fatal("Failed to enable ALoop: ", err)
	}
	defer cleanup(ctx)

	// Setup the test audio file.
	testFileLocation := filepath.Join(filesapp.DownloadPath, enTestFile)
	if err := fsutil.CopyFile(s.DataPath(enTestFile), testFileLocation); err != nil {
		s.Fatalf("Failed to copy the test image to %s: %s", testFileLocation, err)
	}
	defer os.Remove(testFileLocation)

	// Launch inputs test web server.
	its, err := testserver.Launch(ctx, cr)
	if err != nil {
		s.Fatal("Fail to launch inputs test server: ", err)
	}
	defer its.Close()

	inputField := testserver.TextAreaInputField

	if err := inputField.ClickUntilVKShown(ctx, tconn); err != nil {
		s.Fatal("Failed to click input field to show virtual keyboard: ", err)
	}

	defer func() {
		if err := vkb.HideVirtualKeyboard(ctx, tconn); err != nil {
			s.Log("Failed to hide virtual keyboard: ", err)
		}
	}()

	if err := vkb.WaitUntilShown(ctx, tconn); err != nil {
		s.Fatal("Failed to wait for virtual keyboard shown and locationed: ", err)
	}

	vkb.SwitchToVoiceInput(ctx, tconn)

	if err := input.AudioFromFile(ctx, testFileLocation); err != nil {
		s.Fatal("Failed to input audio: ", err)
	}

	if err := inputField.WaitForValueToBe(ctx, tconn, expectedText); err != nil {
		s.Fatal("Failed to verify input: ", err)
	}
}
