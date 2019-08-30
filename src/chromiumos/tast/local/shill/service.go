// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package shill provides D-Bus wrappers and utilities for shill service.
package shill

import (
	"context"

	"github.com/godbus/dbus"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/dbusutil"
)

const (
	dbusServiceInterface = "org.chromium.flimflam.Service"
)

// Service wraps a Service D-Bus object in shill.
type Service struct {
	obj dbus.BusObject
}

// NewService connects to a service in Shill.
func NewService(ctx context.Context, path dbus.ObjectPath) (*Service, error) {
	_, obj, err := dbusutil.Connect(ctx, dbusService, path)
	if err != nil {
		return nil, err
	}
	s := &Service{obj: obj}
	return s, nil
}

// GetProperties returns a list of properties provided by the service.
func (s *Service) GetProperties(ctx context.Context) (map[string]interface{}, error) {
	props := make(map[string]interface{})
	if err := call(ctx, s.obj, dbusServiceInterface, "GetProperties").Store(&props); err != nil {
		return nil, errors.Wrap(err, "failed getting properties")
	}
	return props, nil
}

// SetProperty sets a string property to the given value
func (s *Service) SetProperty(ctx context.Context, property string, val interface{}) error {
	return call(ctx, s.obj, dbusServiceInterface, "SetProperty", property, val).Err
}
