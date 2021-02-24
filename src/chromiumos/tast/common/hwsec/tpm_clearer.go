// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package hwsec

import (
	"context"
)

// TPMClearer is an interface of to clear the TPM.
type TPMClearer interface {
	// ClearTPM clears the TPM.
	ClearTPM(ctx context.Context) error
}
