// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package hwsec

import (
	"time"
)

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
)
