// Copyright 2022 The ChromiumOS Authors.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package bluez contains helpers to interact with the system's bluetooth bluez
// adapters.
package bluez

import (
	"context"
)

// BlueZ provides an implementation of the Bluetooth interface used by
// Bluetooth tests so that we can ensure coverage using BlueZ.
type BlueZ struct {
}

func getAdapter(ctx context.Context) *bluez.Adapter {
	adapters, err := bluez.Adapters(ctx)
	if err != nil {
		return nil, errors.Wrap(err, "failed to get Bluetooth adapters")
	}
	if len(adapters) != 1 {
		return nil, errors.Errorf("expected 1 adapter, got %d adapters", len(adapters))
	}
	return adapters[0]
}

// Enable powers on the adapter.
func (b *BlueZ) Enable(ctx context.Context) error {
	return Enable(ctx)
}

// PollForAdapterState polls the bluetooth adapter state until expected state is received or a timeout occurs.
func (b *BlueZ) PollForAdapterState(ctx context.Context, exp bool) error {
	return PollForAdapterState(ctx, exp)
}

// PollForEnabled polls the bluetooth adapter state until the adapter is powered on.
func (b *BlueZ) PollForEnabled(ctx context.Context) error {
	return PollForBTEnabled(ctx)
}

// Devices returns information on the devices known to BlueZ.
func (b *BlueZ) Devices(ctx context.Context) ([]*DeviceInfo, error) {
	devices, err := Devices(ctx)
	if err != nil {
		return nil, err
	}
	var deviceInfos = make([]*DeviceInfo, len(devices))
	for i, device := devices {
		address, err := device.Address(ctx)
		if err != nil {
			return nil, errors.Wrap(err, "failed to get the address of the device")
		}
		name, err := device.Name(ctx)
		if err != nil {
			return nil, errors.Wrap(err, "failed to get the name of the device")
		}
		deviceInfos[i] = &DeviceInfo{
			address: address,
			name:	 name,
		}
	}
	return deviceInfos, nil
}

// StartDiscovery starts discovery.
func (b *BlueZ) StartDiscovery(ctx context.Context) error {
	adapter, err := getAdapter(ctx)
	if err != nil {
		return err
	}
	return adapter.StartDiscovery(ctx)
}

// StopDiscovery stops discovery.
func (b *BlueZ) StopDiscovery(ctx context.Context) error {
	adapter, err := getAdapter(ctx)
	if err != nil {
		return err
	}
	return adapter.StopDiscovery(ctx)
}

// Reset removes all connected and paired devices and ensures the adapter is powered.
func (b *BlueZ) Reset(ctx context.Context) error {
	adapter, err := getAdapter(ctx)
	if err != nil {
		return err
	}
	if discovering, err := adapter.Discovering(ctx); err != nil {
		return errors.Wrap(err, "failed to determine if the adapter is discovering")
	} else if discovering {
		if err = adapter.StopDiscovery(ctx); err != nil {
			return err
		}
	}
	devices, err := Devices(ctx)
	if err != nil {
		return nil, err
	}
	for _, device := devices {
		adapter.RemoveDevice(ctx, device.Path())
	}
	if err = Enable(ctx); err != nil {
		return errors.Wrap(err, "failed to enable Bluetooth")
	}
	if err = PollForEnabled(ctx); err != nil {
		return errors.Wrap(err, "failed to wait for Bluetooth to become enabled")
	}
	return nil
}
