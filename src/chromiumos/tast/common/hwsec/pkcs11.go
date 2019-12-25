// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package hwsec

import (
	"context"
	"fmt"
	"os"
	"strconv"
	"strings"

	"chromiumos/tast/errors"
	"chromiumos/tast/testing"
)

// Pkcs11Scratchpad is the scratchpad temporary space for storing keys, certs and other data used in testing. This is specified without trailing slash.
const Pkcs11Scratchpad string = "/tmp"

// pkcs11UtilityToCryptohome is an interface used internally in this file that represents the stuffs we need from cryptohome. Usually stuffs like utilityCryptohomeBinary implements this.
type pkcs11UtilityToCryptohome interface {
	// GetTokenForUser retrieve the token slot for the user token if |username| is non-empty, or system token if |username| is empty.
	GetTokenForUser(ctx context.Context, username string) (int, error)
}

// Pkcs11Util is the class that stores the persistent state that is required to run all PKCS#11 related utility calls.
type Pkcs11Util struct {
	runner  CmdRunner
	utility pkcs11UtilityToCryptohome
}

// NewPkcs11Util creates a new Pkcs11Util
func NewPkcs11Util(r CmdRunner, u pkcs11UtilityToCryptohome) (*Pkcs11Util, error) {
	return &Pkcs11Util{r, u}, nil
}

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
func (p *Pkcs11Util) Pkcs11ClearObject(ctx context.Context, slot int, objID string, objType string) error {
	chapsPath := GetChapsCryptokiModule()
	if chapsPath == "" {
		return errors.New("unable to find chaps module")
	}

	for i := 0; i < 20; i++ {
		_, err := p.runner.Run(ctx, "pkcs11-tool", "--module="+chapsPath, "--slot="+strconv.Itoa(slot), "--delete-object", "--type", objType, "--id", objID)
		if err != nil {
			// If we fail to delete that object, then it's already gone, so we are done.
			break
		}
	}
	return nil
}

// Pkcs11ClearObjectOfAllType remove all object with the given ID |objID| in the token in slot |slot|, regardless of type. Note that this method will try to remove as many object as possible, even if there's failure somewhere.
func (p *Pkcs11Util) Pkcs11ClearObjectOfAllType(ctx context.Context, slot int, objID string) error {
	// Note that if there are multiple failures in this function, only 1 is returned in the end.
	var result error

	for _, t := range []string{"privkey", "pubkey", "cert", "data", "secrkey"} {
		if err := p.Pkcs11ClearObject(ctx, slot, objID, t); err != nil {
			result = errors.Wrap(err, fmt.Sprintf("Failed to clear %s for slot %d objID %s: ", t, slot, objID))
			// Note, we'll continue in face of error because we want to clear as much out as possible.
		}
	}

	return result
}

// Pkcs11KeyInfo stores the information for a particular key, both on disk and in chaps keystore.
type Pkcs11KeyInfo struct {
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

	// NOTE: If any reference type is added in the future, modify Pkcs11CreateCopiedKey to deep copy.
}

// Pkcs11CreateRsaSoftwareKey create a key and insert it into the system token (if |username| is empty), or user token specified by |username|. The object will have an ID of |objID|, and the corresponding public key will be deposited in /tmp/$keyname.key.
func (p *Pkcs11Util) Pkcs11CreateRsaSoftwareKey(ctx context.Context, utility pkcs11UtilityToCryptohome, username string, keyname string, objID string, forceSoftwareBacked bool) (Pkcs11KeyInfo, error) {
	result := Pkcs11KeyInfo{}
	result.keyPrefix = Pkcs11Scratchpad + "/" + keyname
	result.privKeyPath = result.keyPrefix + "-priv.der"
	result.pubKeyPath = result.keyPrefix + "-pub.der"
	result.certPath = result.keyPrefix + "-cert.der"
	result.objID = objID
	slot, err := utility.GetTokenForUser(ctx, username)
	if err != nil {
		return Pkcs11KeyInfo{}, errors.Wrap(err, "failed to get slot")
	}
	result.slot = slot

	// Create the private key and cert.
	_, err = p.runner.Run(ctx, "openssl", "req", "-nodes", "-x509", "-sha1", "-newkey", "rsa:2048", "-keyout", "/tmp/"+keyname+"-priv.key", "-out", "/tmp/"+keyname+"-cert.crt", "-days", "365", "-subj", "/C=US/ST=CA/L=MTV/O=ChromiumOS/CN=chromiumos.example.com")
	if err != nil {
		return Pkcs11KeyInfo{}, errors.Wrap(err, "failed to create key with openssl")
	}

	// Extract the public key from the private key.
	_, err = p.runner.Run(ctx, "openssl", "rsa", "-in", "/tmp/"+keyname+"-priv.key", "-pubout", "-out", "/tmp/"+keyname+"-pub.key")
	if err != nil {
		return Pkcs11KeyInfo{}, errors.Wrap(err, "failed to extract public key from private key with OpenSSL")
	}

	// Convert the private key to DER format.
	_, err = p.runner.Run(ctx, "openssl", "pkcs8", "-inform", "pem", "-outform", "der", "-in", "/tmp/"+keyname+"-priv.key", "-out", result.privKeyPath, "-nocrypt")
	if err != nil {
		return Pkcs11KeyInfo{}, errors.Wrap(err, "failed to convert private key to DER format with OpenSSL")
	}

	// Convert the public key to DER format.
	_, err = p.runner.Run(ctx, "openssl", "rsa", "-pubin", "-inform", "pem", "-outform", "der", "-in", "/tmp/"+keyname+"-pub.key", "-out", result.pubKeyPath)
	if err != nil {
		return Pkcs11KeyInfo{}, errors.Wrap(err, "failed to convert private key to DER format with OpenSSL")
	}

	// Convert the cert to DER format.
	_, err = p.runner.Run(ctx, "openssl", "x509", "-in", "/tmp/"+keyname+"-cert.crt", "-outform", "der", "-out", result.certPath)
	if err != nil {
		return Pkcs11KeyInfo{}, errors.Wrap(err, "failed to convert cert to DER format with openssl")
	}

	// Import the private key into chaps
	args := []string{"--import", "--path=" + result.privKeyPath, "--type=privkey", "--id=" + result.objID}
	if forceSoftwareBacked {
		args = append(args, "--force_software")
	}
	_, err = p.runner.Run(ctx, "p11_replay", args...)
	if err != nil {
		return Pkcs11KeyInfo{}, errors.Wrap(err, "failed to import private key into chaps")
	}

	// Import the certificate into chaps
	_, err = p.runner.Run(ctx, "p11_replay", "--import", "--path="+result.certPath, "--type=cert", "--id="+result.objID)
	if err != nil {
		return Pkcs11KeyInfo{}, errors.Wrap(err, "failed to import certificate into chaps")
	}

	return result, nil
}

// Pkcs11CreateRsaGeneratedKey create a key by generating it in TPM and insert it into the system token (if |username| is empty), or user token specified by |username|. The object will have an ID of |objID|, and the corresponding public key will be deposited in /tmp/$keyname.key.
func (p *Pkcs11Util) Pkcs11CreateRsaGeneratedKey(ctx context.Context, utility pkcs11UtilityToCryptohome, username string, keyname string, objID string) (Pkcs11KeyInfo, error) {
	// Locate chaps first.
	chapsPath := GetChapsCryptokiModule()
	if chapsPath == "" {
		return Pkcs11KeyInfo{}, errors.New("unable to find chaps module")
	}

	result := Pkcs11KeyInfo{}
	result.keyPrefix = Pkcs11Scratchpad + "/" + keyname
	// No private key.
	result.privKeyPath = ""
	result.pubKeyPath = result.keyPrefix + "-pub.der"
	// No certs.
	result.certPath = ""
	result.objID = objID
	slot, err := utility.GetTokenForUser(ctx, username)
	if err != nil {
		return Pkcs11KeyInfo{}, errors.Wrap(err, "failed to get slot")
	}
	result.slot = slot

	// Generate the key.
	_, err = p.runner.Run(ctx, "pkcs11-tool", "--module="+chapsPath, "--slot="+strconv.Itoa(slot), "--keypairgen", "--key-type", "rsa:2048", "--id="+result.objID)
	if err != nil {
		return Pkcs11KeyInfo{}, errors.Wrap(err, "failed to generate key with pkcs11-tool")
	}

	// Export the public key.
	_, err = p.runner.Run(ctx, "pkcs11-tool", "--module="+chapsPath, "--slot="+strconv.Itoa(slot), "--id="+result.objID, "--read-object", "--type", "pubkey", "-o", result.pubKeyPath)
	if err != nil {
		return Pkcs11KeyInfo{}, errors.Wrap(err, "failed to generate key with pkcs11-tool")
	}

	return result, nil
}

// Pkcs11CreateCopiedKey creates a copy of |origKey| and set its CKA_ID to |objID|, and other attributes according to |attributes| map. It returns (key, message, err) tuple, whereby |err| is nil iff the operation is successful. |key| is the new key and |message| is the stdout of p11_replay command where available.
func (p *Pkcs11Util) Pkcs11CreateCopiedKey(ctx context.Context, origKey Pkcs11KeyInfo, objID string, attributes map[string]string) (Pkcs11KeyInfo, string, error) {
	// Set the object ID
	attributes["CKA_ID"] = objID

	// Generate the attribute string
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
		testing.ContextLog(ctx, "p11_replay failed with: "+msg)
		return Pkcs11KeyInfo{}, msg, errors.Wrap(err, "Failed to run p11_replay")
	}

	const P11replayCopyObjectSuccessMsg string = "Operation completed successfully."

	if !strings.Contains(msg, P11replayCopyObjectSuccessMsg) {
		testing.ContextLog(ctx, "p11_replay failed with: "+msg)
		return Pkcs11KeyInfo{}, msg, errors.New("Incorrect response from p11_replay")
	}

	newKey := origKey
	newKey.objID = objID

	return newKey, msg, nil
}

// Pkcs11DestroyKey destroys the given |key| by removing it from disk and keystore.
func (p *Pkcs11Util) Pkcs11DestroyKey(ctx context.Context, key Pkcs11KeyInfo) error {
	// Note that we only return the last error, since all errors in this function is non-fatal.
	var result error

	// Remove the on disk files first.
	if _, err := p.runner.Run(ctx, "sh", "-c", "rm -f \""+key.keyPrefix+"*\""); err != nil {
		testing.ContextLog(ctx, "Failed to remove on disk files starting with "+key.keyPrefix, err)
		result = errors.Wrap(err, "Failed to remove on disk files starting with "+key.keyPrefix)
	}

	// Remove the objects in key store
	if err := p.Pkcs11ClearObjectOfAllType(ctx, key.slot, key.objID); err != nil {
		testing.ContextLog(ctx, "Failed to remove object in keystore, id="+key.objID, err)
		result = errors.Wrap(err, "Failed to remove object in keystore, id="+key.objID)
	}

	return result
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
	toolSignInputFileProcessor func(ctx context.Context, runner CmdRunner, input string) string

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
func Pkcs11NoOpFileProcessor(ctx context.Context, runner CmdRunner, input string) string {
	return input
}

// Constants for Pkcs11MechanismInfo

// Pkcs11SHA1RSAPKCS returns a mechanism info for PKCS#1 v1.5 signature scheme with SHA1
func (p *Pkcs11Util) Pkcs11SHA1RSAPKCS() Pkcs11MechanismInfo {
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

// Pkcs11SHA256RSAPKCS returns a mechanism info for PKCS#1 v1.5 signature scheme with SHA256
func (p *Pkcs11Util) Pkcs11SHA256RSAPKCS() Pkcs11MechanismInfo {
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

// Pkcs11SHA1RSAPKCSPSS returns a mechanism info for RSA PSS signature scheme with SHA1. Note that this mechanism bundles RSA PSS and SHA1 together as a single mechanism.
func (p *Pkcs11Util) Pkcs11SHA1RSAPKCSPSS() Pkcs11MechanismInfo {
	return Pkcs11MechanismInfo{
		name:                       "SHA1-RSA-PKCS-PSS",
		toolMParam:                 "SHA1-RSA-PKCS-PSS",
		toolExtraParam:             []string{"--mgf", "MGF1-SHA1"},
		toolSignInputFileProcessor: Pkcs11NoOpFileProcessor,
		opensslDgstParam:           "-sha1",
		opensslDgstExtraParam:      []string{"-sigopt", "rsa_padding_mode:pss", "-sigopt", "digest:sha1"},
		canSignVerify:              true,
	}
}

// Pkcs11SHA256RSAPKCSPSS returns a mechanism info for RSA PSS signature scheme with SHA256. Note that this mechanism bundles RSA PSS and SHA256 together as a single mechanism.
func (p *Pkcs11Util) Pkcs11SHA256RSAPKCSPSS() Pkcs11MechanismInfo {
	return Pkcs11MechanismInfo{
		name:                       "SHA256-RSA-PKCS-PSS",
		toolMParam:                 "SHA256-RSA-PKCS-PSS",
		toolExtraParam:             []string{"--mgf", "MGF1-SHA256"},
		toolSignInputFileProcessor: Pkcs11NoOpFileProcessor,
		opensslDgstParam:           "-sha256",
		opensslDgstExtraParam:      []string{"-sigopt", "rsa_padding_mode:pss", "-sigopt", "digest:sha256"},
		canSignVerify:              true,
	}
}

// Pkcs11SHA1FileProcessor is for Pkcs11MechanismInfo.toolSignInputFileProcessor.
// This function takes an input file and sha1 it then return the file name.
func Pkcs11SHA1FileProcessor(ctx context.Context, r CmdRunner, input string) string {
	output := input + ".sha1"
	_, err := r.Run(ctx, "sh", "-c", fmt.Sprintf("openssl dgst -binary -sha1 '%s' > '%s'", input, output))
	if err != nil {
		testing.ContextLog(ctx, "failed to sha1 the input file "+input)
		return ""
	}
	return output
}

// Pkcs11SHA1RSAPKCSPSSGeneric returns a mechanism info for generic RSA PSS signature scheme with SHA1. Note that this mechanism is using standalone, generic version of the RSA PSS mechanism, and SHA1 is specified as the hash algorithm in PSS parameters (instead of being part of mechanism).
func (p *Pkcs11Util) Pkcs11SHA1RSAPKCSPSSGeneric() Pkcs11MechanismInfo {
	return Pkcs11MechanismInfo{
		name:                       "RSA-PKCS-PSS + SHA1",
		toolMParam:                 "RSA-PKCS-PSS",
		toolExtraParam:             []string{"--hash-algorithm", "SHA-1", "--mgf", "MGF1-SHA1"},
		toolSignInputFileProcessor: Pkcs11SHA1FileProcessor,
		opensslDgstParam:           "-sha1",
		opensslDgstExtraParam:      []string{"-sigopt", "rsa_padding_mode:pss", "-sigopt", "digest:sha1"},
		canSignVerify:              true,
	}
}

// Pkcs11SHA256FileProcessor is for Pkcs11MechanismInfo.toolSignInputFileProcessor.
// This function takes an input file and sha1 it then return the file name.
func Pkcs11SHA256FileProcessor(ctx context.Context, r CmdRunner, input string) string {
	output := input + ".sha256"
	_, err := r.Run(ctx, "sh", "-c", fmt.Sprintf("openssl dgst -binary -sha256 '%s' > '%s'", input, output))
	if err != nil {
		testing.ContextLog(ctx, "failed to sha256 the input file "+input)
		return ""
	}
	return output
}

// Pkcs11SHA256RSAPKCSPSSGeneric returns a mechanism info for generic RSA PSS signature scheme with SHA1. Note that this mechanism is using standalone, generic version of the RSA PSS mechanism, and SHA256 is specified as the hash algorithm in PSS parameters (instead of being part of mechanism).
func (p *Pkcs11Util) Pkcs11SHA256RSAPKCSPSSGeneric() Pkcs11MechanismInfo {
	return Pkcs11MechanismInfo{
		name:                       "RSA-PKCS-PSS + SHA256",
		toolMParam:                 "RSA-PKCS-PSS",
		toolExtraParam:             []string{"--hash-algorithm", "SHA256", "--mgf", "MGF1-SHA256"},
		toolSignInputFileProcessor: Pkcs11SHA256FileProcessor,
		opensslDgstParam:           "-sha256",
		opensslDgstExtraParam:      []string{"-sigopt", "rsa_padding_mode:pss", "-sigopt", "digest:sha256"},
		canSignVerify:              true,
	}
}

// Pkcs11Sign sign the |input| and write the signature to |output|, using the |mechanism|, and signed with |key|.
// It'll return nil iff the signing is successful.
func (p *Pkcs11Util) Pkcs11Sign(ctx context.Context, key Pkcs11KeyInfo, input string, output string, mechanism Pkcs11MechanismInfo) error {
	chapsPath := GetChapsCryptokiModule()
	if chapsPath == "" {
		return errors.New("unable to find chaps module")
	}

	// Remove the output first, if it exists.
	_, err := p.runner.Run(ctx, "rm", "-f", output)
	if err != nil {
		return errors.New("failed to remove the output before signing")
	}

	args := append([]string{"--module=" + chapsPath, "--slot=" + strconv.Itoa(key.slot), "--id=" + key.objID, "--sign", "-m", mechanism.toolMParam}, mechanism.toolExtraParam...)
	args = append(args, "-i", mechanism.toolSignInputFileProcessor(ctx, p.runner, input), "-o", output)
	_, err = p.runner.Run(ctx, "/usr/bin/pkcs11-tool", args...)
	if err != nil {
		return errors.Wrap(err, "failed to sign with "+mechanism.name+": ")
	}

	return nil
}

// Pkcs11VerifyWithOpenSSL verify the signature at |signaturePath|, signed with |key| and |mechanism|, and the message at |input|.
// It'll return nil iff the signature is valid.
func (p *Pkcs11Util) Pkcs11VerifyWithOpenSSL(ctx context.Context, key Pkcs11KeyInfo, input string, signaturePath string, mechanism Pkcs11MechanismInfo) error {
	// Verify with OpenSSL
	args := append([]string{"dgst", mechanism.opensslDgstParam, "-verify", key.pubKeyPath, "-keyform", "der"}, mechanism.opensslDgstExtraParam...)
	args = append(args, "-signature", signaturePath, input)
	binaryMsg, err := p.runner.Run(ctx, "openssl", args...)
	if err != nil {
		return errors.Wrap(err, "failed to verify the signature of "+mechanism.name+": ")
	}
	msg := string(binaryMsg)
	if msg != "Verified OK\n" {
		return errors.New("failed to verify the signature of " + mechanism.name + ": Message mismatch, unexpected: " + msg)
	}

	return nil
}

// Pkcs11SignVerify is just a convenient runner to test both signing and verification.
// |altInput| is path to another test file that differs in content to input. It is used to check that verify() indeed reject corrupted input.
func (p *Pkcs11Util) Pkcs11SignVerify(ctx context.Context, key Pkcs11KeyInfo, input string, altInput string, mechanism Pkcs11MechanismInfo) error {
	// Test signing.
	if err := p.Pkcs11Sign(ctx, key, input, input+".sig", mechanism); err != nil {
		return err
	}
	// Test verification of signed message.
	if err := p.Pkcs11VerifyWithOpenSSL(ctx, key, input, input+".sig", mechanism); err != nil {
		return err
	}
	// Test verification of another message (should fail).
	err := p.Pkcs11VerifyWithOpenSSL(ctx, key, altInput, input+".sig", mechanism)
	if err == nil {
		// Should not happen
		return errors.New("verification functionality for " + mechanism.name + " failed, corrupted message is verified")
	}
	return nil
}

// Pkcs11GetObjectAttribute retrieve the object of |objType| type and the id specified in |key|, and get its attribute |attributeName|. The returned tuple is (result, cmdMessage, error), error is nil iff the operation is successful, and in that case result holds the hex encoded attribute value. cmdMessage always holds the stdout of the p11_replay command, if such is available.
func (p *Pkcs11Util) Pkcs11GetObjectAttribute(ctx context.Context, key Pkcs11KeyInfo, objType string, attributeName string) (string, string, error) {
	cmd := fmt.Sprintf("p11_replay --get_attribute --slot=%d --id=%s --attribute=%s --type=%s", key.slot, key.objID, attributeName, objType)

	// Execute the command and convert its output.
	binaryMsg, err := p.runner.Run(ctx, "sh", "-c", cmd)
	var msg string
	if binaryMsg != nil {
		msg = string(binaryMsg)
	}

	if err != nil {
		return "", msg, errors.Wrap(err, "failed to get attribute with p11_replay: ")
	}

	const P11replayGetObjectAttributeDataPrefix string = "Attribute Data in hex: "

	var result = "<NULL>" // Note that the result from cryptohome command is hex encoded, so "<NULL>" is not possible.
	// Try to parse the result.
	for _, s := range strings.Split(msg, "\n") {
		if strings.HasPrefix(s, P11replayGetObjectAttributeDataPrefix) {
			if result != "<NULL>" {
				// Extra data.
				return "", msg, errors.New("extra data in parsing get object attribute output: ")
			}
			result = s[len(P11replayGetObjectAttributeDataPrefix):]
		}
	}
	return result, msg, nil
}

// Pkcs11SetObjectAttribute retrieve the object of |objType| type and the id specified in |key|, and set its attribute |attributeName| with the value |attributeValue|. The returned tuple is (cmdMessage, error), whereby error is nil iff the operation is successful. cmdMessage holds the stdout from p11_replay command if such is available.
func (p *Pkcs11Util) Pkcs11SetObjectAttribute(ctx context.Context, key Pkcs11KeyInfo, objType string, attributeName string, attributeValue string) (string, error) {
	cmd := fmt.Sprintf("p11_replay --set_attribute --slot=%d --id=%s --attribute=%s --data=%s --type=%s", key.slot, key.objID, attributeName, attributeValue, objType)

	// Execute the command and convert its output.
	binaryMsg, err := p.runner.Run(ctx, "sh", "-c", cmd)
	var msg string
	if binaryMsg != nil {
		msg = string(binaryMsg)
	}

	if err != nil {
		return msg, errors.Wrap(err, "failed to set attribute with p11_replay, call failed: ")
	}

	const P11replaySetObjectAttributeSuccessMsg string = "Set attribute OK."

	if !strings.Contains(msg, P11replaySetObjectAttributeSuccessMsg) {
		return msg, errors.New("failed to set attribute with p11_replay, incorrect response")
	}
	return msg, nil
}
