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

	"chromiumos/tast/local/arc"
	"chromiumos/tast/local/arc/ui"
	"chromiumos/tast/local/cryptohome"
	"chromiumos/tast/local/testexec"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         Keymaster,
		Desc:         "Test arc-keymasterd storing keys in Chrome OS properly",
		Contacts:     []string{"edmanp@google.com", "deserg@google.com", "arc-framework+tast@google.com"},
		Attr:         []string{"informational"},
		SoftwareDeps: []string{"android_both", "chrome"},
		Data:         []string{"KeystoreExerciser.apk"},
		Pre:          arc.Booted(),
	})
}

func fileExists(filename string) bool {
	info, err := os.Stat(filename)
	if os.IsNotExist(err) {
		return false
	}
	return !info.IsDir()
}

func Keymaster(ctx context.Context, s *testing.State) {
	// Retrieve Chrome and ARC objects.
	cr := s.PreValue().(arc.PreData).Chrome
	a := s.PreValue().(arc.PreData).ARC

	// Test constants.
	const (
		apk = "KeystoreExerciser.apk"
		pkg = "io.demo.edmanp.keystoreexerciser"
		cls = "io.demo.edmanp.keystoreexerciser.MainActivity"

		idPrefix = "io.demo.edmanp.keystoreexerciser:id/"

		buttonGenerateSignVerify = "button_gen_sign_verify"
		buttonFindSignVerify     = "button_find_sign_verify"
		buttonSign               = "button_sign"
		buttonVerify             = "button_verify"

		buttonGenerateEncryptDecrypt = "button_gen_encrypt_decrypt"
		buttonFindEncryptDecrypt     = "button_find_encrypt_decrypt"
		buttonEncrypt                = "button_encrypt"
		buttonDecrypt                = "button_decrypt"

		buttonClear = "button_clear"

		textViewStatusHeaderID = idPrefix + "textView"
		textViewStatusID       = idPrefix + "textView_status"

		keySuffixSignVerify     = "demo_generated_sign_verify"
		keySuffixEncryptDecrypt = "demo_generated_encrypt_decrypt"

		referenceContentsPadding = 16
		filenameLength           = 16
	)

	// Helper functions.
	buttonID := func(buttonName string) string {
		return idPrefix + buttonName
	}

	must := func(err error) {
		if err != nil {
			s.Fatal("Failed: ", err)
		}
	}

	mustReturn := func(val string, err error) string {
		if err != nil {
			s.Fatal("Failed: ", err)
		}
		return val
	}

	// Initialize key store paths.
	userHash, err := cryptohome.UserHash(ctx, cr.User())
	if err != nil {
		s.Fatalf("Failed to find user hash for %s: %v", cr.User(), err)
	}
	s.Logf("Userhash: %s", userHash)

	KeysStoreChromeOS := "/run/daemon-store/arc-keymasterd/" + userHash + "/key_blobs/"
	KeysStoreArc := "/data/misc/keystore/user_0/"

	// Key store helper functions.
	bootstrapCommandOut := func(name string, arg ...string) string {
		out, err := arc.BootstrapCommand(ctx, name, arg...).Output(testexec.DumpLogOnError)
		if err != nil {
			s.Fatalf("%s failed: %s", name, err)
		}

		return string(out)
	}

	arcDeleteTestKey := func(keySuffix string) {
		arc.BootstrapCommand(ctx, "/system/bin/rm", KeysStoreArc+"*"+keySuffix).Run()
	}

	crosDeleteAllKeys := func() {
		files, err := filepath.Glob(filepath.Join(KeysStoreChromeOS, "*"))
		if err != nil {
			s.Fatalf("Failed to delete Chrome OS keys: %s", err)
		}
		for _, file := range files {
			err = os.RemoveAll(file)
			if err != nil {
				s.Fatalf("Failed to delete Chrome OS key %s: %s", file, err)
			}
		}
	}

	arcCheckKeyExists := func(keySuffix string) string {
		privateKeySuffix := "USRPKEY_" + keySuffix
		certSuffix := "USRCERT_" + keySuffix

		out := bootstrapCommandOut("/system/bin/ls", KeysStoreArc)

		files := strings.Split(out, "\n")

		certFound := false
		privateKeyFound := false
		privateKeyFilename := ""

		for _, filename := range files {
			filename = strings.TrimSpace(filename)
			if strings.HasSuffix(filename, privateKeySuffix) {
				if privateKeyFound {
					s.Fatal("Multiple number of private keys in ARC were generated")
				}
				privateKeyFound = true
				privateKeyFilename = filename
			} else if strings.HasSuffix(filename, certSuffix) {
				if certFound {
					s.Fatal("Multiple number of certificate files in ARC were generated")
				}
				certFound = true
			}
		}

		if !certFound {
			s.Fatalf("Certificate file for %s was not found", keySuffix)
		}

		if !privateKeyFound {
			s.Fatalf("Private key for %s was not found", keySuffix)
		}

		return privateKeyFilename
	}

	arcRetrieveFilenameFromReference := func(referenceFilename string) string {
		out := bootstrapCommandOut("/system/bin/cat", KeysStoreArc+referenceFilename)
		data := strings.TrimSpace(string(out))
		dataLen := len(data)
		if dataLen < filenameLength {
			s.Fatalf("Invalid reference content: too short: %d", dataLen)
		}
		return data[dataLen-filenameLength:]
	}

	crosCheckKeyExists := func(filename string) {
		keyPath := KeysStoreChromeOS + filename
		if !fileExists(keyPath) {
			s.Fatalf("Key matter in Chrome OS was not found at path: %s", keyPath)
		}
	}

	// Initialize UI Automator.
	d, err := ui.NewDevice(ctx, a)
	if err != nil {
		s.Fatal("Failed initializing UI Automator: ", err)
	}
	defer d.Close()

	// UI helper functions.
	checkStatusOk := func(buttonName string, errMsg string) {
		s.Log("Wait for status update")
		must(d.Object(ui.ID(textViewStatusID), ui.TextContains(buttonName)).WaitForExists(ctx, 30*time.Second))

		statusText := mustReturn(d.Object(ui.ID(textViewStatusID)).GetText(ctx))
		s.Logf("Got status: %s", statusText)

		splitList := strings.Split(statusText, "; ")
		status := splitList[len(splitList)-1]

		if status != "Status: OK" {
			s.Fatalf("%s: status: failed", errMsg)
		}
	}

	testButton := func(buttonName string, errMsg string) {
		must(d.Object(ui.ID(buttonID(buttonName))).Click(ctx))
		checkStatusOk(buttonName, errMsg)
	}

	testGenerateKeyButton := func(buttonName string, keySuffix string, errMsg string) {
		arcDeleteTestKey(keySuffix)
		crosDeleteAllKeys()
		testButton(buttonName, errMsg)

		referenceFilename := arcCheckKeyExists(keySuffix)
		keyFilename := arcRetrieveFilenameFromReference(referenceFilename)
		crosCheckKeyExists(keyFilename)
	}

	// Start App.
	s.Log("Starting App")
	if err := a.Install(ctx, s.DataPath(apk)); err != nil {
		s.Fatal("Failed installing app: ", err)
	}

	if err := a.Command(ctx, "am", "start", "-W", pkg+"/"+cls).Run(); err != nil {
		s.Fatal("Failed starting app: ", err)
	}

	// Wait for App showing up.
	must(d.Object(ui.ID(textViewStatusHeaderID), ui.TextContains("Status")).WaitForExists(ctx, 30*time.Second))

	// Test buttons logics.
	testGenerateKeyButton(buttonGenerateSignVerify, keySuffixSignVerify, "Failed to generate key for signing/verifying")

	testButton(buttonFindSignVerify, "Failed to find key for signing/verifying")

	testButton(buttonSign, "Failed to sign data")

	testButton(buttonVerify, "Failed to verify data")

	testGenerateKeyButton(buttonGenerateEncryptDecrypt, keySuffixEncryptDecrypt, "Failed to generate key for encrypting/decrypting")

	testButton(buttonFindEncryptDecrypt, "Failed to find key for encrypting/decrypting")

	testButton(buttonEncrypt, "Failed to encrypt data")

	testButton(buttonDecrypt, "Failed to decrypt data")
}
