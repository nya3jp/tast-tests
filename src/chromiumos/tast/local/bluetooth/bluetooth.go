// Copyright 2022 The ChromiumOS Authors.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package bluetooth

import (
	"context"
)

// Bluetooth defines the core interface used by Bluetooth tests to interact
// with the Bluetooth implementation in an implementation agnostic manner.
type Bluetooth interface {
	Enable(ctx context.Context) error
	PollForAdapterState(ctx context.Context, exp bool) error
	PollForEnabled(ctx context.Context) error
}
