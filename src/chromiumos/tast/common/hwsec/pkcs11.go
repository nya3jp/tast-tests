// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package hwsec

import (
	"context"
	"os"
	"strconv"

	"chromiumos/tast/errors"
	"chromiumos/tast/testing"
)

// GetChapsCryptokiModule return the path to the chaps pkcs#11 module.
func GetChapsCryptokiModule() string {
	if _, err := os.Stat("/usr/lib64/libchaps.so"); !os.IsNotExist(err) {
		return "/usr/lib64/libchaps.so"
	}
	if _, err := os.Stat("/usr/lib/libchaps.so"); !os.IsNotExist(err) {
		return "/usr/lib/libchaps.so"
	}
	return ""
}

// Pkcs11ClearObject remove all object with the given ID |objID| in the token in slot |slot| and of type |objType|.
// |objType| is usually "privkey" or "cert".
func Pkcs11ClearObject(ctx context.Context, s *testing.State, slot int, objID string, objType string) error {
	chapsPath := GetChapsCryptokiModule()
	if chapsPath == "" {
		return errors.New("unable to find chaps module")
	}

	for i := 0; i < 20; i++ {
		_, err := Call(ctx, s, "pkcs11-tool", "--module="+chapsPath, "--slot=0", "--delete-object", "--type", objType, "--id", objID)
		if err != nil {
			// If we fail to delete that object, then it's already gone, so we are done.
			break
		}
	}
	return nil
}

// Pkcs11KeyInfo stores the information for a particular key, both on disk and in chaps keystore.
type Pkcs11KeyInfo struct {
	// File path to the public key stored in DER format.
	pubKeyPath string

	// File path to the private key stored in DER format. This may be empty to indicate that the private key cannot be extracted.
	privKeyPath string

	// File path to the certificate in DER format
	certPath string

	// The PKCS#11 token slot that holds this key.
	slot int

	// The PKCS#11 Object ID of the key.
	objID string
}

// Pkcs11CreateRsaSoftwareKey create a key and insert it into the system token (if |username| is empty), or user token specified by |username|. The object will have an ID of |objID|, and the corresponding public key will be deposited in /tmp/$keyname.key.
func Pkcs11CreateRsaSoftwareKey(ctx context.Context, s *testing.State, utility Utility, username string, keyname string, objID string) (Pkcs11KeyInfo, error) {
	result := Pkcs11KeyInfo{}
	result.privKeyPath = "/tmp/" + keyname + "-priv.der"
	result.pubKeyPath = "/tmp/" + keyname + "-pub.der"
	result.certPath = "/tmp/" + keyname + "-cert.der"
	result.objID = objID
	slot, err := utility.GetTokenForUser(username)
	if err != nil {
		return Pkcs11KeyInfo{}, errors.Wrap(err, "failed to get slot")
	}
	result.slot = slot

	// Create the private key and cert.
	_, err = Call(ctx, s, "openssl", "req", "-nodes", "-x509", "-sha1", "-newkey", "rsa:2048", "-keyout", "/tmp/"+keyname+"-priv.key", "-out", "/tmp/"+keyname+"-cert.crt", "-days", "365", "-subj", "/C=US/ST=CA/L=MTV/O=ChromiumOS/CN=chromiumos.example.com")
	if err != nil {
		return Pkcs11KeyInfo{}, errors.Wrap(err, "failed to create key with openssl")
	}

	// Extract the public key from the private key.
	_, err = Call(ctx, s, "openssl", "rsa", "-in", "/tmp/"+keyname+"-priv.key", "-pubout", "-out", "/tmp/"+keyname+"-pub.key")
	if err != nil {
		return Pkcs11KeyInfo{}, errors.Wrap(err, "failed to extract public key from private key with OpenSSL")
	}

	// Convert the private key to DER format.
	_, err = Call(ctx, s, "openssl", "pkcs8", "-inform", "pem", "-outform", "der", "-in", "/tmp/"+keyname+"-priv.key", "-out", result.privKeyPath, "-nocrypt")
	if err != nil {
		return Pkcs11KeyInfo{}, errors.Wrap(err, "failed to convert private key to DER format with OpenSSL")
	}

	// Convert the public key to DER format.
	_, err = Call(ctx, s, "openssl", "rsa", "-pubin", "-inform", "pem", "-outform", "der", "-in", "/tmp/"+keyname+"-pub.key", "-out", result.pubKeyPath)
	if err != nil {
		return Pkcs11KeyInfo{}, errors.Wrap(err, "failed to convert private key to DER format with OpenSSL")
	}

	// Convert the cert to DER format.
	_, err = Call(ctx, s, "openssl", "x509", "-in", "/tmp/"+keyname+"-cert.crt", "-outform", "der", "-out", result.certPath)
	if err != nil {
		return Pkcs11KeyInfo{}, errors.Wrap(err, "failed to convert cert to DER format with openssl")
	}

	// Import the private key into chaps
	_, err = Call(ctx, s, "p11_replay", "--import", "--path="+result.privKeyPath, "--type=privkey", "--id="+result.objID)
	if err != nil {
		return Pkcs11KeyInfo{}, errors.Wrap(err, "failed to import private key into chaps")
	}

	// Import the certificate into chaps
	_, err = Call(ctx, s, "p11_replay", "--import", "--path="+result.certPath, "--type=cert", "--id="+result.objID)
	if err != nil {
		return Pkcs11KeyInfo{}, errors.Wrap(err, "failed to import certificate into chaps")
	}

	return result, nil
}

// Pkcs11MechanismInfo stores the information regarding a mechanism, and the various related parameters for using this mechanism with various tools such as openssl and pkcs11-tool.
// Note that there's a set of constants defined in this file for users of this struct.
type Pkcs11MechanismInfo struct {
	// Name used to identify this mechanism in debug message.
	name string

	// This string is fed to pkcs11-tool as -m parameter.
	// ex. pkcs11-tool [...] -m SHA1-RSA-PKCS [...]
	toolMParam string

	// This array of string is fed to pkcs11-tool as well, after -m parameter.
	// This specify any extra arguments required.
	toolExtraParam []string

	// This is a function that'll take the input path to sign method, and a path that is actually used by pkcs11-tool.
	// This is needed because some mechanisms such as the generic RSA-PKCS-PSS takes hashed input instead of
	toolSignInputFileProcessor func(ctx context.Context, s *testing.State, input string) string

	// The digest used by openssl dgst when we try to verify a signature of this type.
	opensslDgstParam string

	// This array of string is fed to openssl when we try to verify signatures generated by this method.
	// ex. openssl dgst [...] -sigopt rsa_padding_mode:pss -sigopt digest:sha256 [...]
	opensslDgstExtraParam []string

	// Set to true if this mechanism can be used for sign and verify.
	canSignVerify bool
}

// Pkcs11NoOpFileProcessor is for Pkcs11MechanismInfo.toolSignInputFileProcessor.
// This function does nothing to the input.
func Pkcs11NoOpFileProcessor(ctx context.Context, s *testing.State, input string) string {
	return input
}

// Constants for Pkcs11MechanismInfo

// Pkcs11SHA1RSAPKCS returns a mechanism info for PKCS#1 v1.5 signature scheme + SHA1
func Pkcs11SHA1RSAPKCS() Pkcs11MechanismInfo {
	return Pkcs11MechanismInfo{
		name:                       "SHA1-RSA-PKCS",
		toolMParam:                 "SHA1-RSA-PKCS",
		toolExtraParam:             []string{},
		toolSignInputFileProcessor: Pkcs11NoOpFileProcessor,
		opensslDgstParam:           "-sha1",
		opensslDgstExtraParam:      []string{},
		canSignVerify:              true,
	}
}

// Pkcs11SHA256RSAPKCS returns a mechanism info for PKCS#1 v1.5 signature scheme + SHA256
func Pkcs11SHA256RSAPKCS() Pkcs11MechanismInfo {
	return Pkcs11MechanismInfo{
		name:                       "SHA256-RSA-PKCS",
		toolMParam:                 "SHA256-RSA-PKCS",
		toolExtraParam:             []string{},
		toolSignInputFileProcessor: Pkcs11NoOpFileProcessor,
		opensslDgstParam:           "-sha256",
		opensslDgstExtraParam:      []string{},
		canSignVerify:              true,
	}
}

// Pkcs11Sign sign the |input| and write the signature to |output|, using the |mechanism|, and signed with |key|.
// It'll return nil iff the signing is successful.
func Pkcs11Sign(ctx context.Context, s *testing.State, key Pkcs11KeyInfo, input string, output string, mechanism Pkcs11MechanismInfo) error {
	chapsPath := GetChapsCryptokiModule()
	if chapsPath == "" {
		return errors.New("unable to find chaps module")
	}

	// Remove the output first, if it exists.
	_, err := Call(ctx, s, "rm", "-f", output)
	if err != nil {
		return errors.New("failed to remove the output before signing")
	}

	args := append([]string{"--module=" + chapsPath, "--slot=" + strconv.Itoa(key.slot), "--id=" + key.objID, "--sign", "-m", mechanism.toolMParam}, mechanism.toolExtraParam...)
	args = append(args, "-i", mechanism.toolSignInputFileProcessor(ctx, s, input), "-o", output)
	_, err = Call(ctx, s, "/usr/bin/pkcs11-tool", args...)
	if err != nil {
		return errors.Wrap(err, "failed to sign with "+mechanism.name+": ")
	}

	return nil
}

// Pkcs11VerifyWithOpenSSL verify the signature at |signaturePath|, signed with |key| and |mechanism|, and the message at |input|.
// It'll return nil iff the signature is valid.
func Pkcs11VerifyWithOpenSSL(ctx context.Context, s *testing.State, key Pkcs11KeyInfo, input string, signaturePath string, mechanism Pkcs11MechanismInfo) error {
	// Verify with OpenSSL
	args := append([]string{"dgst", mechanism.opensslDgstParam, "-verify", key.pubKeyPath, "-keyform", "der"}, mechanism.opensslDgstExtraParam...)
	args = append(args, "-signature", signaturePath, input)
	binaryMsg, err := Call(ctx, s, "openssl", args...)
	if err != nil {
		return errors.Wrap(err, "failed to verify the signature of "+mechanism.name+": ")
	}
	msg := string(binaryMsg)
	if msg != "Verified OK\n" {
		return errors.New("failed to verify the signature of " + mechanism.name + ": Message mismatch, unexpected: " + msg)
	}

	return nil
}

// Pkcs11SignVerify is just a convenient helper to test both signing and verification.
// |altInput| is path to another test file that differs in content to input. It is used to check that verify() indeed reject corrupted input.
func Pkcs11SignVerify(ctx context.Context, s *testing.State, key Pkcs11KeyInfo, input string, altInput string, mechanism Pkcs11MechanismInfo) error {
	// Test signing.
	if err := Pkcs11Sign(ctx, s, key, input, input+".sig", mechanism); err != nil {
		return err
	}
	// Test verification of signed message.
	if err := Pkcs11VerifyWithOpenSSL(ctx, s, key, input, input+".sig", mechanism); err != nil {
		return err
	}
	// Test verification of another message (should fail).
	err := Pkcs11VerifyWithOpenSSL(ctx, s, key, altInput, input+".sig", mechanism)
	if err == nil {
		// Should not happen
		return errors.New("verification functionality for " + mechanism.name + " failed, corrupted message is verified")
	}
	return nil
}
