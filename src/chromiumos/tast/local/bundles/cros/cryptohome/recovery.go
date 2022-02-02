// Copyright 2022 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package cryptohome

import (
	"context"

	"chromiumos/tast/local/cryptohome"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:     Recovery,
		Desc:     "Checks that cryptohome recovery process succeeds with fake/local mediation",
		Contacts: []string{"anastasiian@chromium.org", "cros-lurs@google.com"},
		Attr:     []string{"group:mainline", "informational"},
	})
}

func Recovery(ctx context.Context, s *testing.State) {
	testTool, newErr := cryptohome.NewRecoveryTestTool()
	if newErr != nil {
		s.Fatal("Failed to initialize RecoveryTestTool", newErr)
	}
	defer removeDir(s, testTool)

	if err := testTool.CreateHsmPayload(ctx); err != nil {
		s.Fatal("Failed to execute CreateHsmPayload: ", err)
	}

	if err := testTool.CreateRecoveryRequest(ctx); err != nil {
		s.Fatal("Failed to execute CreateRecoveryRequest: ", err)
	}

	if err := testTool.FakeMediate(ctx); err != nil {
		s.Fatal("Failed to execute FakeMediate: ", err)
	}

	if err := testTool.Decrypt(ctx); err != nil {
		s.Fatal("Failed to execute Decrypt: ", err)
	}

	if err := testTool.Validate(ctx); err != nil {
		s.Fatal("Failed to validate: ", err)
	}
}

func removeDir(s *testing.State, testTool *cryptohome.RecoveryTestTool) {
	if err := testTool.RemoveDir(); err != nil {
		s.Fatal("Failed to remove dir: ", err)
	}
}
