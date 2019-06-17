// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package hwsec

const (
	// DefaultTakingOwnershipTimeout is the default timeout while taking TPM ownership.
	DefaultTakingOwnershipTimeout int = 40 * 1000
	// DefaultPreparationForEnrolmentTimeout is the default timeout for attestation to be prepared.
	DefaultPreparationForEnrolmentTimeout int = 40 * 1000
	// AttestationDBPath is the path of attestation database.
	AttestationDBPath string = "/mnt/stateful_partition/unencrypted/preserve/attestation.epb"
	// TpmManagerLocalDataPath is the path of tpm_manager local data (only applicable for distributed model).
	TpmManagerLocalDataPath string = "/var/lib/tpm_manager/local_tpm_data"
)
