// Copyright 2022 The ChromiumOS Authors.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package floss provides a Floss implementation of the Bluetooth interface.
package floss

import (
	"context"
)

// Floss ...
type Floss struct {
}

// Enable ...
func (b *Floss) Enable(ctx context.Context) error {
	return nil
}

// PollForAdapterState ...
func (b *Floss) PollForAdapterState(ctx context.Context, exp bool) error {
	return nil
}

// PollForEnabled ...
func (b *Floss) PollForEnabled(ctx context.Context) error {
	return nil
}
