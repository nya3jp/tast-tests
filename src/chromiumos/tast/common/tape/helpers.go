// Copyright 2022 The ChromiumOS Authors.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package tape

import (
	"context"
	"time"

	"chromiumos/tast/errors"
)

// ServiceAccountVar holds the name of the variable which stores the service account credentials for TAPE.
const ServiceAccountVar = "tape.service_account_key"

// LeaseAccount leases an account from the given poolID for the provided amount of time and optionally locks it.
func LeaseAccount(ctx context.Context, poolID string, leaseLength time.Duration, lockAccount bool, credsJSON []byte) (account *Account, cleanup func(ctx context.Context) error, err error) {
	client, err := NewTapeClient(ctx, WithCredsJSON(credsJSON))
	if err != nil {
		return nil, nil, errors.Wrap(err, "failed to setup TAPE client")
	}

	params := NewRequestOTAParams(int32(leaseLength.Seconds()), &poolID, false)
	acc, err := RequestAccount(ctx, client, params)
	if err != nil {
		return acc, nil, errors.Wrap(err, "failed to request an account")
	}

	return acc, func(ctx context.Context) error {
		if err := acc.ReleaseAccount(ctx, client); err != nil {
			return errors.Wrap(err, "failed to release an account")
		}
		return nil
	}, nil
}
