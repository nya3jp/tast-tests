// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package apps

import (
	"context"
	"time"

	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/input"
	"chromiumos/tast/local/uidetection"
	"chromiumos/tast/testing"
)

// VDILoginTimeout is the timeout for login to VDI applications.
const VDILoginTimeout = time.Minute

// VDILoginConfig holds data necessary to login to a specific VDI application.
type VDILoginConfig struct {
	Server   string
	Username string
	Password string
}

// VDIInt is an interface for VDI application providing common way to connect
// to VDI application and other shared functionality.
type VDIInt interface {
	Init(s *testing.FixtState, d *uidetection.Context)
	Login(ctx context.Context, k *input.KeyboardEventWriter, cfg *VDILoginConfig) error
	WaitForMainScreenVisible(ctx context.Context) error
	SearchAndOpenApplication(ctx context.Context, k *input.KeyboardEventWriter, appName string) uiauto.Action
	ResetSearch(ctx context.Context, k *input.KeyboardEventWriter) error
}
