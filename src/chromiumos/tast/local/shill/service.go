// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package shill

import (
	"context"

	"github.com/godbus/dbus"

	"chromiumos/tast/errors"
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
	ServicePropertyState          = "State"
	ServicePropertyStaticIPConfig = "StaticIPConfig"
	ServicePropertyVisible        = "Visible"

	// WiFi service property names.
	ServicePropertyPassphrase        = "Passphrase"
	ServicePropertySecurityClass     = "SecurityClass"
	ServicePropertySSID              = "SSID"
	ServicePropertyWiFiBSSID         = "WiFi.BSSID"
	ServicePropertyFTEnabled         = "WiFi.FTEnabled"
	ServicePropertyWiFiFrequency     = "WiFi.Frequency"
	ServicePropertyWiFiFrequencyList = "WiFi.FrequencyList"
	ServicePropertyWiFiHexSSID       = "WiFi.HexSSID"
	ServicePropertyWiFiHiddenSSID    = "WiFi.HiddenSSID"
	ServicePropertyWiFiPhyMode       = "WiFi.PhyMode"

	// EAP service property names.
	ServicePropertyEAPCACertPEM = "EAP.CACertPEM"
	ServicePropertyEAPMethod    = "EAP.EAP"
	ServicePropertyEAPInnerEAP  = "EAP.InnerEAP"
	ServicePropertyEAPIdentity  = "EAP.Identity"
	ServicePropertyEAPPassword  = "EAP.Password"
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

// ServiceConnectedStates is a list of service states that are considered connected.
var ServiceConnectedStates = []string{
	ServiceStatePortal,
	ServiceStateNoConnectivity,
	ServiceStateRedirectFound,
	ServiceStatePortalSuspected,
	ServiceStateOnline,
	ServiceStateReady,
}

// Security options defined in dbus-constants.h
const (
	SecurityWPA   = "wpa"
	SecurityWEP   = "wep"
	SecurityRSN   = "rsn"
	Security8021x = "802_1x"
	SecurityPSK   = "psk"
	SecurityNone  = "none"
)

// Service wraps a Service D-Bus object in shill.
type Service struct {
	PropertyHolder
}

// NewService connects to a service in Shill.
func NewService(ctx context.Context, path dbus.ObjectPath) (*Service, error) {
	ph, err := NewPropertyHolder(ctx, dbusServiceInterface, path)
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

// Remove calls the Remove method on the service.
func (s *Service) Remove(ctx context.Context) error {
	return s.dbusObject.Call(ctx, "Remove").Err
}
