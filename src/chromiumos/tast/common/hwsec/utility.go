// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package hwsec

import (
	apb "chromiumos/system_api/attestation_proto"
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
	// IsTPMReady returns the flag to indicate if TPM is ready and any encounted error during the opeation.
	IsTPMReady() (bool, error)

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

	// Checks the vault for |username|.
	CheckVault(username string, password string) (bool, error)

	// Removes the vault of |username|.
	RemoveVault(username string) (bool, error)

	// Reports if the vault key of |username| is TPM-backed.
	IsTPMWrappedKeySet(username string) (bool, error)

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

	// Clears the TPM owner password if the owner dependencies are all gone.
	ClearOwnerPassword() error

	// Delete all keys of |username| with label prefixed by |prefix| in attestation. Deletes system-level keys if |username| is empty.
	DeleteKeys(username string, prefix string) error
}
