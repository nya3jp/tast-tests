// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package shill

import (
	"context"

	"github.com/godbus/dbus"

	"chromiumos/tast/common/shillconst"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/dbusutil"
)

const (
	dbusServiceInterface = "org.chromium.flimflam.Service"
)

// Service wraps a Service D-Bus object in shill.
type Service struct {
	*dbusutil.PropertyHolder
}

// NewService connects to a service in Shill.
func NewService(ctx context.Context, path dbus.ObjectPath) (*Service, error) {
	ph, err := dbusutil.NewPropertyHolder(ctx, dbusService, dbusServiceInterface, path)
	if err != nil {
		return nil, err
	}
	return &Service{PropertyHolder: ph}, nil
}

// CreateWatcher returns a PropertiesWatcher to observe the Service "PropertyChanged" signal.
func (s *Service) CreateWatcher(ctx context.Context) (*PropertiesWatcher, error) {
	return NewPropertiesWatcher(ctx, s.DBusObject)
}

// GetDevice returns the Device object corresponding to the Service object
func (s *Service) GetDevice(ctx context.Context) (*Device, error) {
	serviceProps, err := s.GetProperties(ctx)
	if err != nil {
		return nil, err
	}
	deviceObjPath, err := serviceProps.GetObjectPath(shillconst.ServicePropertyDevice)
	if err != nil {
		return nil, errors.Wrap(err, "no device associated with service")
	}
	device, err := NewDevice(ctx, deviceObjPath)
	if err != nil {
		return nil, err
	}
	return device, nil
}

// Connect calls the Connect method on the service.
func (s *Service) Connect(ctx context.Context) error {
	return s.Call(ctx, "Connect").Err
}

// Disconnect calls the Disconnect method on the service.
func (s *Service) Disconnect(ctx context.Context) error {
	return s.Call(ctx, "Disconnect").Err
}

// Remove calls the Remove method on the service.
func (s *Service) Remove(ctx context.Context) error {
	return s.Call(ctx, "Remove").Err
}
