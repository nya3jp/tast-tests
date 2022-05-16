// Copyright 2022 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package tape holds methods for accessing the TAPE service.
package tape

import (
	"context"
	"time"

	"chromiumos/tast/common/tape"
	"chromiumos/tast/errors"
)

// ServiceAccountVar holds the name of the variable which stores the service account credentials for TAPE.
const ServiceAccountVar = "tape.service_account_key"

// LeaseGenericAccount leases a generic account given for the provided amount of time.
func LeaseGenericAccount(ctx context.Context, poolID string, leaseLength time.Duration, credsJSON []byte) (account *tape.GenericAccount, cleanup func(ctx context.Context) error, err error) {
	client, err := tape.NewTapeClient(ctx, tape.WithCredsJSON(credsJSON))
	if err != nil {
		return nil, nil, errors.Wrap(err, "failed to setup TAPE client")
	}

	params := tape.RequestGenericAccountParams{
		TimeoutInSeconds: int32(leaseLength.Seconds()),
		PoolID:           &poolID,
	}

	gar, err := tape.RequestGenericAccount(ctx, params, client)
	if err != nil {
		return gar, nil, errors.Wrap(err, "failed to request generic account")
	}

	return gar, func(ctx context.Context) error {
		if err := tape.ReleaseGenericAccount(ctx, gar, client); err != nil {
			return errors.Wrap(err, "failed to release generic account")
		}

		return nil
	}, nil
}
