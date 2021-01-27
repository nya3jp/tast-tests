// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package hwsec

import (
	"context"

	apb "chromiumos/system_api/attestation_proto"
)

// AttestationClient is an interface of attestation client.
type AttestationClient interface {
	// GetStatus returns the attestation status.
	GetStatus(ctx context.Context, req *apb.GetStatusRequest) (*apb.GetStatusReply, error)
}
