// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package hwsec

import (
	"time"

	apb "chromiumos/system_api/attestation_proto"
)

// VAType indicates the type VA server, of which the possible value are default and test; see the const definition below.
type VAType int

// ACAType indicates the type ACA server, of which the possible value are default and test; see the const definition below.
type ACAType int

// PCAType is basically an alias of ACAType from legacy cryptohome's terminology.
type PCAType ACAType

const (
	// PollingInterval is the polling interval we use in this library and the libraries extending this.
	PollingInterval = 100 * time.Millisecond
	// DefaultTakingOwnershipTimeout is the default timeout while taking TPM ownership.
	DefaultTakingOwnershipTimeout = 40 * time.Second
	// DefaultPreparationForEnrolmentTimeout is the default timeout for attestation to be prepared.
	DefaultPreparationForEnrolmentTimeout = 40 * time.Second
	// AttestationDBPath is the path of attestation database.
	AttestationDBPath = "/mnt/stateful_partition/unencrypted/preserve/attestation.epb"
	// TpmManagerLocalDataPath is the path of tpm_manager local data (only applicable for distributed model).
	TpmManagerLocalDataPath = "/var/lib/tpm_manager/local_tpm_data"
	// OwnerPasswordLength is the owner password length we set by our implementation.
	OwnerPasswordLength = 20
)

const (
	// DefaultACA indicates the default ACA server.
	DefaultACA ACAType = iota
	// TestACA indicates the test ACA server.
	TestACA
)

const (
	// DefaultPCA indicates the default PCA server.
	DefaultPCA PCAType = iota
	// TestPCA indicates the test PCA server.
	TestPCA
)

const (
	// DefaultVA indicates the default VA server.
	DefaultVA VAType = iota
	// TestVA indicates the test VA server.
	TestVA
)

const (
	// DefaultCertProfile is the default cert profile we use when tesing.
	DefaultCertProfile apb.CertificateProfile = apb.CertificateProfile_ENTERPRISE_USER_CERTIFICATE
	// DefaultCertOrigin is the default value of the certificate origin.
	DefaultCertOrigin string = ""
	// DefaultCertLabel is the default label to identify the cert.
	DefaultCertLabel string = "aaa"
	// DefaultKeyPayload is the default key playload used for testing.
	DefaultKeyPayload string = "payload"
)
