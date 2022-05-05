// Copyright 2022 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package bluetooth

import (
	"context"

	"github.com/godbus/dbus"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/dbusutil"
)

// AdminPolicy provides readonly properties to indicate the current values of
// admin policy.
type AdminPolicy struct {
	obj  dbus.BusObject
	path dbus.ObjectPath
}

const policyIface = service + ".AdminPolicyStatus1"

// AdminPolicies creates an AdminPolicy for all bluetooth adapters in the system.
func AdminPolicies(ctx context.Context) ([]*AdminPolicy, error) {
	var adminPolicies []*AdminPolicy
	_, obj, err := dbusutil.Connect(ctx, service, "/")
	if err != nil {
		return nil, err
	}
	managed, err := dbusutil.ManagedObjects(ctx, obj)
	if err != nil {
		return nil, err
	}
	for _, path := range managed[policyIface] {
		adminPolicy, err := NewAdminPolicies(ctx, path)
		if err != nil {
			return nil, err
		}
		adminPolicies = append(adminPolicies, adminPolicy)
	}
	return adminPolicies, nil
}

// NewAdminPolicies creates a new bluetooth AdminPolicy from the passed D-Bus object path.
func NewAdminPolicies(ctx context.Context, path dbus.ObjectPath) (*AdminPolicy, error) {
	_, obj, err := dbusutil.Connect(ctx, service, path)
	if err != nil {
		return nil, err
	}
	return &AdminPolicy{obj, path}, nil
}

// Path gets the D-Bus path this device was created from.
func (a *AdminPolicy) Path() dbus.ObjectPath {
	return a.path
}

// ServiceAllowList returns the serviceAllowList of the adapter.
func (a *AdminPolicy) ServiceAllowList(ctx context.Context) ([]string, error) {
	const prop = policyIface + ".ServiceAllowList"
	value, err := dbusutil.Property(ctx, a.obj, prop)
	if err != nil {
		return []string{}, err
	}
	serviceAllowList, ok := value.([]string)
	if !ok {
		return []string{}, errors.New("serviceAllowList property not a string slice")
	}
	return serviceAllowList, nil
}
