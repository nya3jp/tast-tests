// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package inputs

import (
	"context"
	"os"
	"path/filepath"
	"time"

	"chromiumos/tast/ctxutil"
	"chromiumos/tast/fsutil"
	"chromiumos/tast/local/bundles/cros/inputs/testserver"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/ime"
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/chrome/uiauto/faillog"
	"chromiumos/tast/local/chrome/uiauto/filesapp"
	"chromiumos/tast/local/chrome/uiauto/vkb"
	"chromiumos/tast/local/input"
	"chromiumos/tast/testing"
)

const (
	audioFileEN = "voice_en_hello_20201021.wav"
	audioFileCN = "voice_cn_hello_20201021.wav"
)

// Struct to contain the virtual keyboard speech test parameters.
type speechTestParams struct {
	audioFile    string
	expectedText string
	imeID        ime.InputMethodCode
}

func init() {
	testing.AddTest(&testing.Test{
		Func:         VirtualKeyboardSpeech,
		Desc:         "Test voice input functionality on virtual keyboard",
		Contacts:     []string{"shengjun@chromium.org", "essential-inputs-team@google.com"},
		SoftwareDeps: []string{"chrome", "google_virtual_keyboard"},
		Attr:         []string{"group:mainline", "informational", "group:input-tools"},
		Data:         []string{audioFileCN},
		Params: []testing.Param{
			{Name: "1"},
			{Name: "2"},
			{Name: "3"},
			{Name: "4"},
			{Name: "5"},
			{Name: "6"},
			{Name: "7"},
			{Name: "8"},
			{Name: "9"},
			{Name: "10"},
			{Name: "11"},
			{Name: "12"},
			{Name: "13"},
			{Name: "14"},
			{Name: "15"},
			{Name: "16"},
			{Name: "17"},
			{Name: "18"},
			{Name: "19"},
			{Name: "20"},
			{Name: "21"},
			{Name: "22"},
			{Name: "23"},
			{Name: "24"},
			{Name: "25"},
			{Name: "26"},
			{Name: "27"},
			{Name: "28"},
			{Name: "29"},
			{Name: "30"},
			{Name: "31"},
			{Name: "32"},
			{Name: "33"},
			{Name: "34"},
			{Name: "35"},
			{Name: "36"},
			{Name: "37"},
			{Name: "38"},
			{Name: "39"},
			{Name: "40"},
			{Name: "41"},
			{Name: "42"},
			{Name: "43"},
			{Name: "44"},
			{Name: "45"},
			{Name: "46"},
			{Name: "47"},
			{Name: "48"},
			{Name: "49"},
			{Name: "50"},
			{Name: "51"},
			{Name: "52"},
			{Name: "53"},
			{Name: "54"},
			{Name: "55"},
			{Name: "56"},
			{Name: "57"},
			{Name: "58"},
			{Name: "59"},
			{Name: "60"},
			{Name: "61"},
			{Name: "62"},
			{Name: "63"},
			{Name: "64"},
			{Name: "65"},
			{Name: "66"},
			{Name: "67"},
			{Name: "68"},
			{Name: "69"},
			{Name: "70"},
			{Name: "71"},
			{Name: "72"},
			{Name: "73"},
			{Name: "74"},
			{Name: "75"},
			{Name: "76"},
			{Name: "77"},
			{Name: "78"},
			{Name: "79"},
			{Name: "80"},
			{Name: "81"},
			{Name: "82"},
			{Name: "83"},
			{Name: "84"},
			{Name: "85"},
			{Name: "86"},
			{Name: "87"},
			{Name: "88"},
			{Name: "89"},
			{Name: "90"},
			{Name: "91"},
			{Name: "92"},
			{Name: "93"},
			{Name: "94"},
			{Name: "95"},
			{Name: "96"},
			{Name: "97"},
			{Name: "98"},
			{Name: "99"},
			{Name: "100"},
		},
	})
}

func VirtualKeyboardSpeech(ctx context.Context, s *testing.State) {
	cleanupCtx := ctx
	// Use a shortened context for test operations to reserve time for cleanup.
	ctx, shortCancel := ctxutil.Shorten(ctx, 10*time.Second)
	defer shortCancel()

	// TODO(crbug/1173252): Clean up states within Chrome using preconditions.
	cr, err := chrome.New(ctx, chrome.VKEnabled(), chrome.ExtraArgs("--force-tablet-mode=touch_view"))
	defer cr.Close(cleanupCtx)

	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to connect Test API: ", err)
	}

	defer faillog.DumpUITreeOnError(cleanupCtx, s.OutDir(), s.HasError, tconn)

	// Test parameters that are specific to the current test case.
	audioFile := audioFileCN
	expectedText := "你好"
	testIME := ime.IMEPrefix + string(ime.INPUTMETHOD_PINYIN_CHINESE_SIMPLIFIED)

	// Set up the test audio file.
	testFileLocation := filepath.Join(filesapp.DownloadPath, audioFile)
	if err := fsutil.CopyFile(s.DataPath(audioFile), testFileLocation); err != nil {
		s.Fatalf("Failed to copy the test image to %s: %s", testFileLocation, err)
	}
	defer os.Remove(testFileLocation)

	// Add the ime input being tested to the test device.
	if err := ime.AddAndSetInputMethod(ctx, tconn, testIME); err != nil {
		s.Fatalf("Failed to set input method to %s: %v: ", testIME, err)
	}

	// Launch inputs test web server.
	its, err := testserver.Launch(ctx, cr, tconn)
	if err != nil {
		s.Fatal("Failed to launch inputs test server: ", err)
	}
	defer its.Close()

	// Select the input field being tested.
	inputField := testserver.TextAreaInputField
	vkbCtx := vkb.NewContext(cr, tconn)

	if err := its.ClickFieldUntilVKShown(inputField)(ctx); err != nil {
		s.Fatal("Failed to show VK: ", err)
	}

	cleanup, err := input.EnableAloopInput(ctx, tconn)
	if err != nil {
		s.Fatal("Failed to enable Aloop input: ", err)
	}
	defer cleanup(ctx)
	if err = uiauto.Combine("verify audio input",
		its.Clear(inputField),
		vkbCtx.SwitchToVoiceInput(),
		func(ctx context.Context) error {
			return input.AudioFromFile(ctx, testFileLocation)
		},
		its.WaitForFieldValueToBe(inputField, expectedText),
	)(ctx); err != nil {
		s.Fatal("Failed to validate voice input: ", err)
	}
}
