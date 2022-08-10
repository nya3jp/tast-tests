// Copyright 2022 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package bluez

import (
	"context"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/dbusutil"
)

// ServiceAllowList returns the serviceAllowList of the adapter.
func (a *Adapter) ServiceAllowList(ctx context.Context) ([]string, error) {
	prop := dbusutil.BuildIfacePath(bluezAdminPolicyStatusIface, "ServiceAllowList")
	value, err := dbusutil.Property(ctx, a.dbus.Obj(), prop)
	if err != nil {
		return nil, err
	}
	serviceAllowList, ok := value.([]string)
	if !ok {
		return nil, errors.New("serviceAllowList property not a string slice")
	}
	return serviceAllowList, nil
}
