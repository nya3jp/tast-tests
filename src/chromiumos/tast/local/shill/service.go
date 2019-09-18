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
	ServicePropertyState          ServiceProperty = "State"
	ServicePropertyStaticIPConfig ServiceProperty = "StaticIPConfig"
	ServicePropertySecurityClass  ServiceProperty = "SecurityClass"

	// WiFi service property names.
	ServicePropertyWiFiHiddenSSID ServiceProperty = "WiFi.HiddenSSID"
)

// ServiceConnectedStates is a list of service states that are considered connected.
var ServiceConnectedStates = []string{"portal", "no-connectivity", "redirect-found", "portal-suspected", "online", "ready"}

// Service wraps a Service D-Bus object in shill.
type Service struct {
	conn *dbus.Conn
	obj  dbus.BusObject
	path dbus.ObjectPath
}

// NewService connects to a service in Shill.
func NewService(ctx context.Context, path dbus.ObjectPath) (*Service, error) {
	conn, obj, err := dbusutil.Connect(ctx, dbusService, path)
	if err != nil {
		return nil, err
	}
	s := &Service{conn: conn, obj: obj, path: path}
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

// WaitForPropertyIn waits until a property is in a list of expected values
func (s *Service) WaitForPropertyIn(ctx context.Context, property ServiceProperty, expected interface{}, timeout time.Duration) error {
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	if t := reflect.TypeOf(expected); t.Kind() != reflect.Slice && t.Kind() != reflect.Array {
		return errors.New("expected values are not array or slice")
	}
	var expectedSlice []interface{}
	for i, val := 0, reflect.ValueOf(expected); i < val.Len(); i++ {
		expectedSlice = append(expectedSlice, val.Index(i).Interface())
	}

	spec := dbusutil.MatchSpec{
		Type:      "signal",
		Path:      s.path,
		Interface: dbusServiceInterface,
		Member:    "PropertyChanged",
	}
	sw, err := dbusutil.NewSignalWatcher(ctx, s.conn, spec)
	if err != nil {
		return err
	}
	defer sw.Close(ctx)

	props, err := s.GetProperties(ctx)
	if err != nil {
		return err
	}
	for _, v := range expectedSlice {
		if v == props[property] {
			return nil
		}
	}

	for {
		select {
		case sig := <-sw.Signals:
			if len(sig.Body) < 2 {
				continue
			}
			if foundProp, ok := sig.Body[0].(string); !ok || string(property) != foundProp {
				continue
			}
			foundVal, ok := sig.Body[1].(dbus.Variant)
			if !ok {
				continue
			}
			for _, v := range expectedSlice {
				if v == foundVal.Value() {
					return nil
				}
			}
		case <-ctx.Done():
			return ctx.Err()
		}
	}
}
