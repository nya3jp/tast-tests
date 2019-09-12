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
	"chromiumos/tast/local/chrome"
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

type testEnvironment struct {
	ctx               context.Context
	s                 *testing.State
	cr                *chrome.Chrome
	a                 *arc.ARC
	d                 *ui.Device
	storePathChromeOS string
	storePathArc      string
}

// General helper functions.

func buttonID(buttonName string) string {
	return idPrefix + buttonName
}

func must(s *testing.State, err error) {
	if err != nil {
		s.Fatal("Failed: ", err)
	}
}

// Key store helper functions.

func fileExists(filename string) bool {
	info, err := os.Stat(filename)
	if os.IsNotExist(err) {
		return false
	}
	return !info.IsDir()
}

func bootstrapCommandOut(env *testEnvironment, name string, arg ...string) string {
	out, err := arc.BootstrapCommand(env.ctx, name, arg...).Output(testexec.DumpLogOnError)
	if err != nil {
		env.s.Fatalf("%s failed: %s", name, err)
	}

	return string(out)
}

func arcDeleteTestKey(env *testEnvironment, keySuffix string) {
	arc.BootstrapCommand(env.ctx, "/system/bin/rm", env.storePathArc+"*"+keySuffix).Run()
}

func crosDeleteAllKeys(env *testEnvironment) {
	files, err := filepath.Glob(filepath.Join(env.storePathChromeOS, "*"))
	if err != nil {
		env.s.Fatalf("Failed to delete Chrome OS keys: %s", err)
	}
	for _, file := range files {
		err = os.RemoveAll(file)
		if err != nil {
			env.s.Fatalf("Failed to delete Chrome OS key %s: %s", file, err)
		}
	}
}

func arcCheckKeyExists(env *testEnvironment, keySuffix string) string {
	privateKeySuffix := "USRPKEY_" + keySuffix
	certSuffix := "USRCERT_" + keySuffix

	out := bootstrapCommandOut(env, "/system/bin/ls", env.storePathArc)

	files := strings.Split(out, "\n")

	certFound := false
	privateKeyFound := false
	privateKeyFilename := ""

	for _, filename := range files {
		filename = strings.TrimSpace(filename)
		if strings.HasSuffix(filename, privateKeySuffix) {
			if privateKeyFound {
				env.s.Fatal("Multiple number of private keys in ARC were generated")
			}
			privateKeyFound = true
			privateKeyFilename = filename
		} else if strings.HasSuffix(filename, certSuffix) {
			if certFound {
				env.s.Fatal("Multiple number of certificate files in ARC were generated")
			}
			certFound = true
		}
	}

	if !certFound {
		env.s.Fatalf("Certificate file for %s was not found", keySuffix)
	}

	if !privateKeyFound {
		env.s.Fatalf("Private key for %s was not found", keySuffix)
	}

	return privateKeyFilename
}

func arcRetrieveFilenameFromReference(env *testEnvironment, referenceFilename string) string {
	out := bootstrapCommandOut(env, "/system/bin/cat", env.storePathArc+referenceFilename)
	data := strings.TrimSpace(string(out))
	dataLen := len(data)
	if dataLen < filenameLength {
		env.s.Fatalf("Invalid reference content: too short: %d", dataLen)
	}
	return data[dataLen-filenameLength:]
}

func crosCheckKeyExists(env *testEnvironment, filename string) {
	keyPath := env.storePathChromeOS + filename
	if !fileExists(keyPath) {
		env.s.Fatalf("Key matter in Chrome OS was not found at path: %s", keyPath)
	}
}

// UI helper functions.

func checkStatusOk(env *testEnvironment, buttonName string, errMsg string) {
	env.s.Log("Wait for status update")
	must(env.s, env.d.Object(ui.ID(textViewStatusID), ui.TextContains(buttonName)).WaitForExists(env.ctx, 30*time.Second))

	statusText, err := env.d.Object(ui.ID(textViewStatusID)).GetText(env.ctx)
	if err != nil {
		env.s.Fatal("Failed to retrieve status: ", err)
	}

	env.s.Logf("Got status: %s", statusText)

	splitList := strings.Split(statusText, "; ")
	status := splitList[len(splitList)-1]

	if status != "Status: OK" {
		env.s.Fatalf("%s: status: failed", errMsg)
	}
}

func testButton(env *testEnvironment, buttonName string, errMsg string) {
	must(env.s, env.d.Object(ui.ID(buttonID(buttonName))).Click(env.ctx))
	checkStatusOk(env, buttonName, errMsg)
}

func testGenerateKeyButton(env *testEnvironment, buttonName string, keySuffix string, errMsg string) {
	arcDeleteTestKey(env, keySuffix)
	crosDeleteAllKeys(env)
	testButton(env, buttonName, errMsg)

	referenceFilename := arcCheckKeyExists(env, keySuffix)
	keyFilename := arcRetrieveFilenameFromReference(env, referenceFilename)
	crosCheckKeyExists(env, keyFilename)
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
		s.Fatal("Failed initializing UI Automator: ", err)
	}
	defer d.Close()

	env := &testEnvironment{
		ctx:               ctx,
		s:                 s,
		cr:                cr,
		a:                 a,
		d:                 d,
		storePathChromeOS: "/run/daemon-store/arc-keymasterd/" + userHash + "/key_blobs/",
		storePathArc:      "/data/misc/keystore/user_0/",
	}

	// Start App.
	s.Log("Starting App")
	if err := a.Install(ctx, s.DataPath(apk)); err != nil {
		s.Fatal("Failed installing app: ", err)
	}

	if err := a.Command(ctx, "am", "start", "-W", pkg+"/"+cls).Run(); err != nil {
		s.Fatal("Failed starting app: ", err)
	}

	must(s, d.Object(ui.ID(textViewStatusHeaderID), ui.TextContains("Status")).WaitForExists(ctx, 30*time.Second))

	// Test keymaster logics.
	testGenerateKeyButton(env, buttonGenerateSignVerify, keySuffixSignVerify, "Failed to generate key for signing/verifying")

	testButton(env, buttonFindSignVerify, "Failed to find key for signing/verifying")

	testButton(env, buttonSign, "Failed to sign data")

	testButton(env, buttonVerify, "Failed to verify data")

	testGenerateKeyButton(env, buttonGenerateEncryptDecrypt, keySuffixEncryptDecrypt, "Failed to generate key for encrypting/decrypting")

	testButton(env, buttonFindEncryptDecrypt, "Failed to find key for encrypting/decrypting")

	testButton(env, buttonEncrypt, "Failed to encrypt data")

	testButton(env, buttonDecrypt, "Failed to decrypt data")
}
