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

const (
	dbusServiceInterface = "org.chromium.flimflam.Service"
)

// Service property names defined in dbus-constants.h .
const (
	// Service property names.
	ServicePropertyDevice         = "Device"
	ServicePropertyName           = "Name"
	ServicePropertyType           = "Type"
	ServicePropertyIsConnected    = "IsConnected"
	ServicePropertyMode           = "Mode"
	ServicePropertySSID           = "SSID"
	ServicePropertyState          = "State"
	ServicePropertyStaticIPConfig = "StaticIPConfig"
	ServicePropertySecurityClass  = "SecurityClass"
	ServicePropertyPassphrase     = "Passphrase"

	// WiFi service property names.
	ServicePropertyWiFiHiddenSSID = "WiFi.HiddenSSID"
	ServicePropertyFtEnabled      = "WiFi.FTEnabled"
)

// Service state values defined in dbus-constants.h
const (
	ServiceStateIdle              = "idle"
	ServiceStateCarrier           = "carrier"
	ServiceStateAssociation       = "association"
	ServiceStateConfiguration     = "configuration"
	ServiceStateReady             = "ready"
	ServiceStatePortal            = "portal"
	ServiceStateNoConnectivity    = "no-connectivity"
	ServiceStateRedirectFound     = "redirect-found"
	ServiceStatePortalSuspected   = "portal-suspected"
	ServiceStateOffline           = "offline"
	ServiceStateOnline            = "online"
	ServiceStateDisconnect        = "disconnecting"
	ServiceStateFailure           = "failure"
	ServiceStateActivationFailure = "activation-failure"
)

// Service wraps a Service D-Bus object in shill.
type Service struct {
	dbusObject *DBusObject
	path       dbus.ObjectPath
	props      *Properties
}

// NewService connects to a service in Shill.
func NewService(ctx context.Context, path dbus.ObjectPath) (*Service, error) {
	conn, obj, err := dbusutil.Connect(ctx, dbusService, path)
	if err != nil {
		return nil, err
	}
	dbusObj := &DBusObject{iface: dbusServiceInterface, obj: obj, conn: conn}
	props, err := NewProperties(ctx, dbusObj)
	if err != nil {
		return nil, err
	}
	return &Service{dbusObject: dbusObj, path: path, props: props}, nil
}

// Properties returns existing properties.
func (s *Service) Properties() *Properties {
	return s.props
}

// String returns the path of the service.
// It is so named to conform to the Stringer interface.
func (s *Service) String() string {
	return s.dbusObject.String()
}

// GetProperties refreshes and returns properties.
func (s *Service) GetProperties(ctx context.Context) (*Properties, error) {
	props, err := NewProperties(ctx, s.dbusObject)
	if err != nil {
		return nil, err
	}
	s.props = props
	return props, nil
}

// SetProperty sets a property to the given value.
func (s *Service) SetProperty(ctx context.Context, property string, val interface{}) error {
	return s.props.SetProperty(ctx, property, val)
}

// GetDevice returns the Device object corresponding to the Service object
func (s *Service) GetDevice(ctx context.Context) (*Device, error) {
	serviceProps, err := s.GetProperties(ctx)
	if err != nil {
		return nil, err
	}
	deviceObjPath, err := serviceProps.GetObjectPath(ServicePropertyDevice)
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
	return s.dbusObject.Call(ctx, "Connect").Err
}

// Disconnect calls the Disconnect method on the service.
func (s *Service) Disconnect(ctx context.Context) error {
	return s.dbusObject.Call(ctx, "Disconnect").Err
}
