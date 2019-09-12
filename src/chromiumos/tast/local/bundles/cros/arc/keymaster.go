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

func init() {
	testing.AddTest(&testing.Test{
		Func:         Keymaster,
		Desc:         "Test that keys of ARC apps are stored in Chrome OS",
		Contacts:     []string{"edmanp@google.com", "deserg@google.com", "arc-framework+tast@google.com"},
		Attr:         []string{"informational"},
		SoftwareDeps: []string{"android_p", "chrome"},
		Data:         []string{"KeystoreExerciser.apk"},
		Pre:          arc.Booted(),
	})
}

// Test constants.
const (
	apk           = "KeystoreExerciser.apk"
	componentName = "demo.keystoreexerciser/demo.keystoreexerciser.MainActivity"

	idPrefix = "demo.keystoreexerciser:id/"

	buttonRunTestsID = idPrefix + "buttonRunTests"

	textViewStatusHeaderID = idPrefix + "textViewStatusHeader"
	textViewStatusID       = idPrefix + "textViewStatus"

	keySuffixSignVerify     = "demo_generated_sign_verify"
	keySuffixEncryptDecrypt = "demo_generated_encrypt_decrypt"

	rmNoSuchFileError = "No such file or directory"

	filenameLength = 16

	storePathArc = "/data/misc/keystore/user_0/"
)

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

	appID, err := retrieveAppID(ctx, a)
	if err != nil {
		s.Fatal("Failed retrieve app id: ", err)
	}
	s.Logf("App ID: %s", appID)
	arcPrivateKeyPathPrefix := filepath.Join(storePathArc, appID+"_USRPKEY_")
	arcCertSignPathPrefix := filepath.Join(storePathArc, appID+"_USRCERT_")

	must(s, d.Object(ui.ID(textViewStatusHeaderID), ui.TextContains("Status")).WaitForExists(ctx, 30*time.Second))

	// Remove key files to ensure test correctness. Error check is not needed
	// because files may not exist.
	must(s, arcDeleteKey(ctx, arcPrivateKeyPathPrefix, arcCertSignPathPrefix, keySuffixSignVerify))
	must(s, arcDeleteKey(ctx, arcPrivateKeyPathPrefix, arcCertSignPathPrefix, keySuffixEncryptDecrypt))
	must(s, crosDeleteAllKeys(storePathCros))

	// Execute.
	must(s, d.Object(ui.ID(buttonRunTestsID)).Click(ctx))

	s.Log("Wait for status update")
	must(s, d.Object(ui.ID(textViewStatusID), ui.TextContains("Status")).WaitForExists(ctx, 30*time.Second))

	statusText, err := d.Object(ui.ID(textViewStatusID)).GetText(ctx)
	if err != nil {
		s.Fatal("Failed to retrieve status: ", err)
	}

	s.Logf("Got status: %s", statusText)

	if statusText != "Status: OK" {
		s.Fatal("Run tests status: failed")
	}

	// Verify.
	must(s, verifyKeyGenerated(ctx, arcPrivateKeyPathPrefix, arcCertSignPathPrefix, storePathCros, keySuffixSignVerify))
	must(s, verifyKeyGenerated(ctx, arcPrivateKeyPathPrefix, arcCertSignPathPrefix, storePathCros, keySuffixEncryptDecrypt))
}

// Helper functions.

func must(s *testing.State, err error) {
	if err != nil {
		s.Fatal("Failed: ", err)
	}
}

func retrieveAppID(ctx context.Context, a *arc.ARC) (string, error) {
	out, err := a.Command(ctx, "pm", "list", "packages", "-U", "demo.keystoreexerciser").Output(testexec.DumpLogOnError)

	if err != nil {
		return "", errors.Wrap(err, "failed to retrieve app pm info")
	}

	pmInfo := strings.TrimSpace(string(out))
	idIndex := strings.Index(pmInfo, "uid:")
	if idIndex == -1 {
		return "", errors.New("app id was not found in app pm info")
	}

	return pmInfo[idIndex+4:], nil
}

func arcDeleteKey(ctx context.Context, arcPrivatePathPrefix string, arcCertPathPrefix string, keySuffix string) error {
	err := arc.BootstrapCommand(ctx, "/system/bin/rm", "-f", arcPrivatePathPrefix+keySuffix).Run()
	if err != nil {
		return errors.Wrapf(err, "failed to remove %s private key", keySuffix)
	}

	err = arc.BootstrapCommand(ctx, "/system/bin/rm", "-f", arcCertPathPrefix+keySuffix).Run()
	if err != nil {
		return errors.Wrapf(err, "failed to remove %s certificate", keySuffix)
	}

	return nil
}

func crosDeleteAllKeys(storePathCros string) error {
	files, err := filepath.Glob(filepath.Join(storePathCros, "*"))
	if err != nil {
		return errors.Wrap(err, "failed to delete Chrome OS keys")
	}
	for _, file := range files {
		err = os.RemoveAll(file)
		if err != nil {
			return errors.Wrapf(err, "failed to delete Chrome OS key %s", file)
		}
	}

	return nil
}

func arcRetrieveFilenameFromReference(ctx context.Context, referenceFilepath string) (string, error) {
	out, err := arc.BootstrapCommand(ctx, "/system/bin/cat", referenceFilepath).Output(testexec.DumpLogOnError)
	if err != nil {
		return "", errors.Wrap(err, "failed to get reference contents")
	}

	// Private key contents is stored with an auto-generated prefix so it's
	// necessary to check if string is long enough and to extract the data from
	// the string end
	data := strings.TrimSpace(string(out))
	dataLen := len(data)
	if dataLen < filenameLength {
		return "", errors.Errorf("invalid reference content: too short: %d", dataLen)
	}
	return data[dataLen-filenameLength:], nil
}

func crosCheckKeyExists(storePathCros string, filename string) error {
	keyPath := filepath.Join(storePathCros, filename)

	info, err := os.Stat(keyPath)
	if os.IsNotExist(err) || info.IsDir() {
		return errors.Errorf("key matter in Chrome OS was not found at path: %s", keyPath)
	}

	return nil
}

func verifyKeyGenerated(ctx context.Context, arcPrivateKeyPathPrefix string, arcCertPathPrefix string, storePathCros string, keySuffix string) error {

	arcPrivateKeyFilepath := arcPrivateKeyPathPrefix + keySuffix
	arcCertFilepath := arcCertPathPrefix + keySuffix

	err := arc.BootstrapCommand(ctx, "/system/bin/ls", arcPrivateKeyFilepath).Run()
	if err != nil {
		return errors.Wrapf(err, "private key for %s does not exist", keySuffix)
	}

	err = arc.BootstrapCommand(ctx, "/system/bin/ls", arcCertFilepath).Run()
	if err != nil {
		return errors.Wrapf(err, "certificate for %s does not exist", keySuffix)
	}

	keyFilename, err := arcRetrieveFilenameFromReference(ctx, arcPrivateKeyFilepath)
	if err != nil {
		return errors.Wrapf(err, "failed to retrieve key filename from reference %s", arcPrivateKeyFilepath)
	}

	err = crosCheckKeyExists(storePathCros, keyFilename)
	if err != nil {
		return errors.Wrapf(err, "failed to check if key at path %s exists", keyFilename)
	}

	return nil
}
