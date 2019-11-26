// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package hwsec

import (
	"context"
	"time"

	apb "chromiumos/system_api/attestation_proto"
	"chromiumos/tast/errors"
	"chromiumos/tast/testing"
)

// ClientType is an alias of string which is used as an enum to specify a
// |Utility| implementation type.
type ClientType string

// The collection of all valid |ClientType|s.
const (
	// CryptohomeProxyLegacyType refers to the implementation that talks to
	// cryptohomed via legacy cryptohome dbus interface.
	CryptohomeProxyLegacyType ClientType = "CryptohomeProxyLegacy"
	CryptohomeProxyNewType    ClientType = "CryptohomeProxyNew"
	DistributedModeProxyType  ClientType = "DistributedModeProxy"
	CryptohomeBinaryType      ClientType = "CryptohomeBinary"
)

const (
	pollingIntervalMillis int = 10
)

// Utility is a collection of utility functions that run hwsec-related commands on the DUT.
type Utility interface {
	// IsTpmReady returns the flag to indicate if TPM is ready and any encounted error during the opeation.
	IsTpmReady() (bool, error)

	// IsPreparedForEnrollment returns the flag to indicate if the DUT is
	// prepared for enrollment and any encounted error during the opeation.
	IsPreparedForEnrollment() (bool, error)

	// IsEnrolled returns the flag to indicate if the DUT is
	// enrolled and any encounted error during the opeation.
	IsEnrolled() (bool, error)

	// Take TPM ownership on DUT and wait until either ownership is taken or the operation fails; returns the flag to tell if the the DUT's TPM is owned after the call and any error encounted during the operation.
	EnsureOwnership() (bool, error)

	// Checks if any currently mounted vault; If the operation succeeds, then error will be nil, and the bool will contain if any user vault is mounted (true if any vault is mounted). Otherwise, an error is returned.
	IsMounted() (bool, error)

	// Unmounts the vault of |username|
	Unmount(username string) (bool, error)

	// Creates the vault for |username| if not exist.
	CreateVault(username string, password string) (bool, error)

	// Removes the vault of |username|.
	RemoveVault(username string) (bool, error)

	// Reports if the vault key of |username| is TPM-backed.
	IsTpmWrappedKeySet(username string) (bool, error)

	// Get the token for user specified in |username|.
	GetTokenForUser(username string) (int, error)

	// Creates an enroll request that is sent to the corresponding pca server of |PCAType|
	// later, and any error encountered during the operation.
	CreateEnrollRequest(PCAType int) (string, error)

	// Finishes the enroll with |resp| from pca server of |PCAType|. Returns any
	// encountered error during the operation.
	FinishEnroll(PCAType int, resp string) error

	// Creates a certificate request that is sent to the corresponding pca server
	// of |PCAType| later, and any error encountered during the operation.
	CreateCertRequest(PCAType int,
		profile apb.CertificateProfile,
		username string,
		origin string) (string, error)

	// Finishes the certified key creation with |resp| from PCA server. Returns any encountered error during the operation.
	FinishCertRequest(response string, username string, label string) error

	// Validates and then sign |challenge| from the VA server. See attestationd's dbus interface for document of the arguments.
	SignEnterpriseVAChallenge(
		VAType int,
		username string,
		label string,
		domain string,
		deviceID string,
		includeSignedPublicKey bool,
		challenge []byte) (string, error)

	// Signs |challenge| with the attestation key of |username| with |label|. Uses system-level key if |username| is empty.
	SignSimpleChallenge(
		username string,
		label string,
		challenge []byte) (string, error)

	// Gets the public key of |username| with |label|. Gets system-level key if |username| is empty.
	GetPublicKey(
		username string,
		label string) (string, error)

	// Gets the key payload of |username| with |label|. Gets system-level key payload if |username| is empty.
	GetKeyPayload(
		username string,
		label string) (string, error)

	// Sets the key payload of |username| with |label|. Sets system-level key payload  if |username| is empty.
	SetKeyPayload(
		username string,
		label string,
		payload string) (bool, error)

	// Registers the key of |username| with |label|. Registers system-level key if |username| is empty.
	RegisterKeyWithChapsToken(
		username string,
		label string) (bool, error)

	// Sets the enrollment and cert request process to be handled in asynchronous way.
	// Currently only applicable to implementation using cryptohome binary.
	SetAttestationAsyncMode(async bool) error

	// Gets the EID used for attestation.
	GetEnrollmentID() (string, error)

	// Gets the TPM owner password. Note that empty owner password doesn't causes errors.
	GetOwnerPassword() (string, error)

	// Delete all keys of |username| with label prefixed by |prefix| in attestation. Deletes system-level keys if |username| is empty.
	DeleteKeys(username string, prefix string) error
}

// EnsureTpmIsReady ensures the TPM is ready when the function returns |nil|.
// Otherwise, returns any encountered error, including timeout after
// |timeoutInMs|.
func EnsureTpmIsReady(ctx context.Context, utility Utility, timeoutInMs int) error {
	isReady, err := utility.IsTpmReady()
	if err != nil {
		return errors.Wrap(err, "failed to ensure ownership due to error in |IsTpmReady|")
	}
	if isReady == false {
		result, err := utility.EnsureOwnership()
		if err != nil {
			return errors.Wrap(err, "failed to ensure ownership due to error in |TakeOwnership|")
		}
		if result == false {
			return errors.New("failed to take ownership")
		}
	}
	return testing.Poll(ctx, func(context.Context) error {
		isReady, _ := utility.IsTpmReady()
		if isReady {
			return nil
		}
		return errors.New("haven't confirmed to be owned")
	}, &testing.PollOptions{
		Timeout:  time.Duration(timeoutInMs) * time.Millisecond,
		Interval: time.Duration(pollingIntervalMillis) * time.Millisecond,
	})
}

// EnsureTpmIsReset ensures the TPM is reset when the function returns |nil|.
// Otherwise, returns any encountered error.
func EnsureTpmIsReset(ctx context.Context, s *testing.State, utility Utility) error {
	// Note that this method runs on remote test only. The reason why it's in this file is because we want to keep this method and the others in this series together.
	isReady, err := utility.IsTpmReady()
	if err != nil {
		return errors.Wrap(err, "failed to check ownership due to error in |IsTpmReady|")
	}
	if !isReady {
		return nil
	}
	if _, err := call(ctx, s, "crossystem clear_tpm_owner_request=1"); err != nil {
		return errors.Wrap(err, "failed to file clear_tpm_owner_request")
	}
	if err = reboot(ctx, s); err != nil {
		return errors.Wrap(err, "failed to reboot")
	}
	isReady, err = utility.IsTpmReady()
	if err != nil {
		return errors.Wrap(err, "failed to check if TPM is reset due to error in |IsTpmReady|")
	}
	if isReady {
		return errors.New("ineffective reset of tpm")
	}
	return nil
}

// EnsureIsPreparedForEnrollment ensures the DUT is prepareed for enrollment
// when the function returns |nil|. Otherwise, returns any encountered error,
// including timeout after |timeoutInMs|.
func EnsureIsPreparedForEnrollment(ctx context.Context, utility Utility, timeoutInMs int) error {
	return testing.Poll(ctx, func(context.Context) error {
		// intentionally ignores error; retry the operation until timeout.
		isPrepared, err := utility.IsPreparedForEnrollment()
		if err != nil {
			return err
		}
		if !isPrepared {
			return errors.New("not prepared yet")
		}
		return nil
	}, &testing.PollOptions{
		Timeout:  time.Duration(timeoutInMs) * time.Millisecond,
		Interval: time.Duration(pollingIntervalMillis) * time.Millisecond,
	})
}

// NewUtility returns the implementation corresponding to
// |clientType|. See the document of each implementation for more details.
func NewUtility(ctx context.Context, s *testing.State, clientType ClientType) (Utility, error) {
	switch clientType {
	case CryptohomeBinaryType:
		proxy, err := NewCryptohomeBinary(ctx, s)
		if err != nil {
			return nil, err
		}
		defaultAsynAttestationMode := true
		return utilityCryptohomeBinary{utilityCommon{ctx, s}, proxy, &defaultAsynAttestationMode}, nil
	case CryptohomeProxyLegacyType, CryptohomeProxyNewType, DistributedModeProxyType:
		return nil, errors.New("not implemented")
	default:
		return nil, errors.New("unrecognized client type")
	}
}
