// Copyright 2019 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package shill

import (
	"context"
	"time"

	"github.com/godbus/dbus/v5"

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

// GetIPConfigs returns the IPConfig objects list of the associated Device. Note
// that this is not the IPConfig object of the Service.
func (s *Service) GetIPConfigs(ctx context.Context) ([]*IPConfig, error) {
	device, err := s.GetDevice(ctx)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to get associated Device for %s", s.ObjectPath())
	}
	props, err := device.GetProperties(ctx)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to get Device properties for %s", s.ObjectPath())
	}
	paths, err := props.GetObjectPaths(shillconst.DevicePropertyIPConfigs)
	var ret []*IPConfig
	for _, path := range paths {
		ipconfig, err := NewIPConfig(ctx, path)
		if err != nil {
			return nil, errors.Wrapf(err, "failed to create IPConfig from path %s", path)
		}
		ret = append(ret, ipconfig)
	}
	return ret, nil
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

// GetState returns the current service state.
func (s *Service) GetState(ctx context.Context) (string, error) {
	props, err := s.GetProperties(ctx)
	if err != nil {
		return "", errors.Wrap(err, "unable to get properties")
	}
	state, err := props.GetString(shillconst.ServicePropertyState)
	if err != nil {
		return "", errors.Wrap(err, "unable to get service state from properties")
	}
	return state, nil
}

// GetName returns the current service name.
func (s *Service) GetName(ctx context.Context) (string, error) {
	return s.getStringProperty(ctx, shillconst.ServicePropertyName)
}

// GetEid returns the current service EID (cellular only).
func (s *Service) GetEid(ctx context.Context) (string, error) {
	return s.getStringProperty(ctx, shillconst.ServicePropertyCellularEID)
}

// GetIccid returns the current service ICCID (cellular only).
func (s *Service) GetIccid(ctx context.Context) (string, error) {
	return s.getStringProperty(ctx, shillconst.ServicePropertyCellularICCID)
}

// GetSecurity returns the current service Security (WiFi only).
func (s *Service) GetSecurity(ctx context.Context) (string, error) {
	return s.getStringProperty(ctx, shillconst.ServicePropertySecurity)
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

// IsVisible returns true if the the service is visible.
func (s *Service) IsVisible(ctx context.Context) (bool, error) {
	props, err := s.GetProperties(ctx)
	if err != nil {
		return false, errors.Wrap(err, "unable to get properties")
	}
	visible, err := props.GetBool(shillconst.ServicePropertyVisible)
	if err != nil {
		return false, errors.Wrap(err, "unable to get IsVisible from properties")
	}
	return visible, nil
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

func (s *Service) getStringProperty(ctx context.Context, propertyName string) (string, error) {
	props, err := s.GetProperties(ctx)
	if err != nil {
		return "", errors.Wrap(err, "failed to read service properties")
	}
	value, err := props.GetString(propertyName)
	if err != nil {
		return "", errors.Wrapf(err, "failed to read service property %s", propertyName)
	}
	return value, nil
}
