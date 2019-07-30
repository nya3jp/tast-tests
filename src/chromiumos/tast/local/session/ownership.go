// Copyright 2018 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package session

import (
	"context"
	"crypto"
	"crypto/rsa"
	"crypto/sha1"
	"crypto/x509"
	"io/ioutil"
	"os"
	"time"

	"github.com/golang/protobuf/proto"
	"golang.org/x/crypto/pkcs12"

	"chromiumos/policy/enterprise_management"
	lm "chromiumos/system_api/login_manager_proto"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/upstart"
	"chromiumos/tast/testing"
)

const (
	// PolicyPath is a directory containing policy files.
	PolicyPath = "/var/lib/whitelist"

	// localStatePath is a file containing local state JSON.
	localStatePath = "/home/chronos/Local State"
)

// SetUpDevice prepares the device for ownership & policy related tests.
func SetUpDevice(ctx context.Context) error {
	const uiSetupTimeout = 90 * time.Second

	testing.ContextLog(ctx, "Restarting ui job")
	sctx, cancel := context.WithTimeout(ctx, uiSetupTimeout)
	defer cancel()

	if err := upstart.StopJob(sctx, "ui"); err != nil {
		return errors.Wrap(err, "failed to stop ui job")
	}
	// In case of error, run EnsureJobRunning with the original
	// context to recover the job for the following tests.
	// In case of success, this is (effectively) no-op.
	defer upstart.EnsureJobRunning(ctx, "ui")

	if err := ClearDeviceOwnership(sctx); err != nil {
		return errors.Wrap(err, "failed to clear device ownership")
	}
	if err := upstart.EnsureJobRunning(sctx, "ui"); err != nil {
		return errors.Wrap(err, "failed to restart ui job")
	}
	return nil
}

// PrepareChromeForTesting prepares Chrome for ownership or policy tests.
// This prevents a crash on startup due to synchronous profile creation and not
// knowing whether to expect policy, see https://crbug.com/950812.
func PrepareChromeForTesting(ctx context.Context, m *SessionManager) error {
	_, err := m.EnableChromeTesting(ctx, true, []string{"--profile-requires-policy=true"}, []string{})
	return err
}

// ClearDeviceOwnership deletes DUT's ownership infomation.
func ClearDeviceOwnership(ctx context.Context) error {
	testing.ContextLog(ctx, "Clearing device owner info")

	// The UI must be stopped while we do this, or the session_manager will
	// write the policy and key files out again.
	if goal, state, _, err := upstart.JobStatus(ctx, "ui"); err != nil {
		return err
	} else if goal != upstart.StopGoal || state != upstart.WaitingState {
		return errors.Errorf("device ownership is being cleared while ui is not stopped: %v/%v", goal, state)
	}

	if err := os.RemoveAll(PolicyPath); err != nil {
		return errors.Wrapf(err, "failed to remove %s", PolicyPath)
	}

	if err := os.Remove(localStatePath); err != nil && !os.IsNotExist(err) {
		return errors.Wrapf(err, "failed to remove %s", localStatePath)
	}

	return nil
}

// DevicePolicyDescriptor creates a PolicyDescriptor suitable for storing and
// retrieving device policy using session_manager's policy storage interface.
func DevicePolicyDescriptor() *lm.PolicyDescriptor {
	accountType := lm.PolicyAccountType_ACCOUNT_TYPE_DEVICE
	domain := lm.PolicyDomain_POLICY_DOMAIN_CHROME
	return &lm.PolicyDescriptor{
		AccountType: &accountType,
		Domain:      &domain,
	}
}

// ExtractPrivKey reads a PKCS #12 format file at path, then extracts and
// returns RSA private key.
func ExtractPrivKey(path string) (*rsa.PrivateKey, error) {
	p12, err := ioutil.ReadFile(path)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to read %s", path)
	}
	key, _, err := pkcs12.Decode(p12, "" /* password */)
	if err != nil {
		return nil, errors.Wrap(err, "failed to decode p12 file")
	}
	privKey, ok := key.(*rsa.PrivateKey)
	if !ok {
		return nil, errors.New("RSA private key is not found")
	}
	return privKey, nil
}

// BuildPolicy creates PolicyFetchResponse used in session_manager from
// the given parameters.
func BuildPolicy(user string, key, oldKey *rsa.PrivateKey, s *enterprise_management.ChromeDeviceSettingsProto) (*enterprise_management.PolicyFetchResponse, error) {
	sdata, err := proto.Marshal(s)
	if err != nil {
		return nil, errors.Wrap(err, "failed to serialize settings")
	}
	polType := "google/chromeos/device"
	pol := &enterprise_management.PolicyData{
		PolicyType:  &polType,
		PolicyValue: sdata,
	}
	if user != "" {
		pol.Username = &user
	}
	polData, err := proto.Marshal(pol)
	if err != nil {
		return nil, errors.Wrap(err, "failed to serialize policy")
	}
	polSign, err := sign(key, polData)
	if err != nil {
		return nil, errors.Wrap(err, "failed to sign policy data")
	}

	pubDer, err := x509.MarshalPKIXPublicKey(&key.PublicKey)
	if err != nil {
		return nil, errors.Wrap(err, "failed to marshal public key to DER")
	}
	if oldKey == nil {
		oldKey = key
	}
	pubSign, err := sign(oldKey, pubDer)
	if err != nil {
		return nil, errors.Wrap(err, "failed to serialize public key")
	}

	return &enterprise_management.PolicyFetchResponse{
		PolicyData:            polData,
		PolicyDataSignature:   polSign,
		NewPublicKey:          pubDer,
		NewPublicKeySignature: pubSign,
	}, nil
}

// StoreSettings requests given SessionManager to store the
// ChromeDeviceSettingsProto data for the user with key.
func StoreSettings(ctx context.Context, sm *SessionManager, user string, key, oldKey *rsa.PrivateKey, s *enterprise_management.ChromeDeviceSettingsProto) error {
	response, err := BuildPolicy(user, key, oldKey, s)
	if err != nil {
		return err
	}

	// Send the data to session_manager.
	w, err := sm.WatchPropertyChangeComplete(ctx)
	if err != nil {
		return errors.Wrap(err, "failed to start watching PropertyChangeComplete signal")
	}
	defer w.Close(ctx)
	if err := sm.StorePolicyEx(ctx, DevicePolicyDescriptor(), response); err != nil {
		return errors.Wrap(err, "failed to call StorePolicyEx")
	}
	select {
	case <-w.Signals:
	case <-ctx.Done():
		return errors.Wrap(ctx.Err(), "timed out waiting for PropertyChangeComplete signal")
	}

	return nil
}

// sign signs the blob with the given key, and returns the signature.
func sign(key *rsa.PrivateKey, blob []byte) ([]byte, error) {
	h := sha1.New()
	h.Write(blob)
	digest := h.Sum(nil)
	return rsa.SignPKCS1v15(nil, key, crypto.SHA1, digest)
}

// RetrieveSettings requests to given SessionManager to return the currently
// stored ChromeDeviceSettingsProto.
func RetrieveSettings(ctx context.Context, sm *SessionManager) (*enterprise_management.ChromeDeviceSettingsProto, error) {
	ret, err := sm.RetrievePolicyEx(ctx, DevicePolicyDescriptor())
	if err != nil {
		return nil, errors.Wrap(err, "failed to retrieve policy")
	}

	rPol := &enterprise_management.PolicyData{}
	if err = proto.Unmarshal(ret.PolicyData, rPol); err != nil {
		return nil, errors.Wrap(err, "failed to parse PolicyData")
	}

	rsettings := &enterprise_management.ChromeDeviceSettingsProto{}
	if err = proto.Unmarshal(rPol.PolicyValue, rsettings); err != nil {
		return nil, errors.Wrap(err, "failed to parse PolicyValue")
	}
	return rsettings, nil
}
