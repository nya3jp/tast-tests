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
	// CryptohomeProxyLegacyType refers to |hwsecUtilityCryptohomeProxyLegacy|
	CryptohomeProxyLegacyType ClientType = "CryptohomeProxyLegacy"
	CryptohomeProxyNewType    ClientType = "CryptohomeProxyNew"
	DistributedModeProxyType  ClientType = "DistributedModeProxy"
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
	Sleep(milli int) error
}

// EnsureTpmIsReady ensures the TPM is ready when the function returns |nil|.
// Otherwise, returns any encountered error, including timeout after
// |timeoutInMillis|.
func EnsureTpmIsReady(utility Utility, timeoutInMillis int) error {
	expiredTimeInMillis := time.Now().UnixNano()/int64(time.Millisecond) + int64(timeoutInMillis)
	isReady, err := utility.IsTpmReady()
	if err != nil {
		return errors.Wrap(err, "Failed to ensure ownership due to error in |IsTpmReady|")
	}
	if isReady == false {
		result, err := utility.TakeOwnership()
		if err != nil {
			return errors.Wrap(err, "Failed to ensure ownership due to error in |TakeOwnership|")
		}
		if result == false {
			return errors.New("Failed to take ownership")
		}
	} else {
		return nil
	}
	for expiredTimeInMillis > time.Now().UnixNano()/int64(time.Millisecond) {
		// Ignores |err| here in case the error messages repeat undesirably.
		isReady, _ := utility.IsTpmReady()
		if isReady == false {
			err := utility.Sleep(pollingIntervalMillis)
			if err != nil {
				return errors.Wrap(err, "timeout")
			}
		} else {
			return nil
		}
	}
	return errors.New("timeout")
}

// EnsureIsPreparedForEnrollment ensures the DUT is prepareed for enrollment when the function returns |nil|. Otherwise, returns any encountered error, including timeout after |timeoutInMillis|.
func EnsureIsPreparedForEnrollment(utility Utility, timeoutInMillis int) error {
	expiredTimeInMillis := time.Now().UnixNano()/int64(time.Millisecond) + int64(timeoutInMillis)
	for expiredTimeInMillis > time.Now().UnixNano()/int64(time.Millisecond) {
		// Ignores |err| here in case the error messages repeat undesirably.
		isPrepared, _ := utility.IsPreparedForEnrollment()
		if isPrepared == false {
			err := utility.Sleep(pollingIntervalMillis)
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
			return hwsecUtilityCryptohomeLegacy{}, err
		}
		return hwsecUtilityCryptohomeLegacy{utilityCommon{ctx}, proxy}, nil
	case CryptohomeProxyNewType, DistributedModeProxyType:
		return nil, errors.New("not implemented")
	default:
		return nil, errors.New("unrezognized client type")
	}
}

// utilityCommon implements the common function shared across all
// implementations of |Utility|.
type utilityCommon struct {
	ctx context.Context
}

func (utility utilityCommon) Sleep(millis int) error {
	return testing.Sleep(utility.ctx, time.Duration(millis)*time.Millisecond)
}
