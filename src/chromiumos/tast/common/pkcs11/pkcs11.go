// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package pkcs11

import (
	"context"
	"fmt"
	"path/filepath"
	"strconv"
	"strings"

	"chromiumos/tast/common/hwsec"
	"chromiumos/tast/errors"
	"chromiumos/tast/testing"
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
	f1 := filepath.Join(scratchpadPath, "testfile1.txt")
	if _, err := p.runner.Run(ctx, "sh", "-c", fmt.Sprintf("echo test1 > %s", f1)); err != nil {
		return "", "", errors.Wrap(err, "failed to create test file 1")
	}
	f2 := filepath.Join(scratchpadPath, "testfile2.txt")
	if _, err := p.runner.Run(ctx, "sh", "-c", fmt.Sprintf("echo test2 > %s", f2)); err != nil {
		return "", "", errors.Wrap(err, "failed to create test file 2")
	}

	p.scratchpadPath = scratchpadPath
	return f1, f2, nil
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
func (p *Util) ClearObjects(ctx context.Context, slot int, objID string, objType string) error {
	// We try to delete the key 20 times, an anecdotally chosen number,
	// because we don't usually encounter 20 objects with the same ID.
	for i := 0; i < 20; i++ {
		if _, err := p.RunPkcs11Tool(ctx, "--slot="+strconv.Itoa(slot), "--delete-object", "--type", objType, "--id", objID); err != nil {
			// If we fail to delete that object, then it's already gone, so we are done.
			return nil
		}
	}
	// If we get here, there's either more than 20 objects with the same ID (unlikely),
	// or something is wrong with --delete-object.
	return errors.Errorf("failed to remove object of type %q and ID %q in slot %d", objType, objID, slot)
}

// ClearObjectsOfAllType remove all objects with the given ID objID in the token in slot slot, regardless of type.
func (p *Util) ClearObjectsOfAllType(ctx context.Context, slot int, objID string) error {
	for _, t := range []string{"privkey", "pubkey", "cert", "data", "secrkey"} {
		if err := p.ClearObjects(ctx, slot, objID, t); err != nil {
			return errors.Wrapf(err, "failed to clear objects of type %q", t)
		}
	}
	return nil
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

	// NOTE: If any reference type is added in the future, modify CreateCopiedKey to deep copy.
}

// CreateRSASoftwareKey create a key and insert it into the system token (if username is empty), or user token specified by username. The object will have an ID of objID, and the corresponding public key will be deposited in /tmp/$keyname.key.
func (p *Util) CreateRSASoftwareKey(ctx context.Context, utility utilityToCryptohome, username string, keyname string, objID string, forceSoftwareBacked bool, checkSoftwareBacked bool) (*KeyInfo, error) {
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
	args := []string{"--import", "--path=" + result.privKeyPath, "--type=privkey", "--id=" + result.objID}
	if forceSoftwareBacked {
		args = append(args, "--force_software")
	}
	if _, err := p.runner.Run(ctx, "p11_replay", args...); err != nil {
		return nil, errors.Wrap(err, "failed to import private key into chaps")
	}

	// Import the certificate into chaps.
	if _, err := p.runner.Run(ctx, "p11_replay", "--import", "--path="+result.certPath, "--type=cert", "--id="+result.objID); err != nil {
		return nil, errors.Wrap(err, "failed to import certificate into chaps")
	}

	// If required, check that it's software backed or not.
	if checkSoftwareBacked {
		isSoftwareBacked, err := result.IsSoftwareBacked(ctx, p)
		if err != nil {
			return nil, errors.Wrap(err, "failed to get kKeyInSoftware attribute")
		}

		if isSoftwareBacked != forceSoftwareBacked {
			return nil, errors.Errorf("mismatch in kKeyInSoftware attribute (%t) and force software backed parameter (%t)", isSoftwareBacked, forceSoftwareBacked)
		}
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

// CreateCopiedKey creates a copy of origKey and sets its CKA_ID to objID, and other attributes according to attributes map. It returns (key, message, err) tuple, whereby err is nil iff the operation is successful. key is the new key and message is the stdout of p11_replay command where available.
func (p *Util) CreateCopiedKey(ctx context.Context, origKey *KeyInfo, objID string, attributes map[string]string) (*KeyInfo, string, error) {
	// Set the object ID.
	attributes["CKA_ID"] = objID

	// Generate the attribute string.
	attributesList := make([]string, 0, len(attributes))
	for k, v := range attributes {
		attributesList = append(attributesList, k+":"+v)
	}
	attributesParam := strings.Join(attributesList, ",")

	binaryMsg, err := p.runner.Run(ctx, "p11_replay", "--copy_object", "--slot="+strconv.Itoa(origKey.slot), "--id="+origKey.objID, "--attr_list="+attributesParam, "--type=privkey")
	var msg string
	if binaryMsg != nil {
		msg = string(binaryMsg)
	}
	if err != nil {
		testing.ContextLog(ctx, "p11_replay failed with: ", msg)
		return nil, msg, errors.Wrap(err, "failed to run p11_replay")
	}

	const p11replayCopyObjectSuccessMsg = "Operation completed successfully."

	if !strings.Contains(msg, p11replayCopyObjectSuccessMsg) {
		testing.ContextLog(ctx, "p11_replay failed with: ", msg)
		return nil, msg, errors.New("incorrect response from p11_replay")
	}

	newKey := *origKey
	newKey.objID = objID

	return &newKey, msg, nil
}

// DestroyKey destroys the given key by removing it from disk and keystore.
func (key *KeyInfo) DestroyKey(ctx context.Context, p *Util) error {
	// Remove the objects in key store.
	if err := p.ClearObjectsOfAllType(ctx, key.slot, key.objID); err != nil {
		return errors.Wrap(err, "failed to remove objects")
	}

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

	// toolMParam field specifies the PKCS#11 mechanism code (algorithm+scheme) used for this set of mechanism configurations. Technically, this string is fed to pkcs11-tool as -m parameter. ex. pkcs11-tool [...] -m SHA1-RSA-PKCS [...]
	toolMParam string

	// toolExtraParam is a set of parameters for the mechanisms used if the mechanism specified in toolMParam have any such accompanying parameters outside of the PKCS#11 mechanism code.
	// Technically, this array of string is fed to pkcs11-tool as well, after -m parameter.
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

// HashFileProcessor is for MechanismInfo.toolSignInputFileProcessor.
// This function takes an input file and compute the hash hash and then return the file name.
// Usual inputs for hash is "sha1" or "sha256".
func HashFileProcessor(ctx context.Context, r hwsec.CmdRunner, input, hash string) string {
	cmd := fmt.Sprintf("openssl dgst -binary -%s '%s' > '%s.%s'", hash, input, input, hash)
	_, err := r.Run(ctx, "sh", "-c", cmd)
	if err != nil {
		testing.ContextLog(ctx, fmt.Sprintf("failed to %s the input file %s", hash, input))
		return ""
	}
	return fmt.Sprintf("%s.%s", input, hash)
}

// NewMechanism returns the MechanismInfo struct with the name equal to name. It'll panic if not found.
func NewMechanism(name string) *MechanismInfo {
	switch name {
	case "SHA1-RSA-PKCS":
		// Mechanism info for PKCS#1 v1.5 signature scheme with SHA1.
		return &MechanismInfo{
			name:                       name,
			toolMParam:                 "SHA1-RSA-PKCS",
			toolExtraParam:             []string{},
			toolSignInputFileProcessor: NoOpFileProcessor,
			opensslDgstParam:           "-sha1",
			opensslDgstExtraParam:      []string{},
		}
	case "SHA256-RSA-PKCS":
		// Mechanism info for PKCS#1 v1.5 signature scheme with SHA256.
		return &MechanismInfo{
			name:                       name,
			toolMParam:                 "SHA256-RSA-PKCS",
			toolExtraParam:             []string{},
			toolSignInputFileProcessor: NoOpFileProcessor,
			opensslDgstParam:           "-sha256",
			opensslDgstExtraParam:      []string{},
		}
	case "SHA1-RSA-PKCS-PSS":
		// Mechanism info for RSA PSS signature scheme with SHA1.
		// Note that this mechanism bundles RSA PSS and SHA1 together as a single mechanism.
		return &MechanismInfo{
			name:                       name,
			toolMParam:                 "SHA1-RSA-PKCS-PSS",
			toolExtraParam:             []string{"--mgf", "MGF1-SHA1"},
			toolSignInputFileProcessor: NoOpFileProcessor,
			opensslDgstParam:           "-sha1",
			opensslDgstExtraParam:      []string{"-sigopt", "rsa_padding_mode:pss", "-sigopt", "digest:sha1"},
		}
	case "SHA256-RSA-PKCS-PSS":
		// Mechanism info for RSA PSS signature scheme with SHA256.
		// Note that this mechanism bundles RSA PSS and SHA256 together as a single mechanism.
		return &MechanismInfo{
			name:                       name,
			toolMParam:                 "SHA256-RSA-PKCS-PSS",
			toolExtraParam:             []string{"--mgf", "MGF1-SHA256"},
			toolSignInputFileProcessor: NoOpFileProcessor,
			opensslDgstParam:           "-sha256",
			opensslDgstExtraParam:      []string{"-sigopt", "rsa_padding_mode:pss", "-sigopt", "digest:sha256"},
		}
	case "RSA-PKCS-PSS+SHA1":
		// Mechanism info for generic RSA PSS signature scheme with SHA1.
		// Note that this mechanism is using standalone, generic version of the RSA PSS mechanism, and SHA1 is specified as the hash algorithm in PSS parameters (instead of being part of mechanism).
		return &MechanismInfo{
			name:           name,
			toolMParam:     "RSA-PKCS-PSS",
			toolExtraParam: []string{"--hash-algorithm", "SHA-1", "--mgf", "MGF1-SHA1"},
			toolSignInputFileProcessor: func(ctx context.Context, r hwsec.CmdRunner, input string) string {
				return HashFileProcessor(ctx, r, input, "sha1")
			},
			opensslDgstParam:      "-sha1",
			opensslDgstExtraParam: []string{"-sigopt", "rsa_padding_mode:pss", "-sigopt", "digest:sha1"},
		}
	case "RSA-PKCS-PSS+SHA256":
		// Mechanism info for generic RSA PSS signature scheme with SHA1.
		// Note that this mechanism is using standalone, generic version of the RSA PSS mechanism, and SHA256 is specified as the hash algorithm in PSS parameters (instead of being part of mechanism).
		return &MechanismInfo{
			name:           name,
			toolMParam:     "RSA-PKCS-PSS",
			toolExtraParam: []string{"--hash-algorithm", "SHA256", "--mgf", "MGF1-SHA256"},
			toolSignInputFileProcessor: func(ctx context.Context, r hwsec.CmdRunner, input string) string {
				return HashFileProcessor(ctx, r, input, "sha256")
			},
			opensslDgstParam:      "-sha256",
			opensslDgstExtraParam: []string{"-sigopt", "rsa_padding_mode:pss", "-sigopt", "digest:sha256"},
		}
	default:
		panic(fmt.Sprintf("Mechanism %q not found", name))
	}
}

// Sign sign the input and write the signature to output, using the mechanism, and signed with key.
// It'll return nil iff the signing is successful.
func (key *KeyInfo) Sign(ctx context.Context, p *Util, input string, output string, mechanism *MechanismInfo) error {
	args := append([]string{"--slot=" + strconv.Itoa(key.slot), "--id=" + key.objID, "--sign", "-m", mechanism.toolMParam}, mechanism.toolExtraParam...)
	args = append(args, "-i", mechanism.toolSignInputFileProcessor(ctx, p.runner, input), "-o", output)
	if _, err := p.RunPkcs11Tool(ctx, args...); err != nil {
		return errors.Wrapf(err, "failed to sign with %s", mechanism.name)
	}

	return nil
}

// VerifyWithOpenSSL verify the signature at signaturePath, signed with key and mechanism, and the message at input.
// It'll return nil iff the signature is valid.
func (key *KeyInfo) VerifyWithOpenSSL(ctx context.Context, p *Util, input string, signaturePath string, mechanism *MechanismInfo) error {
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

// SignAndVerify is just a convenient runner to test both signing and verification.
// altInput is path to another test file that differs in content to input. It is used to check that verify() indeed reject corrupted input.
func (key *KeyInfo) SignAndVerify(ctx context.Context, p *Util, input string, altInput string, mechanism *MechanismInfo) error {
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

// GetObjectAttribute retrieve the object of objType type and the id specified in key, and get its attribute attributeName. The returned tuple is (result, cmdMessage, error), error is nil iff the operation is successful, and in that case result holds the hex encoded attribute value. cmdMessage always holds the stdout of the p11_replay command, if such is available. err could be an error that contains only a single CKR_* code if that is the case.
func (key *KeyInfo) GetObjectAttribute(ctx context.Context, p *Util, objType string, attributeName string) (string, string, error) {
	// Execute the command and convert its output.
	binaryMsg, err := p.runner.Run(ctx, "p11_replay", "--get_attribute", "--slot="+strconv.Itoa(key.slot), "--id="+key.objID, "--attribute="+attributeName, "--type="+objType)
	var msg string
	if binaryMsg != nil {
		msg = string(binaryMsg)
	}
	// Note, since we need to distinguish the reason for failure, we'll return the error found in the error message first (CKR_*). If no error code in message is found, then we'll return the err from calling Run().

	const p11replayGetObjectAttributeDataPrefix = "Attribute Data in hex: "
	const p11replayGetObjectAttributeErrorPrefix = "Unable to access the attribute, error: "

	var resultFound = false
	var result string
	var errorMsgFound = false
	var errorMsg string

	// Try to parse the result.
	for _, s := range strings.Split(msg, "\n") {
		// Try to check for data.
		if strings.HasPrefix(s, p11replayGetObjectAttributeDataPrefix) {
			if resultFound {
				// Extra data.
				return "", msg, errors.Errorf("extra data in parsing get object attribute output %q", result)
			}
			result = s[len(p11replayGetObjectAttributeDataPrefix):]
			resultFound = true
		}

		// Try to check for error message.
		if strings.HasPrefix(s, p11replayGetObjectAttributeErrorPrefix) {
			if errorMsgFound {
				// Extra data.
				return "", msg, errors.Errorf("extra error message in parsing get object attribute output %q", errorMsg)
			}
			errorMsg = s[len(p11replayGetObjectAttributeErrorPrefix):]
			errorMsgFound = true
		}
	}
	// If error message is found, then we'll return that.
	if errorMsgFound {
		if resultFound {
			// Shouldn't happen.
			return "", msg, errors.New("both error message and data is found in get object attribute output")
		}
		// Log the original error from Run() because we are not returning it.
		testing.ContextLog(ctx, "p11_replay failed with error: ", err)
		// Usually errorMsg is one of the CKR_* codes.
		return "", msg, errors.New(errorMsg)
	}
	// If no error message is found, but Run() failed, return that error.
	if err != nil {
		return "", msg, errors.Wrap(err, "failed to get attribute with p11_replay: ")
	}

	return result, msg, nil
}

// SetObjectAttribute retrieve the object of objType type and the id specified in key, and set its attribute attributeName with the value attributeValue. The returned tuple is (cmdMessage, error), whereby error is nil iff the operation is successful. cmdMessage holds the stdout from p11_replay command if such is available.
func (key *KeyInfo) SetObjectAttribute(ctx context.Context, p *Util, objType string, attributeName string, attributeValue string) (string, error) {
	// Execute the command and convert its output.
	binaryMsg, err := p.runner.Run(ctx, "p11_replay", "--set_attribute", "--slot="+strconv.Itoa(key.slot), "--id="+key.objID, "--attribute="+attributeName, "--data="+attributeValue, "--type="+objType)
	var msg string
	if binaryMsg != nil {
		msg = string(binaryMsg)
	}
	// Note, since we need to distinguish the reason for failure, we'll return the error found in the error message first (CKR_*). If no error code in message is found, then we'll return the err from calling Run().

	const p11replaySetObjectAttributeSuccessMsg = "Set attribute OK."
	const p11replaySetObjectAttributeErrorPrefix = "Failed to set attribute, error: "

	var errorMsgFound = false
	var errorMsg string

	// Try to parse the result for error code.
	for _, s := range strings.Split(msg, "\n") {
		if strings.HasPrefix(s, p11replaySetObjectAttributeErrorPrefix) {
			if errorMsgFound {
				// Extra data.
				return msg, errors.Errorf("extra error message in parsing set object attribute output %q", errorMsg)
			}
			errorMsg = s[len(p11replaySetObjectAttributeErrorPrefix):]
			errorMsgFound = true
		}
	}

	var successful = strings.Contains(msg, p11replaySetObjectAttributeSuccessMsg)

	if errorMsgFound {
		if successful {
			// Shouldn't happen.
			return msg, errors.New("both error message and success message is found in set object attribute output")
		}
		// Log the original error from Run() because we are not returning it.
		testing.ContextLog(ctx, "p11_replay failed with error: ", err)
		// Usually errorMsg is one of the CKR_* codes.
		return msg, errors.New(errorMsg)
	}
	// If no error message is found, but Run() failed, return that error.
	if err != nil {
		return msg, errors.Wrap(err, "failed to get attribute with p11_replay")
	}

	// If there's no error, but the output message is still incorrect.
	if !strings.Contains(msg, p11replaySetObjectAttributeSuccessMsg) {
		return msg, errors.New("failed to set attribute with p11_replay, incorrect response")
	}
	return msg, nil
}

// IsSoftwareBacked checks if the given key is backed by hardware or software.
// The return error is nil iff the operation is successful, in that case, the boolean is true iff it is backed in software.
func (key *KeyInfo) IsSoftwareBacked(ctx context.Context, p *Util) (bool, error) {
	isSoftwareBackedStr, msg, err := key.GetObjectAttribute(ctx, p, "privkey", "kKeyInSoftware")
	if err != nil {
		testing.ContextLog(ctx, "GetObjectAttribute failed with: ", msg)
		return false, errors.Wrap(err, "failed to get object attribute kKeyInSoftware")
	}

	if isSoftwareBackedStr == "00" {
		return false, nil
	} else if isSoftwareBackedStr == "01" {
		return true, nil
	}

	return false, errors.Errorf("unknown attribute value %s for kKeyInSoftware", isSoftwareBackedStr)
}
