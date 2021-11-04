// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package shill

import (
	"context"
	"time"

	"github.com/godbus/dbus"

	"chromiumos/tast/common/shillconst"
	"chromiumos/tast/errors"
	"chromiumos/tast/testing"
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

// GetSignalStrength return the current signal strength
func (s *Service) GetSignalStrength(ctx context.Context) (uint8, error) {
	props, err := s.GetProperties(ctx)
	if err != nil {
		return 0, errors.Wrap(err, "unable to get properties")
	}
	strength, err := props.GetUint8(shillconst.ServicePropertyStrength)
	if err != nil {
		return 0, errors.Wrap(err, "unable to get strength from properties")
	}
	return strength, nil
}

// IsConnected returns true if the the service is connected.
func (s *Service) IsConnected(ctx context.Context) (bool, error) {
	props, err := s.GetProperties(ctx)
	if err != nil {
		return false, errors.Wrap(err, "unable to get properties")
	}
	connected, err := props.GetBool(shillconst.ServicePropertyIsConnected)
	if err != nil {
		return false, errors.Wrap(err, "unable to get IsConnected from properties")
	}
	return connected, nil
}

// WaitForConnectedOrError polls for either:
// * Service.IsConnected to be true, in which case nil is returned.
// * Service.Error to be set to an error value, in which case that is returned as an error.
// Any failure also returns an error.
func (s *Service) WaitForConnectedOrError(ctx context.Context) error {
	errorStr := ""
	pollErr := testing.Poll(ctx, func(ctx context.Context) error {
		props, err := s.GetShillProperties(ctx)
		if err != nil {
			return err
		}
		connected, err := props.Get(shillconst.ServicePropertyIsConnected)
		if err != nil {
			return err
		}
		if connected.(bool) {
			return nil
		}
		errorVal, err := props.Get(shillconst.ServicePropertyError)
		if err != nil {
			return err
		}
		errorStr = errorVal.(string)
		// Treat "no-failure" Error values as empty values.
		if errorStr == shillconst.ServiceErrorNoFailure {
			errorStr = ""
		}
		if errorStr != "" {
			return nil
		}
		return errors.New("not connected and no error")
	}, &testing.PollOptions{
		Timeout:  shillconst.DefaultTimeout,
		Interval: 100 * time.Millisecond,
	})
	if pollErr != nil {
		return pollErr
	}
	if errorStr != "" {
		return errors.New(errorStr)
	}
	return nil
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
