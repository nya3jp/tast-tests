// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package firmware

import (
	"context"

	"chromiumos/tast/remote/firmware"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:        Config,
		Desc:        "Verifies that remote tests can load fw-testing-configs properly",
		Contacts:    []string{"cros-fw-engprod@google.com"},
		Data:        firmware.ConfigDatafiles(),
		ServiceDeps: []string{"tast.cros.firmware.UtilsService"},
		Attr:        []string{"group:mainline", "informational"},
	})
}

func Config(ctx context.Context, s *testing.State) {
	h := firmware.NewHelper(s.DUT(), s.RPCHint(), "")
	defer h.Close(ctx)

	h.ConfigDataDir = s.DataPath(firmware.ConfigDir)
	if err := h.RequireConfig(ctx); err != nil {
		s.Fatal("Failed to create firmware config: ", err)
	}

	// Verify that the loaded config's "platform" attribute matches the board (or board variant) returned by RPC.
	expectedPlatform := firmware.CfgPlatformFromLSBBoard(h.Board)
	if h.Config.Platform != expectedPlatform {
		s.Errorf("Unexpected Platform value; got %s, want %s", h.Config.Platform, expectedPlatform)
	}
}
