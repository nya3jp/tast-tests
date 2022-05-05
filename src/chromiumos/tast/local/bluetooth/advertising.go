// Copyright 2022 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package bluetooth

import (
	"context"

	"github.com/godbus/dbus"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/dbusutil"
)

// Advertising provides advertising-related controller capabilities.
type Advertising struct {
	obj  dbus.BusObject
	path dbus.ObjectPath
}

// Struct of SupportedCapabilities property in LEAdvertisingManager1 interface.
type supportedCapabilities struct {
	MaxAdvLen    uint8
	MaxScnRspLen uint8
	MaxTxPower   int16
	MinTxPower   int16
}

const advertisingsIface = service + ".LEAdvertisingManager1"

// Advertisings creates an Advertising for all bluetooth adapters in the system.
func Advertisings(ctx context.Context) ([]*Advertising, error) {
	var advertisings []*Advertising
	_, obj, err := dbusutil.Connect(ctx, service, "/")
	if err != nil {
		return nil, err
	}
	managed, err := dbusutil.ManagedObjects(ctx, obj)
	if err != nil {
		return nil, err
	}
	for _, path := range managed[advertisingsIface] {
		advertising, err := NewAdvertisings(ctx, path)
		if err != nil {
			return nil, err
		}
		advertisings = append(advertisings, advertising)
	}
	return advertisings, nil
}

// NewAdvertisings creates a new bluetooth Advertising from the passed D-Bus object path.
func NewAdvertisings(ctx context.Context, path dbus.ObjectPath) (*Advertising, error) {
	_, obj, err := dbusutil.Connect(ctx, service, path)
	if err != nil {
		return nil, err
	}
	return &Advertising{obj, path}, nil
}

// Path gets the D-Bus path this device was created from.
func (a *Advertising) Path() dbus.ObjectPath {
	return a.path
}

// SupportedCapabilities returns the supportedCapabilities of the adapter.
func (a *Advertising) SupportedCapabilities(ctx context.Context) (supportedCapabilities, error) {
	const prop = advertisingsIface + ".SupportedCapabilities"
	value, err := dbusutil.Property(ctx, a.obj, prop)
	output := supportedCapabilities{}
	if err != nil {
		return output, err
	}
	supportedCapabilities, ok := value.(map[string]dbus.Variant)
	if !ok {
		return output, errors.New("supportedCapabilities property not a string to dbus.Variant map")
	}
	if maxAdvLen, ok := supportedCapabilities["MaxAdvLen"].Value().(uint8); ok {
		output.MaxAdvLen = maxAdvLen
	} else {
		return output, errors.New("MaxAdvLen in supportedCapabilities property not a uint8")
	}

	if maxScnRspLen, ok := supportedCapabilities["MaxScnRspLen"].Value().(uint8); ok {
		output.MaxScnRspLen = maxScnRspLen
	} else {
		return output, errors.New("MaxScnRspLen in supportedCapabilities property not a uint8")
	}

	if maxTxPower, ok := supportedCapabilities["MaxTxPower"].Value().(int16); ok {
		output.MaxTxPower = maxTxPower
	} else {
		return output, errors.New("MaxTxPower in supportedCapabilities property not a int16")
	}

	if minTxPower, ok := supportedCapabilities["MinTxPower"].Value().(int16); ok {
		output.MinTxPower = minTxPower
	} else {
		return output, errors.New("MinTxPower in supportedCapabilities property not a int16")
	}

	return output, nil
}
