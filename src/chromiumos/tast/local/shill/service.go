// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package shill

import (
	"context"
	"reflect"
	"time"

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
	ServicePropertyName           = "Name"
	ServicePropertyType           = "Type"
	ServicePropertyMode           = "Mode"
	ServicePropertySSID           = "SSID"
	ServicePropertyState          = "State"
	ServicePropertyStaticIPConfig = "StaticIPConfig"
	ServicePropertySecurityClass  = "SecurityClass"

	// WiFi service property names.
	ServicePropertyWiFiHiddenSSID = "WiFi.HiddenSSID"
)

// ServiceConnectedStates is a list of service states that are considered connected.
var ServiceConnectedStates = []string{"portal", "no-connectivity", "redirect-found", "portal-suspected", "online", "ready"}

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
	dbusObj := &DBusObject{Interface: dbusServiceInterface, Object: obj, Conn: conn}
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
// It is so named to conforms the Stringer interface.
func (s *Service) String() string {
	return string(s.dbusObject.Object.Path())
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

// WaitForPropertyIn waits until a property is in a list of expected values
func (s *Service) WaitForPropertyIn(ctx context.Context, property string, expected interface{}, timeout time.Duration) error {
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	if t := reflect.TypeOf(expected); t.Kind() != reflect.Slice && t.Kind() != reflect.Array {
		return errors.New("expected values are not array or slice")
	}
	var expectedSlice []interface{}
	for i, val := 0, reflect.ValueOf(expected); i < val.Len(); i++ {
		expectedSlice = append(expectedSlice, val.Index(i).Interface())
	}

	currProps, err := s.GetProperties(ctx)
	if err != nil {
		return err
	}
	for _, expectVal := range expectedSlice {
		currV, err := currProps.Get(property)
		if err == nil && reflect.DeepEqual(currV, expectVal) {
			return nil
		}
	}

	pw, err := s.props.CreateWatcher(ctx)
	if err != nil {
		return err
	}
	defer pw.Close(ctx)

	for {
		err := pw.WaitAll(ctx, property)
		if err != nil {
			return err
		}
		for _, expected := range expectedSlice {
			if currV, err := s.props.Get(property); err == nil && expected == currV {
				return nil
			}
		}
	}
}
