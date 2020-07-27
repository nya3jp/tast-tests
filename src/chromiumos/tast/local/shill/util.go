// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// This file provides small helpers, packaging shill APIs in ways to ease their
// use by others.

package shill

import (
	"context"
	"time"

	"chromiumos/tast/common/shillconst"
	"chromiumos/tast/errors"
)

// WaitForOnline waits for Internet connectivity, a shorthand which is useful so external packages don't have to worry
// about Shill details (e.g., Service, Manager). Tests that require Internet connectivity (e.g., for a real GAIA login)
// need to ensure that before trying to perform Internet requests. This function is one way to do that.
// Returns an error if we don't come back online within a reasonable amount of time.
func WaitForOnline(ctx context.Context) error {
	m, err := NewManager(ctx)
	if err != nil {
		return errors.Wrap(err, "failed to connect to shill's Manager")
	}

	expectProps := map[string]interface{}{
		shillconst.ServicePropertyState: shillconst.ServiceStateOnline,
	}
	if _, err := m.WaitForServiceProperties(ctx, expectProps, 15*time.Second); err != nil {
		return errors.Wrap(err, "network did not come back online")
	}

	return nil
}
