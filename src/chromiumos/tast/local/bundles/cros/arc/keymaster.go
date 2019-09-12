// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package arc

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"time"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/arc"
	"chromiumos/tast/local/arc/ui"
	"chromiumos/tast/local/cryptohome"
	"chromiumos/tast/local/testexec"
	"chromiumos/tast/testing"
)

// Test constants.
const (
	apk           = "KeymasterTest.apk"
	componentName = "test.keymaster/test.keymaster.MainActivity"

	idPrefix = "test.keymaster:id/"

	textViewStatusID = idPrefix + "textViewStatus"

	keySuffixSignVerify     = "demo_generated_sign_verify"
	keySuffixEncryptDecrypt = "demo_generated_encrypt_decrypt"

	keystoreHeaderLength = 40
	filenameLength       = 16

	storePathArc = "/data/misc/keystore/user_0/"
)

type testParams struct {
	keySuffix string
	buttonID  string
}

func init() {
	testing.AddTest(&testing.Test{
		Func:     Keymaster,
		Desc:     "Test that keys of ARC apps are stored in Chrome OS",
		Contacts: []string{"edmanp@google.com", "deserg@google.com", "arc-framework+tast@google.com"},
		Params: []testing.Param{{
			Name: "test_params_sign_verify",
			Val: testParams{
				keySuffix: "demo_generated_sign_verify",
				buttonID:  idPrefix + "buttonRunTestsSignVerify",
			},
		}, {
			Name: "test_params_encrypt_decrypt",
			Val: testParams{
				keySuffix: "demo_generated_encrypt_decrypt",
				buttonID:  idPrefix + "buttonRunTestsEncryptDecrypt",
			},
		}},
		Attr:         []string{"informational"},
		SoftwareDeps: []string{"android_p", "chrome"},
		Data:         []string{"KeymasterTest.apk"},
		Pre:          arc.Booted(),
	})
}

func Keymaster(ctx context.Context, s *testing.State) {
	// Setup.
	cr := s.PreValue().(arc.PreData).Chrome
	a := s.PreValue().(arc.PreData).ARC

	userHash, err := cryptohome.UserHash(ctx, cr.User())
	if err != nil {
		s.Fatal("Failed to retrieve user hash: ", err)
	}

	d, err := ui.NewDevice(ctx, a)
	if err != nil {
		s.Fatal("Failed to initialize UI Automator: ", err)
	}
	defer d.Close()

	storePathCros := filepath.Join("/run/daemon-store/arc-keymasterd/", userHash, "/key_blobs/")

	// Start App.
	s.Log("Starting App")
	if err := a.Install(ctx, s.DataPath(apk)); err != nil {
		s.Fatal("Failed installing app: ", err)
	}

	if err := a.Command(ctx, "am", "start", "-W", componentName).Run(); err != nil {
		s.Fatal("Failed starting app: ", err)
	}

	appUID, err := getTestAppUID(ctx, a)
	if err != nil {
		s.Fatal("Failed to retrieve app id: ", err)
	}
	keySuffix := s.Param().(testParams).keySuffix
	arcPrivateKeyPath := filepath.Join(storePathArc, appUID+"_USRPKEY_"+keySuffix)

	err = d.Object(ui.ID(textViewStatusID)).WaitForExists(ctx, 30*time.Second)
	if err != nil {
		s.Fatal("Failed to run app: ", err)
	}

	// Remove key files to ensure test correctness.
	err = crosDeleteAllFilesInDirectory(storePathCros)
	if err != nil {
		s.Fatal("Failed remove old key files in Chrome OS: ", err)
	}

	// Execute.
	buttonID := s.Param().(testParams).buttonID
	err = d.Object(ui.ID(buttonID)).Click(ctx)
	if err != nil {
		s.Fatal("Failed to click the button for running tests: ", err)
	}

	s.Log("Wait for status update")
	err = d.Object(ui.ID(textViewStatusID), ui.TextContains("Status")).WaitForExists(ctx, 30*time.Second)
	if err != nil {
		s.Fatal("Test execution did not end correctly: could not get a proper text from the status text field: ", err)
	}

	statusText, err := d.Object(ui.ID(textViewStatusID)).GetText(ctx)
	if err != nil {
		s.Fatal("Failed to retrieve status: ", err)
	}

	s.Logf("Got status: %s", statusText)

	if statusText != "Status: OK" {
		s.Fatal("Run tests status: failed")
	}

	// Verify.
	err = verifyKeyGenerated(ctx, arcPrivateKeyPath, storePathCros)
	if err != nil {
		s.Fatal("Failed to verify if keys were stored correctly: ", err)
	}
}

// Helper functions.

func verifyKeyGenerated(ctx context.Context, arcPrivateKeyPath string, storePathCros string) error {

	keyFilename, err := arcRetrieveFilenameFromReference(ctx, arcPrivateKeyPath)
	if err != nil {
		return errors.Wrapf(err, "failed to retrieve key filename from reference %s", arcPrivateKeyPath)
	}

	err = crosCheckFileExistsInDirectory(storePathCros, keyFilename)
	if err != nil {
		return errors.Wrapf(err, "failed to check if key at path %s exists", keyFilename)
	}

	return nil
}

func getTestAppUID(ctx context.Context, a *arc.ARC) (string, error) {
	out, err := a.Command(ctx, "pm", "list", "packages", "-U", "test.keymaster").Output(testexec.DumpLogOnError)

	if err != nil {
		return "", errors.Wrap(err, "failed to list packages")
	}

	pmInfo := strings.TrimSpace(string(out))
	idIndex := strings.Index(pmInfo, "uid:")
	if idIndex == -1 {
		return "", errors.New("app uid not found in pm list packages")
	}

	return pmInfo[idIndex+4:], nil
}

func arcRetrieveFilenameFromReference(ctx context.Context, referenceFilepath string) (string, error) {
	out, err := arc.BootstrapCommand(ctx, "/system/bin/cat", referenceFilepath).Output(testexec.DumpLogOnError)
	if err != nil {
		return "", errors.Wrap(err, "failed to get reference contents")
	}

	data := strings.TrimSpace(string(out))
	dataLength := len(data)
	if dataLength != (keystoreHeaderLength + filenameLength) {
		return "", errors.New("invalid length of reference content")
	}
	return data[keystoreHeaderLength:], nil
}

func crosDeleteAllFilesInDirectory(directoryPath string) error {
	files, err := filepath.Glob(filepath.Join(directoryPath, "*"))
	if err != nil {
		return errors.Wrap(err, "failed to list Chrome OS keys for deletion")
	}
	for _, file := range files {
		err = os.Remove(file)
		if err != nil {
			return errors.Wrapf(err, "failed to delete Chrome OS key %s", file)
		}
	}
	return nil
}

func crosCheckFileExistsInDirectory(directoryPath string, filename string) error {
	filepath := filepath.Join(directoryPath, filename)

	info, err := os.Stat(filepath)
	if os.IsNotExist(err) || info.IsDir() {
		return errors.Errorf("File in Chrome OS was not found at path: %s", filepath)
	}

	return nil
}
