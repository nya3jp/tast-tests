// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package hwsec

import (
	"context"
)

// TPMClearer is an interface of to clear the TPM.
type TPMClearer interface {
	// PreClearTPM should be called before stop high-level TPM daemons
	PreClearTPM(ctx context.Context) error

	// ClearTPM should be called before clean the data on device
	ClearTPM(ctx context.Context) error

	// PostClearTPM should be called after clean the data on device
	PostClearTPM(ctx context.Context) error
}

// SystemStateFiles represents the files that contining system states.
var SystemStateFiles = []string{
	"/home/chronos/.oobe_completed",
	"/mnt/stateful_partition/.tpm_owned",
	"/var/cache/shill/default.profile",
	"/home/.shadow/",
	"/home/chronos/Local State/",
	"/run/cryptohome/",
	"/run/lockbox/",
	"/run/tpm_manager/",
	"/var/cache/app_pack/",
	"/var/lib/",
}
