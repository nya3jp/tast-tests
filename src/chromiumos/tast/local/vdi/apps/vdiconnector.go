// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package apps

import (
	"context"

	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/input"
	"chromiumos/tast/local/uidetection"
	"chromiumos/tast/testing"
)

// VDILoginConfig holds data necessary to login to a specific VDI application.
type VDILoginConfig struct {
	Server   string
	Username string
	Password string
}

// VdiInt is an interface for VDI application providing common way to connet
// to VDI application and other shared functionality.
type VdiInt interface {
	Init(s *testing.FixtState, d *uidetection.Context)
	Login(ctx context.Context, k *input.KeyboardEventWriter, cfg *VDILoginConfig) error
	EnsureMainScreenVisible(ctx context.Context) error
	SearchAndOpenApplication(ctx context.Context, k *input.KeyboardEventWriter, appName string) uiauto.Action
	ResetSearch(ctx context.Context, k *input.KeyboardEventWriter) error
}
