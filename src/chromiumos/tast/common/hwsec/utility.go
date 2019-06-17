// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package hwsec

import (
	"context"
	"time"

	apb "chromiumos/system_api/attestation_proto"
	"chromiumos/tast/dut"
	"chromiumos/tast/errors"
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

	// Asks DUT to take TPM ownership; returns the flag to tell if the operation
	// succeeds and any error encounted during the operation.
	TakeOwnership() (bool, error)

	IsMounted() (bool, error)

	Unmount(username string) (bool, error)

	CreateVault(username string, pass string) (bool, error)

	RemoveVault(username string) (bool, error)

	IsTpmWrappedKeySet(username string) (bool, error)

	// Creates a enroll request that is sent to the correspoding pca server of |PCAType|
	// later, and any error encountered during the operation.
	CreateEnrollRequest(PCAType int) (string, error)

	// Finishes the enroll with |resp| from pca server of |PCAType|. Returns any
	// encountered error during the operation.
	FinishEnroll(PCAType int, resp string) error

	// Creates a certifiacte request that is sent to the correspoding pca server
	// of |PCAType| later, and any error encountered during the operation.
	CreateCertRequest(PCAType int,
		profile apb.CertificateProfile,
		username string,
		origin string) (string, error)

	// Finishes the certified key creation with |resp| from pca server of
	// |PCAType|. Returns any encountered error during the operation.
	FinishCertRequest(response string, username string, label string) error

	// validates and then sign |challenge| from the VA server of |VAType|
	SignEnterpriseVAChallenge(
		VAType int,
		username string,
		label string,
		domain string,
		deviceID string,
		includeSignedPublicKey bool,
		challenge []byte) (string, error)

	SignSimpleChallenge(
		username string,
		label string,
		challenge []byte) (string, error)

	GetPublicKey(
		username string,
		label string) (string, error)

	GetKeyPayload(
		username string,
		label string) (string, error)

	SetKeyPayload(
		username string,
		label string,
		payload string) (bool, error)

	RegisterKeyWithChapsToken(
		username string,
		label string) (bool, error)

	SetAttestationAsyncMode(async bool) error

	GetEnrollmentId() (string, error)

	DeleteKeys(username string, prefix string) error

	sleep(milli int) error
}

// EnsureTpmIsReady ensures the TPM is ready when the function returns |nil|.
// Otherwise, returns any encountered error, including timeout after
// |timeoutInMs|.
func EnsureTpmIsReady(utility Utility, timeoutInMs int) error {
	expiredTimeInMs := time.Now().UnixNano()/int64(time.Millisecond) + int64(timeoutInMs)
	isReady, err := utility.IsTpmReady()
	if err != nil {
		return errors.Wrap(err, "failed to ensure ownership due to error in |IsTpmReady|")
	}
	if isReady == false {
		result, err := utility.TakeOwnership()
		if err != nil {
			return errors.Wrap(err, "failed to ensure ownership due to error in |TakeOwnership|")
		}
		if result == false {
			return errors.New("failed to take ownership")
		}
	} else {
		return nil
	}
	for expiredTimeInMs > time.Now().UnixNano()/int64(time.Millisecond) {
		// Ignores |err| here in case the error messages repeat undesirably.
		isReady, _ := utility.IsTpmReady()
		if isReady == false {
			err := utility.sleep(pollingIntervalMillis)
			if err != nil {
				return errors.Wrap(err, "timeout")
			}
		} else {
			return nil
		}
	}
	return errors.New("timeout")
}

// EnsureTpmIsReady ensures the TPM is ready when the function returns |nil|.
// Otherwise, returns any encountered error, including timeout after
// |timeoutInMs|.
func EnsureTpmIsReset(ctx context.Context, utility Utility) error {
	isReady, err := utility.IsTpmReady()
	if err != nil {
		return errors.Wrap(err, "failed to check ownership due to error in |IsTpmReady|")
	}
	if !isReady {
		return nil
	}
	d, ok := dut.FromContext(ctx)
	if !ok {
		return errors.New("failed to get DUT from context")
	}
	if _, err := d.Command("sh", "-c", "crossystem clear_tpm_owner_request=1").CombinedOutput(ctx); err != nil {
		return errors.Wrap(err, "failed to file clear_tpm_owner_request")
	}
	if _, err := d.Command("sh", "-c", "rm /mnt/stateful_partition/unencrypted/preserve/attestation.epb").CombinedOutput(ctx); err != nil {
		return errors.Wrap(err, "failed to remove attestation database")
	}
	if err = reboot(ctx); err != nil {
		return errors.Wrap(err, "failed to reboot")
	}
	return nil
	isReady, err = utility.IsTpmReady()
	if err != nil {
		return errors.Wrap(err, "failed to check if TPM is resetdue to error in |IsTpmReady|")
	}
	if isReady {
		return errors.New("ineffective reset of tpm")
	}
	return nil
}

// EnsureIsPreparedForEnrollment ensures the DUT is prepareed for enrollment
// when the function returns |nil|. Otherwise, returns any encountered error,
// including timeout after |timeoutInMs|.
func EnsureIsPreparedForEnrollment(utility Utility, timeoutInMs int) error {
	expiredTimeInMs := time.Now().UnixNano()/int64(time.Millisecond) + int64(timeoutInMs)
	for expiredTimeInMs > time.Now().UnixNano()/int64(time.Millisecond) {
		// Ignores |err| here in case the error messages repeat undesirably.
		isPrepared, err := utility.IsPreparedForEnrollment()
		if err != nil {
			return errors.Wrap(err, "failed to determine if prepared for enrollment")
		}
		if isPrepared == false {
			err := utility.sleep(pollingIntervalMillis)
			if err != nil {
				return errors.Wrap(err, "timeout")
			}
		} else {
			return nil
		}
	}
	return errors.New("timeout")
}

// NewUtility returns the implementation corresponding to
// |clientType|. See the document of each implementation for more details.
func NewUtility(ctx context.Context, clientType ClientType) (Utility, error) {
	switch clientType {
	case CryptohomeProxyLegacyType:
		proxy, err := NewCryptohomeProxyLegacy(ctx)
		if err != nil {
			return utilityCryptohomeLegacy{}, err
		}
		return utilityCryptohomeLegacy{utilityCommon{ctx}, proxy}, nil
	case CryptohomeBinaryType:
		proxy, err := NewCryptohomeBinary(ctx)
		if err != nil {
			return nil, err
		}
		b := true
		return utilityCryptohomeBinary{utilityCommon{ctx}, proxy, &b}, nil
	case CryptohomeProxyNewType, DistributedModeProxyType:
		return nil, errors.New("not implemented")
	default:
		return nil, errors.New("unrezognized client type")
	}
}
