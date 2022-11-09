// Copyright 2022 The ChromiumOS Authors
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

// PollForAdapterAvailable polls at least one bluetooth adapter is available.
func (b *BlueZ) PollForAdapterAvailable(ctx context.Context) error {
	return PollForAdapterAvailable(ctx)
}
