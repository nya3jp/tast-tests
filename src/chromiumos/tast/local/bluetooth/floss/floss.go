// Copyright 2022 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package floss provides a Floss implementation of the Bluetooth interface.
package floss

import (
	"context"
)

// Floss provides an implementation of the Bluetooth interface used by
// Bluetooth tests so that we can ensure coverage using Floss.
type Floss struct {
}

// Enable starts the default adapter.
func (f *Floss) Enable(ctx context.Context) error {
	return Enable(ctx)
}

// PollForAdapterState polls the bluetooth adapter state until expected state is received or a timeout occurs.
func (f *Floss) PollForAdapterState(ctx context.Context, exp bool) error {
	return PollForAdapterState(ctx, exp)
}

// PollForEnabled polls the bluetooth adapter state until the adapter is enabled.
func (f *Floss) PollForEnabled(ctx context.Context) error {
	return PollForEnabled(ctx)
}

// PollForAdapterAvailable polls at least one bluetooth adapter is available.
func (f *Floss) PollForAdapterAvailable(ctx context.Context) error {
	return PollForAdapterAvailable(ctx)
}
