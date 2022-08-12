// Copyright 2022 The ChromiumOS Authors.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package bluetooth

import (
	"context"
)

// Bluetooth ...
type Bluetooth interface {
	Enable(ctx context.Context) error
	PollForAdapterState(ctx context.Context, exp bool) error
	PollForEnabled(ctx context.Context) error
}

// TestParams is needed since the parameters passed to Tast tests are checked
// to be the same type and the different implementations are recognized as
// different types.
type TestParams struct {
	Impl Bluetooth
}
