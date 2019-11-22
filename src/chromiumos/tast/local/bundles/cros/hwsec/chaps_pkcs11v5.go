// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package hwsec

import (
	"context"
	"io/ioutil"
	"os"

	libhwsec "chromiumos/tast/common/hwsec"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func: ChapsPKCS11V5,
		Desc: "Verifies PKCS#1 v1.5 works with RSA keys (sign, verify, encrypt, decrypt) in chaps",
		Attr: []string{"informational"},
		Contacts: []string{
			"zuan@chromium.org",
		},
		SoftwareDeps: []string{"chrome"},
	})
}

func getChapsCryptokiModule() string {
	if _, err := os.Stat("/usr/lib64/libchaps.so"); !os.IsNotExist(err) {
		return "/usr/lib64/libchaps.so"
	}
	if _, err := os.Stat("/usr/lib/libchaps.so"); !os.IsNotExist(err) {
		return "/usr/lib/libchaps.so"
	}
	return ""
}

func ChapsPKCS11V5(ctx context.Context, s *testing.State) {
	// Remove all previous keys/certs, if any.
	_, err := libhwsec.Call(ctx, s, "sh", "-c", "rm -f /tmp/testkey1* /tmp/testfile*")

	// Locate the chaps cryptoki module
	chapsPath := getChapsCryptokiModule()
	if chapsPath == "" {
		s.Fatal("Unable to locate chaps cryptoki module")
	}

	// Create the key.
	_, err = libhwsec.Call(ctx, s, "openssl", "req", "-nodes", "-x509", "-sha1", "-newkey", "rsa:2048", "-keyout", "/tmp/testkey1.key", "-out", "/tmp/testkey1.crt", "-days", "365", "-subj", "/C=US/ST=CA/L=MTV/O=ChromiumOS/CN=chromiumos.example.com")
	if err != nil {
		s.Fatal("Failed to create testkey1 with OpenSSL: ", err)
	}

	// Convert the cert to DER format.
	_, err = libhwsec.Call(ctx, s, "openssl", "x509", "-in", "/tmp/testkey1.crt", "-outform", "der", "-out", "/tmp/testkey1-cert.der")
	if err != nil {
		s.Fatal("Failed to convert testkey1 cert to DER format with OpenSSL: ", err)
	}

	// Extract the public key from the private key.
	_, err = libhwsec.Call(ctx, s, "openssl", "rsa", "-in", "/tmp/testkey1.key", "-pubout", "-out", "/tmp/testkey1-pub.key")
	if err != nil {
		s.Fatal("Failed to extract testkey1 public key from private key with OpenSSL: ", err)
	}

	// Convert the private key to DER format.
	_, err = libhwsec.Call(ctx, s, "openssl", "pkcs8", "-inform", "pem", "-outform", "der", "-in", "/tmp/testkey1.key", "-out", "/tmp/testkey1-priv.der", "-nocrypt")
	if err != nil {
		s.Fatal("Failed to convert testkey1 private key to DER format with OpenSSL: ", err)
	}

	// Before importing the key, clear the key store of any private key of the same id.
	for i := 0; i < 20; i++ {
		_, err = libhwsec.Call(ctx, s, "pkcs11-tool", "--module="+chapsPath, "--slot=0", "--delete-object", "--type", "privkey", "--id", "aaaaaa")
		if err != nil {
			// If we fail to delete that object, then it's already gone, so we are done.
			break
		}
	}
	// Clear the certs as well.
	for i := 0; i < 20; i++ {
		_, err = libhwsec.Call(ctx, s, "pkcs11-tool", "--module="+chapsPath, "--slot=0", "--delete-object", "--type", "cert", "--id", "aaaaaa")
		if err != nil {
			// If we fail to delete that object, then it's already gone, so we are done.
			break
		}
	}

	// Import the private key into chaps
	_, err = libhwsec.Call(ctx, s, "p11_replay", "--import", "--path=/tmp/testkey1-priv.der", "--type=privkey", "--id=aaaaaa")
	if err != nil {
		s.Fatal("Failed to import testkey1 private key into chaps: ", err)
	}

	// Import the certificate into chaps
	_, err = libhwsec.Call(ctx, s, "p11_replay", "--import", "--path=/tmp/testkey1-cert.der", "--type=cert", "--id=aaaaaa")
	if err != nil {
		s.Fatal("Failed to import testkey1 certificate into chaps: ", err)
	}

	// Create the test file
	if err = ioutil.WriteFile("/tmp/testfile1.txt", []byte("test1"), 0644); err != nil {
		s.Fatal("Failed to write test file 1")
	}
	if err = ioutil.WriteFile("/tmp/testfile2.txt", []byte("test2"), 0644); err != nil {
		s.Fatal("Failed to write test file 2")
	}

	// Test PKCS#1 v1.5 Signing with SHA1
	_, err = libhwsec.Call(ctx, s, "pkcs11-tool", "--module="+chapsPath, "--slot=0", "--id=aaaaaa", "--sign", "-m", "SHA1-RSA-PKCS", "-i", "/tmp/testfile1.txt", "-o", "/tmp/testfile1.sha1.sig")
	if err != nil {
		s.Fatal("Failed to sign with SHA1 PKCS1 v1.5: ", err)
	}

	// Verify with OpenSSL
	binaryMsg, err := libhwsec.Call(ctx, s, "openssl", "dgst", "-sha1", "-verify", "/tmp/testkey1-pub.key", "-signature", "/tmp/testfile1.sha1.sig", "/tmp/testfile1.txt")
	if err != nil {
		s.Fatal("Failed to verify the signature of SHA1 PKCS1 v1.5: ", err)
	}
	msg := string(binaryMsg)
	if msg != "Verified OK\n" {
		s.Fatal("Failed to verify the signature of SHA1 PKCS1 v1.5: Message mismatch, unexpected: ", msg)
	}

	// Check that if we verify against a different file, it'll fail.
	_, err = libhwsec.Call(ctx, s, "openssl", "dgst", "-sha1", "-verify", "/tmp/testkey1-pub.key", "-signature", "/tmp/testfile1.sha1.sig", "/tmp/testfile2.txt")
	if err == nil {
		s.Fatal("OpenSSL verifies invalid signature of SHA1 PKCS1 v1.5")
	}

	// Test PKCS#1 v1.5 Signing with SHA256
	_, err = libhwsec.Call(ctx, s, "pkcs11-tool", "--module="+chapsPath, "--slot=0", "--id=aaaaaa", "--sign", "-m", "SHA256-RSA-PKCS", "-i", "/tmp/testfile1.txt", "-o", "/tmp/testfile1.sha256.sig")
	if err != nil {
		s.Fatal("Failed to sign with SHA256 PKCS1 v1.5: ", err)
	}

	// Verify with OpenSSL
	binaryMsg, err = libhwsec.Call(ctx, s, "openssl", "dgst", "-sha256", "-verify", "/tmp/testkey1-pub.key", "-signature", "/tmp/testfile1.sha256.sig", "/tmp/testfile1.txt")
	if err != nil {
		s.Fatal("Failed to verify the signature of SHA256 PKCS1 v1.5: ", err)
	}
	msg = string(binaryMsg)
	if msg != "Verified OK\n" {
		s.Fatal("Failed to verify the signature of SHA256 PKCS1 v1.5: Message mismatch, unexpected: ", msg)
	}

	// Check that if we verify against a different file, it'll fail.
	_, err = libhwsec.Call(ctx, s, "openssl", "dgst", "-sha256", "-verify", "/tmp/testkey1-pub.key", "-signature", "/tmp/testfile1.sha256.sig", "/tmp/testfile2.txt")
	if err == nil {
		s.Fatal("OpenSSL verifies invalid signature of SHA256 PKCS1 v1.5")
	}
}
