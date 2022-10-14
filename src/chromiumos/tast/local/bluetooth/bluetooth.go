// Copyright 2022 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package bluetooth

import (
	"context"
)

// Bluetooth defines the core interface used by Bluetooth tests to interact
// with the Bluetooth implementation in an implementation agnostic manner.
type Bluetooth interface {
	// Enable powers on the adapter.
	Enable(ctx context.Context) error

	// PollForAdapterState polls the bluetooth adapter state until expected
	// state is received or a timeout occurs.
	PollForAdapterState(ctx context.Context, exp bool) error

	// PollForEnabled polls the bluetooth adapter state until the adapter is
	// powered on.
	PollForEnabled(ctx context.Context) error

	// Devices returns information on the devices known to the Bluetooth
	// adapter.
	Devices(ctx context.Context) ([]*DeviceInfo, error)

	// StartDiscovery starts discovery.
	StartDiscovery(ctx context.Context) error

	// StopDiscovery stops discovery.
	StopDiscovery(ctx context.Context) error

	// Reset removes all connected and paired devices and ensures the adapter
	// is powered.
	Reset(ctx context.Context) error
}

// DeviceInfo defines a structure used by Bluetooth tests to retrieve
// information on devices that are known to the Bluetooth adapter.
type DeviceInfo struct {
	Address string
	Name    string
}
