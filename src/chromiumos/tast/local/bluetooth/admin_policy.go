// Copyright 2022 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package bluetooth

import (
	"context"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/dbusutil"
)

const policyIface = service + ".AdminPolicyStatus1"

// ServiceAllowList returns the serviceAllowList of the adapter.
func (a *Adapter) ServiceAllowList(ctx context.Context) ([]string, error) {
	const prop = policyIface + ".ServiceAllowList"
	value, err := dbusutil.Property(ctx, a.obj, prop)
	if err != nil {
		return nil, err
	}
	serviceAllowList, ok := value.([]string)
	if !ok {
		return nil, errors.New("serviceAllowList property not a string slice")
	}
	return serviceAllowList, nil
}
