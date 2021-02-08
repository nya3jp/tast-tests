// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package hwsec

import (
	"context"
)

// TPMClearer is an interface of to clear the TPM.
type TPMClearer interface {
	// ClearTPMStep1 should be called before stop high-level TPM daemons
	ClearTPMStep1(ctx context.Context) error

	// ClearTPMStep1 should be called before clean the data on device
	ClearTPMStep2(ctx context.Context) error

	// ClearTPMStep1 should be called after clean the data on device
	ClearTPMStep3(ctx context.Context) error
}
