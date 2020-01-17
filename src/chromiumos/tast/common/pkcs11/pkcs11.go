// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package pkcs11

import (
	"context"
	"fmt"
	"path/filepath"
	"strconv"

	"chromiumos/tast/common/hwsec"
	"chromiumos/tast/errors"
)

// PrepareScratchpadAndTestFiles prepares the scratchpad space at ScratchpadPath by ensuring that it is empty before the test, and exists after the call. Also, this creates 2 test files in it for testing.
// The path to the 2 test files are returned, and err is nil iff the operation is successful.
// This is usually called at the start of pkcs#11 related tests.
func (p *Util) PrepareScratchpadAndTestFiles(ctx context.Context, scratchpadPath string) (string, string, error) {
	// Check that the scratchpad is empty/doesn't exist.
	if _, err := p.runner.Run(ctx, "ls", scratchpadPath); err == nil {
		return "", "", errors.New("scratchpad is not empty")
	}

	// Prepare the scratchpad.
	if _, err := p.runner.Run(ctx, "mkdir", "-p", scratchpadPath); err != nil {
		return "", "", errors.Wrap(err, "failed to create scratchpad")
	}

	// Prepare the test files.
	testFile1Path := filepath.Join(scratchpadPath, "testfile1.txt")
	if _, err := p.runner.Run(ctx, "sh", "-c", fmt.Sprintf("echo test1 > %s", testFile1Path)); err != nil {
		return "", "", errors.Wrap(err, "failed to create test file 1")
	}
	testFile2Path := filepath.Join(scratchpadPath, "testfile2.txt")
	if _, err := p.runner.Run(ctx, "sh", "-c", fmt.Sprintf("echo test2 > %s", testFile2Path)); err != nil {
		return "", "", errors.Wrap(err, "failed to create test file 2")
	}

	p.scratchpadPath = scratchpadPath
	return testFile1Path, testFile2Path, nil
}

// CleanupScratchpad removes the scratchpad at ScratchpadPath. This is usually called at the end of the test.
func (p *Util) CleanupScratchpad(ctx context.Context) error {
	if p.scratchpadPath == "" {
		// Nothing to cleanup.
		return nil
	}
	if _, err := p.runner.Run(ctx, "rm", "-rf", p.scratchpadPath); err != nil {
		return errors.Wrap(err, "failed to remove the scratchpad")
	}
	return nil
}

// utilityToCryptohome is an interface used internally in this file that represents the stuffs we need from cryptohome. Usually stuffs like utilityCryptohomeBinary implements this.
type utilityToCryptohome interface {
	// GetTokenForUser retrieve the token slot for the user token if username is non-empty, or system token if username is empty.
	GetTokenForUser(ctx context.Context, username string) (int, error)
}

// Util is the class that stores the persistent state that is required to run all PKCS#11 related utility calls.
type Util struct {
	// runner is a command runner interface for executing commands on the DUT.
	runner hwsec.CmdRunner

	// utility is an interface to cryptohome for calling cryptohome related operations.
	utility utilityToCryptohome

	// chapsPath is the path to the chaps PKCS#11 module.
	chapsPath string

	// scratchpadPath is the path to the temporary scratchpad used by the test. We keep it so that we can do cleanup at the end.
	scratchpadPath string
}

// NewUtil creates a new Util.
func NewUtil(ctx context.Context, r hwsec.CmdRunner, u utilityToCryptohome) (*Util, error) {
	chapsPath, err := locateChapsModule(ctx, r)
	if err != nil {
		return nil, errors.Wrap(err, "failed to locate the chaps module")
	}
	result := &Util{runner: r, utility: u, chapsPath: chapsPath, scratchpadPath: ""}
	return result, nil
}

// locateChapsModule search for the chaps pkcs#11 module and set it in the Util struct. nil is returned iff the operation is successful.
func locateChapsModule(ctx context.Context, r hwsec.CmdRunner) (string, error) {
	// libchaps.so is on the DUT, so we'll need to use CmdRunner to probe for it.
	for _, dir := range []string{"/usr/lib64", "/usr/lib"} {
		path := filepath.Join(dir, "libchaps.so")
		if _, err := r.Run(ctx, "ls", path); err == nil {
			return path, nil
		}
	}
	return "", errors.New("chaps module not found")
}

// RunPkcs11Tool will execute "pkcs11-tool --module=chapsPath args..." on the DUT.
func (p *Util) RunPkcs11Tool(ctx context.Context, args ...string) ([]byte, error) {
	args = append([]string{"--module=" + p.chapsPath}, args...)
	return p.runner.Run(ctx, "pkcs11-tool", args...)
}

// ClearObjects remove all objects with the given ID objID in the token in slot slot and of type objType.
// objType is usually "privkey" or "cert".
func (p *Util) ClearObjects(ctx context.Context, slot int, objID string, objType string) {
	for i := 0; i < 20; i++ {
		if _, err := p.RunPkcs11Tool(ctx, "--slot="+strconv.Itoa(slot), "--delete-object", "--type", objType, "--id", objID); err != nil {
			// If we fail to delete that object, then it's already gone, so we are done.
			break
		}
	}
}

// ClearObjectsOfAllType remove all objects with the given ID objID in the token in slot slot, regardless of type.
func (p *Util) ClearObjectsOfAllType(ctx context.Context, slot int, objID string) {
	for _, t := range []string{"privkey", "pubkey", "cert", "data", "secrkey"} {
		p.ClearObjects(ctx, slot, objID, t)
	}
}

// KeyInfo stores the information for a particular key, both on disk and in chaps keystore.
type KeyInfo struct {
	// File path to the public key stored in DER format.
	pubKeyPath string

	// File path to the private key stored in DER format. This may be empty to indicate that the private key cannot be extracted.
	privKeyPath string

	// File path to the certificate in DER format.
	certPath string

	// The prefix of path for all files related to this key. This is used in cleanup. Note that this variable should not have any special characters, as this is used unescaped.
	keyPrefix string

	// The PKCS#11 token slot that holds this key.
	slot int

	// The PKCS#11 Object ID of the key.
	objID string
}

// CreateRsaSoftwareKey create a key and insert it into the system token (if username is empty), or user token specified by username. The object will have an ID of objID, and the corresponding public key will be deposited in /tmp/$keyname.key.
func (p *Util) CreateRsaSoftwareKey(ctx context.Context, utility utilityToCryptohome, username string, keyname string, objID string) (*KeyInfo, error) {
	// Get the corresponding slot.
	slot, err := utility.GetTokenForUser(ctx, username)
	if err != nil {
		return nil, errors.Wrap(err, "failed to get slot")
	}

	// Check that scratchpad is available.
	if p.scratchpadPath == "" {
		return nil, errors.New("scratchpad unavailable")
	}
	keyPrefix := filepath.Join(p.scratchpadPath, keyname)
	result := &KeyInfo{
		keyPrefix:   keyPrefix,
		privKeyPath: keyPrefix + "-priv.der",
		pubKeyPath:  keyPrefix + "-pub.der",
		certPath:    keyPrefix + "-cert.der",
		objID:       objID,
		slot:        slot,
	}

	// Create the private key and cert.
	privKeyPemPath := fmt.Sprintf("/tmp/%s-priv.key", keyname)
	certPemPath := fmt.Sprintf("/tmp/%s-cert.crt", keyname)
	if _, err := p.runner.Run(ctx, "openssl", "req", "-nodes", "-x509", "-sha1", "-newkey", "rsa:2048", "-keyout", privKeyPemPath, "-out", certPemPath, "-days", "365", "-subj", "/C=US/ST=CA/L=MTV/O=ChromiumOS/CN=chromiumos.example.com"); err != nil {
		return nil, errors.Wrap(err, "failed to create key with openssl")
	}

	// Extract the public key from the private key.
	pubKeyPemPath := fmt.Sprintf("/tmp/%s-pub.key", keyname)
	if _, err := p.runner.Run(ctx, "openssl", "rsa", "-in", privKeyPemPath, "-pubout", "-out", pubKeyPemPath); err != nil {
		return nil, errors.Wrap(err, "failed to extract public key from private key with OpenSSL")
	}

	// Convert the private key to DER format.
	if _, err := p.runner.Run(ctx, "openssl", "pkcs8", "-inform", "pem", "-outform", "der", "-in", privKeyPemPath, "-out", result.privKeyPath, "-nocrypt"); err != nil {
		return nil, errors.Wrap(err, "failed to convert private key to DER format with OpenSSL")
	}

	// Convert the public key to DER format.
	if _, err := p.runner.Run(ctx, "openssl", "rsa", "-pubin", "-inform", "pem", "-outform", "der", "-in", pubKeyPemPath, "-out", result.pubKeyPath); err != nil {
		return nil, errors.Wrap(err, "failed to convert private key to DER format with OpenSSL")
	}

	// Convert the cert to DER format.
	if _, err := p.runner.Run(ctx, "openssl", "x509", "-in", certPemPath, "-outform", "der", "-out", result.certPath); err != nil {
		return nil, errors.Wrap(err, "failed to convert cert to DER format with openssl")
	}

	// Import the private key into chaps.
	if _, err = p.runner.Run(ctx, "p11_replay", "--import", "--path="+result.privKeyPath, "--type=privkey", "--id="+result.objID); err != nil {
		return nil, errors.Wrap(err, "failed to import private key into chaps")
	}

	// Import the certificate into chaps.
	if _, err := p.runner.Run(ctx, "p11_replay", "--import", "--path="+result.certPath, "--type=cert", "--id="+result.objID); err != nil {
		return nil, errors.Wrap(err, "failed to import certificate into chaps")
	}

	return result, nil
}

// CreateRsaGeneratedKey creates a key by generating it in TPM and insert it into the system token (if username is empty), or user token specified by username. The object will have an ID of objID, and the corresponding public key will be deposited in /tmp/$keyname.key.
func (p *Util) CreateRsaGeneratedKey(ctx context.Context, utility utilityToCryptohome, username, keyname, objID string) (*KeyInfo, error) {
	// Get the corresponding slot.
	slot, err := utility.GetTokenForUser(ctx, username)
	if err != nil {
		return nil, errors.Wrap(err, "failed to get slot")
	}

	// Check that scratchpad is available.
	if p.scratchpadPath == "" {
		return nil, errors.New("scratchpad unavailable")
	}
	keyPrefix := filepath.Join(p.scratchpadPath, keyname)
	result := &KeyInfo{
		keyPrefix:   keyPrefix,
		privKeyPath: "", // No private key.
		pubKeyPath:  keyPrefix + "-pub.der",
		certPath:    "", // No certs.
		objID:       objID,
		slot:        slot,
	}

	// Generate the key.
	if _, err := p.RunPkcs11Tool(ctx, "--slot="+strconv.Itoa(slot), "--keypairgen", "--key-type", "rsa:2048", "--id="+result.objID); err != nil {
		return nil, errors.Wrap(err, "failed to generate key with pkcs11-tool")
	}

	// Export the public key.
	if _, err := p.RunPkcs11Tool(ctx, "--slot="+strconv.Itoa(slot), "--id="+result.objID, "--read-object", "--type", "pubkey", "-o", result.pubKeyPath); err != nil {
		return nil, errors.Wrap(err, "failed to generate key with pkcs11-tool")
	}

	return result, nil
}

// DestroyKey destroys the given key by removing it from disk and keystore.
func (p *Util) DestroyKey(ctx context.Context, key *KeyInfo) error {
	// Remove the objects in key store.
	p.ClearObjectsOfAllType(ctx, key.slot, key.objID)

	// Remove the on disk files.
	if _, err := p.runner.Run(ctx, "sh", "-c", fmt.Sprintf("rm -f \"%s*\"", key.keyPrefix)); err != nil {
		return errors.Wrap(err, "failed to remove on disk files starting with "+key.keyPrefix)
	}

	return nil
}

// MechanismInfo stores the information regarding a mechanism, and the various related parameters for using this mechanism with various tools such as openssl and pkcs11-tool.
// Note that there's a set of constants defined in this file for users of this struct.
type MechanismInfo struct {
	// Name used to identify this mechanism in debug message.
	name string

	// This string is fed to pkcs11-tool as -m parameter. ex. pkcs11-tool [...] -m SHA1-RSA-PKCS [...]
	toolMParam string

	// This array of string is fed to pkcs11-tool as well, after -m parameter.
	// This specify any extra arguments required.
	toolExtraParam []string

	// This is a function that'll take the input path to sign method, and a path that is actually used by pkcs11-tool.
	// This is needed because some mechanisms such as the generic RSA-PKCS-PSS takes hashed input instead of plaintext file.
	toolSignInputFileProcessor func(ctx context.Context, runner hwsec.CmdRunner, input string) string

	// The digest used by openssl dgst when we try to verify a signature of this type.
	opensslDgstParam string

	// This array of string is fed to openssl when we try to verify signatures generated by this method. ex. openssl dgst [...] -sigopt rsa_padding_mode:pss -sigopt digest:sha256 [...]
	opensslDgstExtraParam []string
}

// NoOpFileProcessor is for MechanismInfo.toolSignInputFileProcessor.
// This function does nothing to the input.
func NoOpFileProcessor(ctx context.Context, runner hwsec.CmdRunner, input string) string {
	return input
}

// mechanisms contains the list of valid mechanisms
var mechanisms = []MechanismInfo{
	// Mechanism info for PKCS#1 v1.5 signature scheme with SHA1.
	MechanismInfo{
		name:                       "SHA1-RSA-PKCS",
		toolMParam:                 "SHA1-RSA-PKCS",
		toolExtraParam:             []string{},
		toolSignInputFileProcessor: NoOpFileProcessor,
		opensslDgstParam:           "-sha1",
		opensslDgstExtraParam:      []string{},
	},
	// Mechanism info for PKCS#1 v1.5 signature scheme with SHA256.
	MechanismInfo{
		name:                       "SHA256-RSA-PKCS",
		toolMParam:                 "SHA256-RSA-PKCS",
		toolExtraParam:             []string{},
		toolSignInputFileProcessor: NoOpFileProcessor,
		opensslDgstParam:           "-sha256",
		opensslDgstExtraParam:      []string{},
	},
}

// GetMechanism returns the MechanismInfo struct with the name equal to name. It'll panic if not found.
func GetMechanism(name string) MechanismInfo {
	for _, v := range mechanisms {
		if v.name == name {
			return v
		}
	}
	panic(fmt.Sprintf("Mechanism %q not found", name))
}

// Sign sign the input and write the signature to output, using the mechanism, and signed with key.
// It'll return nil iff the signing is successful.
func (key *KeyInfo) Sign(ctx context.Context, p *Util, input string, output string, mechanism MechanismInfo) error {
	args := append([]string{"--slot=" + strconv.Itoa(key.slot), "--id=" + key.objID, "--sign", "-m", mechanism.toolMParam}, mechanism.toolExtraParam...)
	args = append(args, "-i", mechanism.toolSignInputFileProcessor(ctx, p.runner, input), "-o", output)
	if _, err := p.RunPkcs11Tool(ctx, args...); err != nil {
		return errors.Wrapf(err, "failed to sign with %s", mechanism.name)
	}

	return nil
}

// VerifyWithOpenSSL verify the signature at signaturePath, signed with key and mechanism, and the message at input.
// It'll return nil iff the signature is valid.
func (key *KeyInfo) VerifyWithOpenSSL(ctx context.Context, p *Util, input string, signaturePath string, mechanism MechanismInfo) error {
	// Verify with OpenSSL.
	args := append([]string{"dgst", mechanism.opensslDgstParam, "-verify", key.pubKeyPath, "-keyform", "der"}, mechanism.opensslDgstExtraParam...)
	args = append(args, "-signature", signaturePath, input)
	binaryMsg, err := p.runner.Run(ctx, "openssl", args...)
	if err != nil {
		return errors.Wrapf(err, "failed to verify the signature of %s", mechanism.name)
	}
	msg := string(binaryMsg)
	if msg != "Verified OK\n" {
		return errors.Errorf("failed to verify the signature of %s, because message mismatch, unexpected %q", mechanism.name, msg)
	}

	return nil
}

// SignVerify is just a convenient runner to test both signing and verification.
// altInput is path to another test file that differs in content to input. It is used to check that verify() indeed reject corrupted input.
func (key *KeyInfo) SignVerify(ctx context.Context, p *Util, input string, altInput string, mechanism MechanismInfo) error {
	// Test signing.
	if err := key.Sign(ctx, p, input, input+".sig", mechanism); err != nil {
		return err
	}
	// Test verification of signed message.
	if err := key.VerifyWithOpenSSL(ctx, p, input, input+".sig", mechanism); err != nil {
		return err
	}
	// Test verification of another message (should fail).
	if err := key.VerifyWithOpenSSL(ctx, p, altInput, input+".sig", mechanism); err == nil {
		// Should not happen.
		return errors.Errorf("verification functionality for %s failed, corrupted message is verified", mechanism.name)
	}
	return nil
}
