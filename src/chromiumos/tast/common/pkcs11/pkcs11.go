// Copyright 2020 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package pkcs11

import (
	"context"
	"fmt"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/google/uuid"

	"chromiumos/tast/common/hwsec"
	"chromiumos/tast/errors"
	"chromiumos/tast/testing"
)

// Chaps is the class that stores the persistent state that is required to run all PKCS#11 related utility calls.
type Chaps struct {
	// runner is a command runner interface for executing commands on the DUT.
	runner hwsec.CmdRunner

	// utility is an interface to cryptohome for calling cryptohome related operations.
	utility *hwsec.CryptohomeClient

	// chapsPath is the path to the chaps PKCS#11 module.
	chapsPath string
}

// NewChaps creates a new Chaps.
func NewChaps(ctx context.Context, r hwsec.CmdRunner, u *hwsec.CryptohomeClient) (*Chaps, error) {
	chapsPath, err := locateChapsModule(ctx, r)
	if err != nil {
		return nil, errors.Wrap(err, "failed to locate the chaps module")
	}
	return &Chaps{runner: r, utility: u, chapsPath: chapsPath}, nil
}

// locateChapsModule search for the chaps pkcs#11 module and set it in the Chaps struct. nil is returned iff the operation is successful.
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
func (p *Chaps) RunPkcs11Tool(ctx context.Context, args ...string) ([]byte, error) {
	args = append([]string{"--module=" + p.chapsPath}, args...)
	return p.runner.Run(ctx, "pkcs11-tool", args...)
}

// ClearObjects remove all objects with the given ID objID in the token in slot slot and of type objType.
// objType is usually "privkey" or "cert".
func (p *Chaps) ClearObjects(ctx context.Context, slot int, objID, objType string) error {
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
func (p *Chaps) ClearObjectsOfAllType(ctx context.Context, slot int, objID string) error {
	for _, t := range []string{"privkey", "pubkey", "cert", "data", "secrkey"} {
		if err := p.ClearObjects(ctx, slot, objID, t); err != nil {
			return errors.Wrapf(err, "failed to clear objects of type %q", t)
		}
	}
	return nil
}

// KeyInfo stores the information for a particular key, both on disk and in chaps keystore.
type KeyInfo struct {
	// File path to the public key stored in DER format. This may be empty.
	pubKeyPath string

	// File path to the private key stored in DER format. This may be empty to indicate that the private key cannot be extracted.
	privKeyPath string

	// File path to the certificate in DER format. This may be empty.
	certPath string

	// The prefix of path for all files related to this key. This is used in cleanup. Note that this variable should not have any special characters, as this is used unescaped. This may be empty to indicate that this key doesn't have any residual file on disk.
	keyPrefix string

	// The PKCS#11 token slot that holds this key.
	slot int

	// The PKCS#11 token slot is owned by this user.
	username string

	// The PKCS#11 Object ID of the key.
	objID string

	// NOTE: If any reference type is added in the future, modify CreateKeyCopy to deep copy.
}

// DumpKeyInfo converts the information in the key into a human readable string for debugging purpose.
func (p *Chaps) DumpKeyInfo(k *KeyInfo) string {
	return fmt.Sprintf("Slot %d, ID %q, PubKey %q", k.slot, k.objID, k.pubKeyPath)
}

// UpdateKeySlot will update the given KeyInfo's slot to the current state known by cryptohomed.
// This should be used if there's any reboot/mount/unmount since the key is last updated or created.
func (p *Chaps) UpdateKeySlot(ctx context.Context, k *KeyInfo) error {
	// Get the corresponding slot.
	slot, err := p.utility.GetTokenForUser(ctx, k.username)
	if err != nil {
		return errors.Wrap(err, "failed to get slot")
	}

	k.slot = slot
	return nil
}

// CreateECSoftwareKey create a key and insert it into the system token (if username is empty), or user token specified by username. The object will have an ID of objID, and the corresponding public key will be deposited in the scratchpad.
func (p *Chaps) CreateECSoftwareKey(ctx context.Context, scratchpadPath, username, keyname, objID string, forceSoftwareBacked, checkSoftwareBacked bool) (*KeyInfo, error) {
	// Get the corresponding slot.
	slot, err := p.utility.GetTokenForUser(ctx, username)
	if err != nil {
		return nil, errors.Wrap(err, "failed to get slot")
	}

	// Check that scratchpad is available.
	keyPrefix := filepath.Join(scratchpadPath, keyname)
	result := &KeyInfo{
		keyPrefix:   keyPrefix,
		privKeyPath: keyPrefix + "-priv.der",
		pubKeyPath:  keyPrefix + "-pub.der",
		certPath:    keyPrefix + "-cert.der",
		objID:       objID,
		slot:        slot,
		username:    username,
	}

	// Note: This method calls openssl commands in order to ease debugging.
	// When debugging tests that uses this method, key creation can be replayed
	// on command line by copying and pasting the commands from test log.

	// Create the private key.
	privKeyPemPath := fmt.Sprintf("/tmp/%s-priv.key", keyname)
	if _, err := p.runner.Run(ctx, "openssl", "ecparam", "-name", "prime256v1", "-genkey", "-noout", "-out", privKeyPemPath); err != nil {
		return nil, errors.Wrap(err, "failed to genereate ecc key with openssl")
	}

	// Extract the public key from the private key.
	pubKeyPemPath := fmt.Sprintf("/tmp/%s-pub.key", keyname)
	if _, err := p.runner.Run(ctx, "openssl", "ec", "-in", privKeyPemPath, "-pubout", "-out", pubKeyPemPath); err != nil {
		return nil, errors.Wrap(err, "failed to extract public key from private key with OpenSSL")
	}

	// Convert the private key to DER format.
	if _, err := p.runner.Run(ctx, "openssl", "ec", "-inform", "pem", "-outform", "der", "-in", privKeyPemPath, "-out", result.privKeyPath); err != nil {
		return nil, errors.Wrap(err, "failed to convert private key to DER format with OpenSSL")
	}

	// Convert the public key to DER format.
	if _, err := p.runner.Run(ctx, "openssl", "ec", "-pubin", "-inform", "pem", "-outform", "der", "-in", pubKeyPemPath, "-out", result.pubKeyPath); err != nil {
		return nil, errors.Wrap(err, "failed to convert private key to DER format with OpenSSL")
	}

	// Create the certificate
	certPemPath := fmt.Sprintf("/tmp/%s-cert.crt", keyname)
	if _, err := p.runner.Run(ctx, "openssl", "req", "-nodes", "-x509", "-sha1", "-key", privKeyPemPath, "-out", certPemPath, "-days", "365", "-subj", "/C=US/ST=CA/L=MTV/O=ChromiumOS/CN=chromiumos.example.com"); err != nil {
		return nil, errors.Wrap(err, "failed to create certificate with openssl")
	}

	// Convert the cert to DER format.
	if _, err := p.runner.Run(ctx, "openssl", "x509", "-in", certPemPath, "-outform", "der", "-out", result.certPath); err != nil {
		return nil, errors.Wrap(err, "failed to convert cert to DER format with openssl")
	}

	// Import the private key into chaps.
	args := []string{"--import", "--slot=" + strconv.Itoa(slot), "--path=" + result.privKeyPath, "--type=privkey", "--id=" + result.objID}
	if forceSoftwareBacked {
		args = append(args, "--force_software")
	}
	if _, err := p.runner.Run(ctx, "p11_replay", args...); err != nil {
		return nil, errors.Wrap(err, "failed to import private key into chaps")
	}

	// Import the certificate into chaps.
	if _, err := p.runner.Run(ctx, "p11_replay", "--import", "--slot="+strconv.Itoa(slot), "--path="+result.certPath, "--type=cert", "--id="+result.objID); err != nil {
		return nil, errors.Wrap(err, "failed to import certificate into chaps")
	}

	// If required, check that it's software backed or not.
	if checkSoftwareBacked {
		isSoftwareBacked, err := p.IsSoftwareBacked(ctx, result)
		if err != nil {
			return nil, errors.Wrap(err, "failed to get kKeyInSoftware attribute")
		}

		if isSoftwareBacked != forceSoftwareBacked {
			return nil, errors.Errorf("mismatch in kKeyInSoftware attribute (%t) and force software backed parameter (%t)", isSoftwareBacked, forceSoftwareBacked)
		}
	}
	return result, nil
}

// CreateRSASoftwareKey create a key and insert it into the system token (if username is empty), or user token specified by username. The object will have an ID of objID, and the corresponding public key will be deposited in /tmp/$keyname.key.
func (p *Chaps) CreateRSASoftwareKey(ctx context.Context, scratchpadPath, username, keyname, objID string, forceSoftwareBacked, checkSoftwareBacked bool) (*KeyInfo, error) {
	// Get the corresponding slot.
	slot, err := p.utility.GetTokenForUser(ctx, username)
	if err != nil {
		return nil, errors.Wrap(err, "failed to get slot")
	}

	// Check that scratchpad is available.
	keyPrefix := filepath.Join(scratchpadPath, keyname)
	result := &KeyInfo{
		keyPrefix:   keyPrefix,
		privKeyPath: keyPrefix + "-priv.der",
		pubKeyPath:  keyPrefix + "-pub.der",
		certPath:    keyPrefix + "-cert.der",
		objID:       objID,
		slot:        slot,
		username:    username,
	}

	// Note: This method calls openssl commands in order to ease debugging.
	// When debugging tests that uses this method, key creation can be replayed
	// on command line by copying and pasting the commands from test log.

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
	args := []string{"--import", "--slot=" + strconv.Itoa(slot), "--path=" + result.privKeyPath, "--type=privkey", "--id=" + result.objID}
	if forceSoftwareBacked {
		args = append(args, "--force_software")
	}
	if _, err := p.runner.Run(ctx, "p11_replay", args...); err != nil {
		return nil, errors.Wrap(err, "failed to import private key into chaps")
	}

	// Import the certificate into chaps.
	if _, err := p.runner.Run(ctx, "p11_replay", "--import", "--slot="+strconv.Itoa(slot), "--path="+result.certPath, "--type=cert", "--id="+result.objID); err != nil {
		return nil, errors.Wrap(err, "failed to import certificate into chaps")
	}

	// If required, check that it's software backed or not.
	if checkSoftwareBacked {
		isSoftwareBacked, err := p.IsSoftwareBacked(ctx, result)
		if err != nil {
			return nil, errors.Wrap(err, "failed to get kKeyInSoftware attribute")
		}

		if isSoftwareBacked != forceSoftwareBacked {
			return nil, errors.Errorf("mismatch in kKeyInSoftware attribute (%t) and force software backed parameter (%t)", isSoftwareBacked, forceSoftwareBacked)
		}
	}
	return result, nil
}

const (
	// GenRSA2048 is used to specify that we want to generate RSA 2048 key in CreateGeneratedKey.
	GenRSA2048 = "rsa:2048"
	// GenECP256 is used to specify that we want to generate elliptic curve key with P256 curve in CreateGeneratedKey.
	GenECP256 = "EC:prime256v1"
)

// CreateGeneratedKey creates a key by generating it in TPM and insert it into the system token (if username is empty), or user token specified by username. The object will have an ID of objID, and the corresponding public key will be deposited in /tmp/$keyname.key.
// Use GenRSA2048 or GenECP256 above for keyType.
func (p *Chaps) CreateGeneratedKey(ctx context.Context, scratchpadPath, keyType, username, keyname, objID string) (*KeyInfo, error) {
	// Get the corresponding slot.
	slot, err := p.utility.GetTokenForUser(ctx, username)
	if err != nil {
		return nil, errors.Wrap(err, "failed to get slot")
	}

	// Check that scratchpad is available.
	keyPrefix := filepath.Join(scratchpadPath, keyname)
	result := &KeyInfo{
		keyPrefix:   keyPrefix,
		privKeyPath: "", // No private key.
		pubKeyPath:  keyPrefix + "-pub.der",
		certPath:    "", // No certs.
		objID:       objID,
		slot:        slot,
		username:    username,
	}

	// Generate the key.
	if _, err := p.RunPkcs11Tool(ctx, "--slot="+strconv.Itoa(slot), "--keypairgen", "--key-type", keyType, "--id="+result.objID); err != nil {
		return nil, errors.Wrap(err, "failed to generate key with pkcs11-tool")
	}

	// Export the public key.
	if _, err := p.RunPkcs11Tool(ctx, "--slot="+strconv.Itoa(slot), "--id="+result.objID, "--read-object", "--type", "pubkey", "-o", result.pubKeyPath); err != nil {
		return nil, errors.Wrap(err, "failed to generate key with pkcs11-tool")
	}

	return result, nil
}

// ImportPrivateKeyOptions specifies the various options to ImportPrivateKeyBySlot.
type ImportPrivateKeyOptions struct {
	// PrivKeyPath is the path to the private key file in DER format.
	PrivKeyPath string
	// Slot is the slot to import the key into.
	Slot int
	// Username
	Username string
	// ObjID is the ID of the imported object.
	ObjID string
	// ForceSoftwareBacked, if true, will ensure that it is imported as software backed key.
	ForceSoftwareBacked bool
}

// ImportPrivateKeyBySlot creates a key by importing it from existing DER format private key file specified by opt.PrivKeyPath. The key will be inserted into the token specified by opt.Slot. The object will have an ID of opt.ObjID.
func (p *Chaps) ImportPrivateKeyBySlot(ctx context.Context, opt ImportPrivateKeyOptions) (*KeyInfo, error) {
	// Import the private key into chaps.
	args := []string{"--import", "--slot=" + strconv.Itoa(opt.Slot), "--path=" + opt.PrivKeyPath, "--type=privkey", "--id=" + opt.ObjID}
	if opt.ForceSoftwareBacked {
		args = append(args, "--force_software")
	}
	if _, err := p.runner.Run(ctx, "p11_replay", args...); err != nil {
		return nil, errors.Wrap(err, "failed to import private key into chaps")
	}

	result := &KeyInfo{
		keyPrefix:   "",
		privKeyPath: opt.PrivKeyPath,
		pubKeyPath:  "",
		certPath:    "",
		objID:       opt.ObjID,
		slot:        opt.Slot,
		username:    opt.Username,
	}

	return result, nil
}

// ImportPEMKeyAndCertBySlot imports key and cert of PEM format to the token specified by slot.
// The object will have an ID of objID. It is OK for either privKey or cert to be empty if they are not needed.
func (p *Chaps) ImportPEMKeyAndCertBySlot(ctx context.Context, scratchpadPath, privKey, cert, objID string, slot int, username string) (*KeyInfo, error) {
	if privKey == "" && cert == "" {
		return nil, errors.New("nothing to import")
	}

	result := &KeyInfo{
		keyPrefix:   "", // No keyPrefix, no file is left on disk.
		privKeyPath: "", // Not needed by caller for now.
		pubKeyPath:  "",
		certPath:    "", // Not needed by caller for now.
		objID:       objID,
		slot:        slot,
		username:    username,
	}

	if privKey != "" {
		// Generate temp file path for the PEM to DER conversion.
		keyPemPath := filepath.Join(scratchpadPath, uuid.New().String()+".pem")
		defer p.runner.Run(ctx, "rm", "-f", keyPemPath)
		keyDerPath := filepath.Join(scratchpadPath, uuid.New().String()+".der")
		defer p.runner.Run(ctx, "rm", "-f", keyDerPath)

		// Write out the pem file
		if _, err := p.runner.Run(ctx, "sh", "-c", fmt.Sprintf("echo %q > %q", privKey, keyPemPath)); err != nil {
			return nil, errors.Wrap(err, "failed to write key pem file")
		}

		// Convert the PEM file to DER file.
		if msg, err := p.runner.Run(ctx, "openssl", "rsa", "-in", keyPemPath, "-out", keyDerPath, "-inform", "PEM", "-outform", "DER"); err != nil {
			testing.ContextLogf(ctx, "OpenSSL failed to convert key pem to der: %q", msg)
			return nil, errors.Wrap(err, "failed to convert pem to der")
		}

		// Import the object with p11_replay
		if msg, err := p.runner.Run(ctx, "p11_replay", "--slot="+strconv.Itoa(slot), "--import", "--type=privkey", "--path="+keyDerPath, "--id="+objID); err != nil {
			testing.ContextLogf(ctx, "p11_replay failed to import key: %q", msg)
			return nil, errors.Wrap(err, "failed to import object with p11_replay")
		}
	}

	if cert != "" {
		// Generate temp file path for the PEM to DER conversion.
		certPemPath := filepath.Join(scratchpadPath, uuid.New().String()+".pem")
		defer p.runner.Run(ctx, "rm", "-f", certPemPath)
		certDerPath := filepath.Join(scratchpadPath, uuid.New().String()+".der")
		defer p.runner.Run(ctx, "rm", "-f", certDerPath)

		// Write out the pem file
		if _, err := p.runner.Run(ctx, "sh", "-c", fmt.Sprintf("echo %q > %q", cert, certPemPath)); err != nil {
			return nil, errors.Wrap(err, "failed to write key pem file")
		}

		// Convert the PEM file to DER file.
		if _, err := p.runner.Run(ctx, "openssl", "x509", "-in", certPemPath, "-out", certDerPath, "-inform", "PEM", "-outform", "DER"); err != nil {
			return nil, errors.Wrap(err, "failed to convert pem to der")
		}

		// Import the object with p11_replay
		if _, err := p.runner.Run(ctx, "p11_replay", "--slot="+strconv.Itoa(slot), "--import", "--type=cert", "--path="+certDerPath, "--id="+objID); err != nil {
			return nil, errors.Wrap(err, "failed to import cert with p11_replay")
		}
	}

	return result, nil
}

// generateAttrListForCopyObject is a simple helper function that converts a map from string to string to a string that can be passed to p11_replay --copy_object as --attr_list parameter.
func generateAttrListForCopyObject(attributes *map[string]string) string {
	attributesList := make([]string, 0, len(*attributes))
	for k, v := range *attributes {
		attributesList = append(attributesList, k+":"+v)
	}
	attributesParam := strings.Join(attributesList, ",")
	return attributesParam
}

// CreateKeyCopy creates a copy of origKey and sets its CKA_ID to objID, and other attributes according to attributes map. It returns (key, message, err), whereby err is nil iff the operation is successful. key is the new key and message is the stdout of p11_replay command where available.
func (p *Chaps) CreateKeyCopy(ctx context.Context, origKey *KeyInfo, objID string, attributes map[string]string) (*KeyInfo, string, error) {
	// Set the object ID.
	attributes["CKA_ID"] = objID

	// Generate the attribute string.
	attributesParam := generateAttrListForCopyObject(&attributes)

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
func (p *Chaps) DestroyKey(ctx context.Context, key *KeyInfo) error {
	// Remove the objects in key store.
	if err := p.ClearObjectsOfAllType(ctx, key.slot, key.objID); err != nil {
		return errors.Wrap(err, "failed to remove objects")
	}

	// Remove the on disk files (if any).
	if key.keyPrefix != "" {
		if _, err := p.runner.Run(ctx, "sh", "-c", fmt.Sprintf(`rm -f "%s*"`, key.keyPrefix)); err != nil {
			return errors.Wrapf(err, "failed to remove on disk files starting with %s", key.keyPrefix)
		}
	}

	return nil
}

// MechanismInfo stores the information regarding a mechanism, and the various related parameters for using this mechanism with various tools such as openssl and pkcs11-tool.
// Note that there's a set of constants defined in this file for users of this struct.
type MechanismInfo struct {
	// Name used to identify this mechanism in debug message.
	Name string

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
// This function takes an input file and compute the hash hash and then return the hash file name.
// Usual inputs for hash is "sha1" or "sha256".
func HashFileProcessor(ctx context.Context, r hwsec.CmdRunner, input, hash string) string {
	cmd := fmt.Sprintf("openssl dgst -binary -%s '%s' > '%s.%s'", hash, input, input, hash)
	_, err := r.Run(ctx, "sh", "-c", cmd)
	if err != nil {
		testing.ContextLogf(ctx, "Failed to %s the input file %s", hash, input)
		return ""
	}
	return fmt.Sprintf("%s.%s", input, hash)
}

// Below are some MechanismInfo constants for use by the tests.

// SHA1RSAPKCS represents the MechanismInfo that is a signing scheme that uses SHA1 for hashing and RSA PKCS#1 v1.5 for signing.
var SHA1RSAPKCS = MechanismInfo{
	Name:                       "SHA1-RSA-PKCS",
	toolMParam:                 "SHA1-RSA-PKCS",
	toolExtraParam:             []string{},
	toolSignInputFileProcessor: NoOpFileProcessor,
	opensslDgstParam:           "-sha1",
	opensslDgstExtraParam:      []string{},
}

// SHA256RSAPKCS represents the MechanismInfo that is a signing scheme that uses SHA256 for hashing and RSA PKCS#1 v1.5 for signing.
var SHA256RSAPKCS = MechanismInfo{
	Name:                       "SHA256-RSA-PKCS",
	toolMParam:                 "SHA256-RSA-PKCS",
	toolExtraParam:             []string{},
	toolSignInputFileProcessor: NoOpFileProcessor,
	opensslDgstParam:           "-sha256",
	opensslDgstExtraParam:      []string{},
}

// SHA1RSAPKCSPSS represents the MechanismInfo that is a signing scheme that uses SHA1 for hashing and RSA PSS for signing.
var SHA1RSAPKCSPSS = MechanismInfo{
	Name:                       "SHA1-RSA-PKCS-PSS",
	toolMParam:                 "SHA1-RSA-PKCS-PSS",
	toolExtraParam:             []string{"--mgf", "MGF1-SHA1"},
	toolSignInputFileProcessor: NoOpFileProcessor,
	opensslDgstParam:           "-sha1",
	opensslDgstExtraParam:      []string{"-sigopt", "rsa_padding_mode:pss", "-sigopt", "digest:sha1"},
}

// SHA256RSAPKCSPSS represents the MechanismInfo that is a signing scheme that uses SHA256 for hashing and RSA PSS for signing.
var SHA256RSAPKCSPSS = MechanismInfo{
	Name:                       "SHA256-RSA-PKCS-PSS",
	toolMParam:                 "SHA256-RSA-PKCS-PSS",
	toolExtraParam:             []string{"--mgf", "MGF1-SHA256"},
	toolSignInputFileProcessor: NoOpFileProcessor,
	opensslDgstParam:           "-sha256",
	opensslDgstExtraParam:      []string{"-sigopt", "rsa_padding_mode:pss", "-sigopt", "digest:sha256"},
}

// GenericRSAPKCSPSSWithSHA1 represents the MechanismInfo that uses generic RSA PSS signature scheme with SHA1.
// Note that this mechanism is different from the other one in the sense that this one is using standalone, generic version of the RSA PSS mechanism, and SHA1 is specified as the hash algorithm in PSS parameters (instead of being part of mechanism).
var GenericRSAPKCSPSSWithSHA1 = MechanismInfo{
	Name:           "RSA-PKCS-PSS+SHA1",
	toolMParam:     "RSA-PKCS-PSS",
	toolExtraParam: []string{"--hash-algorithm", "SHA-1", "--mgf", "MGF1-SHA1"},
	toolSignInputFileProcessor: func(ctx context.Context, r hwsec.CmdRunner, input string) string {
		return HashFileProcessor(ctx, r, input, "sha1")
	},
	opensslDgstParam:      "-sha1",
	opensslDgstExtraParam: []string{"-sigopt", "rsa_padding_mode:pss", "-sigopt", "digest:sha1"},
}

// GenericRSAPKCSPSSWithSHA256 represents the MechanismInfo that uses generic RSA PSS signature scheme with SHA256.
// Note that this mechanism is different from the other one in the sense that this one is using standalone, generic version of the RSA PSS mechanism, and SHA256 is specified as the hash algorithm in PSS parameters (instead of being part of mechanism).
var GenericRSAPKCSPSSWithSHA256 = MechanismInfo{
	Name:           "RSA-PKCS-PSS+SHA256",
	toolMParam:     "RSA-PKCS-PSS",
	toolExtraParam: []string{"--hash-algorithm", "SHA256", "--mgf", "MGF1-SHA256"},
	toolSignInputFileProcessor: func(ctx context.Context, r hwsec.CmdRunner, input string) string {
		return HashFileProcessor(ctx, r, input, "sha256")
	},
	opensslDgstParam:      "-sha256",
	opensslDgstExtraParam: []string{"-sigopt", "rsa_padding_mode:pss", "-sigopt", "digest:sha256"},
}

// ECDSASHA1 represents the MechanismInfo that uses ECDSA signature scheme with SHA1.
// Note that NIST P256 curve is used.
var ECDSASHA1 = MechanismInfo{
	Name:           "ECDSA-SHA1-P256",
	toolMParam:     "ECDSA-SHA1",
	toolExtraParam: []string{"--signature-format", "openssl"},
	// Note that openssl format is required, otherwise it'll output raw rs format.
	toolSignInputFileProcessor: NoOpFileProcessor,
	opensslDgstParam:           "-sha1",
}

// Sign sign the input and write the signature to output, using the mechanism, and signed with key.
// It'll return nil iff the signing is successful.
func (p *Chaps) Sign(ctx context.Context, key *KeyInfo, input, output string, mechanism *MechanismInfo) error {
	args := append([]string{"--slot=" + strconv.Itoa(key.slot), "--id=" + key.objID, "--sign", "-m", mechanism.toolMParam}, mechanism.toolExtraParam...)
	args = append(args, "-i", mechanism.toolSignInputFileProcessor(ctx, p.runner, input), "-o", output)
	if _, err := p.RunPkcs11Tool(ctx, args...); err != nil {
		return errors.Wrapf(err, "failed to sign with %s", mechanism.Name)
	}

	return nil
}

// Verify verify the signature at signaturePath, signed with key and mechanism, and the message at input.
// It'll return nil iff the signature is valid.
func (p *Chaps) Verify(ctx context.Context, key *KeyInfo, input, signaturePath string, mechanism *MechanismInfo) error {
	// Verify with OpenSSL.
	args := append([]string{"dgst", mechanism.opensslDgstParam, "-verify", key.pubKeyPath, "-keyform", "der"}, mechanism.opensslDgstExtraParam...)
	args = append(args, "-signature", signaturePath, input)
	binaryMsg, err := p.runner.Run(ctx, "openssl", args...)
	if err != nil {
		return errors.Wrapf(err, "failed to verify the signature of %s", mechanism.Name)
	}
	msg := string(binaryMsg)
	if msg != "Verified OK\n" {
		return errors.Errorf("failed to verify the signature of %s, because message mismatch, unexpected %q", mechanism.Name, msg)
	}

	// Note that it is possible to verify with pkcs11-tools as well, but it'll need version 0.20 or above, which is currently unavailable.
	return nil
}

// Error is a custom error type for storing error that occurs in PKCS#11 APIs with specific CKR_* error code.
type Error struct {
	*errors.E

	// PKCS11RetCode contains the return code from PKCS#11 method calls, and it should be of the form CKR_*
	PKCS11RetCode string

	// CmdMessage holds the stdout and stderr of the command execution, that is, the command that actually invoked the PKCS#11 calls.
	CmdMessage string
}

// GetObjectAttribute retrieves the object of objType type and the id specified in key, and returns its attribute specified by name. The returned values are (value, err), err is nil iff the operation is successful, and in that case value holds the hex encoded attribute value. err could be an error that contains only a single CKR_* code if that is the case.
func (p *Chaps) GetObjectAttribute(ctx context.Context, key *KeyInfo, objType, name string) (value string, err error) {
	// Execute the command and convert its output.
	binaryMsg, err := p.runner.Run(ctx, "p11_replay", "--get_attribute", "--slot="+strconv.Itoa(key.slot), "--id="+key.objID, "--attribute="+name, "--type="+objType)
	var msg string
	if binaryMsg != nil {
		msg = string(binaryMsg)
	}
	// Note, since we need to distinguish the reason for failure, we'll return the error found in the error message first (CKR_*). If no error code in message is found, then we'll return the err from calling Run().

	const (
		dataPrefix  = "Attribute Data in hex: "
		errorPrefix = "Unable to access the attribute, error: "
	)

	var results []string
	var errorMsgs []string

	// Try to parse the result.
	for _, s := range strings.Split(msg, "\n") {
		// Try to check for data.
		if strings.HasPrefix(s, dataPrefix) {
			results = append(results, strings.TrimPrefix(s, dataPrefix))
		}

		// Try to check for error message.
		if strings.HasPrefix(s, errorPrefix) {
			errorMsgs = append(errorMsgs, strings.TrimPrefix(s, errorPrefix))
		}
	}

	if len(results) > 1 {
		// Extra data.
		return "", &Error{E: errors.Errorf("extra data in parsing get object attribute output %q", results), CmdMessage: msg}
	}

	if len(errorMsgs) > 1 {
		// Extra data.
		return "", &Error{E: errors.Errorf("extra error message in parsing get object attribute output %q", errorMsgs), CmdMessage: msg}
	}

	// If error message is found, then we'll return that.
	if len(errorMsgs) != 0 {
		if len(results) != 0 {
			// Shouldn't happen.
			return "", &Error{E: errors.Wrapf(err, "both error message and data is found in get object attribute output; error message %q; data %q", errorMsgs[0], results[0]), CmdMessage: msg}
		}
		// Usually errorMsg is one of the CKR_* codes.
		return "", &Error{E: errors.Wrapf(err, "called to C_GetAttributeValue failed with pkcs11 return code %q", errorMsgs[0]), PKCS11RetCode: errorMsgs[0], CmdMessage: msg}
	}
	// If no error message is found, but Run() failed, return that error.
	if err != nil {
		return "", &Error{E: errors.Wrap(err, "failed to get attribute with p11_replay"), CmdMessage: msg}
	}

	if len(results) == 0 {
		return "", &Error{E: errors.New("attributes value not found in output message"), CmdMessage: msg}
	}

	return results[0], nil
}

// SetObjectAttribute retrieves the object of objType type and the id specified in key, and sets its attribute specified by name with the value value. The returned value is err, whereby err is nil iff the operation is successful.
func (p *Chaps) SetObjectAttribute(ctx context.Context, key *KeyInfo, objType, name, value string) (err error) {
	// Execute the command and convert its output.
	binaryMsg, err := p.runner.Run(ctx, "p11_replay", "--set_attribute", "--slot="+strconv.Itoa(key.slot), "--id="+key.objID, "--attribute="+name, "--data="+value, "--type="+objType)
	var msg string
	if binaryMsg != nil {
		msg = string(binaryMsg)
	}
	// Note, since we need to distinguish the reason for failure, we'll return the error found in the error message first (CKR_*). If no error code in message is found, then we'll return the err from calling Run().

	const (
		successMsg  = "Set attribute OK."
		errorPrefix = "Failed to set attribute, error: "
	)

	var errorMsgs []string

	// Try to parse the result for error code.
	for _, s := range strings.Split(msg, "\n") {
		if strings.HasPrefix(s, errorPrefix) {
			errorMsgs = append(errorMsgs, strings.TrimPrefix(s, errorPrefix))
		}
	}

	if len(errorMsgs) > 1 {
		// Extra data.
		return &Error{E: errors.Errorf("extra error message in parsing set object attribute output %q", errorMsgs), CmdMessage: msg}
	}

	successful := strings.Contains(msg, successMsg)

	if len(errorMsgs) != 0 {
		if successful {
			// Shouldn't happen.
			return &Error{E: errors.Wrapf(err, "both error message and success message is found in set object attribute output; error message %q", errorMsgs[0]), CmdMessage: msg}
		}
		// Usually errorMsg is one of the CKR_* codes.
		return &Error{E: errors.Wrapf(err, "called to C_SetAttributeValue failed with pkcs11 return code %q", errorMsgs[0]), PKCS11RetCode: errorMsgs[0], CmdMessage: msg}
	}
	// If no error message is found, but Run() failed, return that error.
	if err != nil {
		return &Error{E: errors.Wrap(err, "failed to get attribute with p11_replay"), CmdMessage: msg}
	}

	// If there's no error, but the output message is still incorrect.
	if !successful {
		return &Error{E: errors.New("failed to set attribute with p11_replay, incorrect response"), CmdMessage: msg}
	}
	return nil
}

// IsSoftwareBacked checks if the given key is backed by hardware or software.
// The return error is nil iff the operation is successful, in that case, the boolean is true iff it is backed in software.
func (p *Chaps) IsSoftwareBacked(ctx context.Context, key *KeyInfo) (bool, error) {
	isSoftwareBackedStr, err := p.GetObjectAttribute(ctx, key, "privkey", "kKeyInSoftware")
	if err != nil {
		var perr *Error
		if errors.As(err, &perr) {
			testing.ContextLog(ctx, "GetObjectAttribute failed with: ", perr.CmdMessage)
		}
		return false, errors.Wrap(err, "failed to get object attribute kKeyInSoftware")
	}

	if isSoftwareBackedStr == "00" {
		return false, nil
	} else if isSoftwareBackedStr == "01" {
		return true, nil
	}

	return false, errors.Errorf("unknown attribute value %s for kKeyInSoftware", isSoftwareBackedStr)
}

// ReplayWifiBySlot replays a EAP-TLS Wifi negotiation by slot.
func (p *Chaps) ReplayWifiBySlot(ctx context.Context, slot int, args ...string) error {
	cmdArgs := append([]string{"--replay_wifi", "--slot=" + strconv.Itoa(slot)}, args...)
	binaryMsg, err := p.runner.Run(ctx, "p11_replay", cmdArgs...)
	msg := string(binaryMsg)
	if err != nil {
		return errors.Wrapf(err, "failed to replay Wifi negotiation with message %q", msg)
	}
	return nil
}

// SlotInfo stores the information for a particular slot in chaps.
type SlotInfo struct {
	// The PKCS#11 token slot index.
	slotIndex int

	// The PKCS#11 token label.
	tokenLabel string
}

// ListSlots lists the slots in chaps
func (p *Chaps) ListSlots(ctx context.Context) ([]SlotInfo, error) {
	const (
		slotPrefix       = "Slot"
		tokenLabelPrefix = "  token label"
	)
	data, err := p.RunPkcs11Tool(ctx, "--list-slots")
	if err != nil {
		return nil, errors.Wrap(err, "failed to list slots")
	}

	var result []SlotInfo
	for _, s := range strings.Split(string(data), "\n") {
		if strings.HasPrefix(s, slotPrefix) {
			result = append(result, SlotInfo{})
			if _, err := fmt.Sscanf(s, "Slot %d", &result[len(result)-1].slotIndex); err != nil {
				return nil, errors.Wrapf(err, "failed to parse slot name %q", s)
			}
		} else if strings.HasPrefix(s, tokenLabelPrefix) {
			if len(result) < 1 {
				return nil, errors.Wrap(err, "label appeared before slot index")
			}
			n := strings.Index(s, ":")
			if n == -1 {
				return nil, errors.Wrapf(err, "failed to parse slot name %q", s)
			}
			result[len(result)-1].tokenLabel = s[n+2:]
		}
	}

	return result, nil
}
