// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package shill

import (
	"context"

	"github.com/godbus/dbus"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/dbusutil"
)

// ServiceProperty is the type for service property names.
type ServiceProperty string

const (
	dbusServiceInterface = "org.chromium.flimflam.Service"
)

// Service property names defined in dbus-constants.h .
const (
	// Service property names.
	ServicePropertyName           ServiceProperty = "Name"
	ServicePropertyType           ServiceProperty = "Type"
	ServicePropertyMode           ServiceProperty = "Mode"
	ServicePropertySSID           ServiceProperty = "SSID"
	ServicePropertyStaticIPConfig ServiceProperty = "StaticIPConfig"
	ServicePropertySecurityClass  ServiceProperty = "SecurityClass"

	// WiFi service property names.
	ServicePropertyWiFiHiddenSSID ServiceProperty = "WiFi.HiddenSSID"
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
func (s *Service) GetProperties(ctx context.Context) (map[ServiceProperty]interface{}, error) {
	props := make(map[ServiceProperty]interface{})
	if err := call(ctx, s.obj, dbusServiceInterface, "GetProperties").Store(&props); err != nil {
		return nil, errors.Wrap(err, "failed getting properties")
	}
	return props, nil
}

// SetProperty sets a string property to the given value
func (s *Service) SetProperty(ctx context.Context, property ServiceProperty, val interface{}) error {
	return call(ctx, s.obj, dbusServiceInterface, "SetProperty", property, val).Err
}
