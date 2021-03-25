// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package shill

import (
	"context"

	"github.com/godbus/dbus"

	"chromiumos/tast/common/shillconst"
	"chromiumos/tast/errors"
)

const (
	dbusServiceInterface = "org.chromium.flimflam.Service"
)

// Service wraps a Service D-Bus object in shill.
type Service struct {
	*PropertyHolder
}

// NewService connects to a service in Shill.
func NewService(ctx context.Context, path dbus.ObjectPath) (*Service, error) {
	ph, err := NewPropertyHolder(ctx, dbusService, dbusServiceInterface, path)
	if err != nil {
		return nil, err
	}
	return &Service{PropertyHolder: ph}, nil
}

// GetDevice returns the Device object corresponding to the Service object
func (s *Service) GetDevice(ctx context.Context) (*Device, error) {
	serviceProps, err := s.GetShillProperties(ctx)
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

// IsConnected returns true if the the service is connected.
func (s *Service) IsConnected(ctx context.Context) (bool, error) {
	properties, err := s.GetShillProperties(ctx)
	if err != nil {
		return false, errors.Wrap(err, "unable to get properties")
	}
	connected, err := properties.GetBool(shillconst.ServicePropertyIsConnected)
	if err != nil {
		return false, errors.Wrap(err, "unable to get IsConnected from properties")
	}
	return connected, nil
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
