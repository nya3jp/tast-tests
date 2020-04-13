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

// Chaps is the class that stores the persistent state that is required to run all PKCS#11 related utility calls.
type Chaps struct {
	// runner is a command runner interface for executing commands on the DUT.
	runner hwsec.CmdRunner

	// utility is an interface to cryptohome for calling cryptohome related operations.
	utility *hwsec.UtilityCryptohomeBinary

	// chapsPath is the path to the chaps PKCS#11 module.
	chapsPath string
}

// NewChaps creates a new Chaps.
func NewChaps(ctx context.Context, r hwsec.CmdRunner, u *hwsec.UtilityCryptohomeBinary) (*Chaps, error) {
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

// CreateRSASoftwareKey create a key and insert it into the system token (if username is empty), or user token specified by username. The object will have an ID of objID, and the corresponding public key will be deposited in /tmp/$keyname.key.
func (p *Chaps) CreateRSASoftwareKey(ctx context.Context, scratchpadPath, username, keyname, objID string) (*KeyInfo, error) {
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
	if _, err = p.runner.Run(ctx, "p11_replay", "--import", "--path="+result.privKeyPath, "--type=privkey", "--id="+result.objID); err != nil {
		return nil, errors.Wrap(err, "failed to import private key into chaps")
	}

	// Import the certificate into chaps.
	if _, err := p.runner.Run(ctx, "p11_replay", "--import", "--path="+result.certPath, "--type=cert", "--id="+result.objID); err != nil {
		return nil, errors.Wrap(err, "failed to import certificate into chaps")
	}

	return result, nil
}

// DestroyKey destroys the given key by removing it from disk and keystore.
func (p *Chaps) DestroyKey(ctx context.Context, key *KeyInfo) error {
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
	Name string

	// toolMParam field specifies the PKCS#11 mechanism code (algorithm+scheme) used for this set of mechanism configurations. Technically, this string is fed to pkcs11-tool as -m parameter. ex. pkcs11-tool [...] -m SHA1-RSA-PKCS [...]
	toolMParam string

	// The digest used by openssl dgst when we try to verify a signature of this type.
	opensslDgstParam string
}

// Below are some MechanismInfo constants for use by the tests.

// SHA1RSAPKCS represents the MechanismInfo that is a signing scheme that uses SHA1 for hashing and RSA PKCS#1 v1.5 for signing.
var SHA1RSAPKCS = MechanismInfo{
	Name:             "SHA1-RSA-PKCS",
	toolMParam:       "SHA1-RSA-PKCS",
	opensslDgstParam: "-sha1",
}

// SHA256RSAPKCS represents the MechanismInfo that is a signing scheme that uses SHA256 for hashing and RSA PKCS#1 v1.5 for signing.
var SHA256RSAPKCS = MechanismInfo{
	Name:             "SHA256-RSA-PKCS",
	toolMParam:       "SHA256-RSA-PKCS",
	opensslDgstParam: "-sha256",
}

// Sign sign the input and write the signature to output, using the mechanism, and signed with key.
// It'll return nil iff the signing is successful.
func (p *Chaps) Sign(ctx context.Context, key *KeyInfo, input, output string, mechanism *MechanismInfo) error {
	if _, err := p.RunPkcs11Tool(ctx, "--slot="+strconv.Itoa(key.slot), "--id="+key.objID, "--sign", "-m", mechanism.toolMParam, "-i", input, "-o", output); err != nil {
		return errors.Wrapf(err, "failed to sign with %s", mechanism.Name)
	}

	return nil
}

// Verify verify the signature at signaturePath, signed with key and mechanism, and the message at input.
// It'll return nil iff the signature is valid.
func (p *Chaps) Verify(ctx context.Context, key *KeyInfo, input, signaturePath string, mechanism *MechanismInfo) error {
	// Verify with OpenSSL.
	binaryMsg, err := p.runner.Run(ctx, "openssl", "dgst", mechanism.opensslDgstParam, "-verify", key.pubKeyPath, "-keyform", "der", "-signature", signaturePath, input)
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
