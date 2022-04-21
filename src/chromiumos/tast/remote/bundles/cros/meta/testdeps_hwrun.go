// Copyright 2022 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package meta

import (
	"context"

	"chromiumos/tast/rpc"
	"chromiumos/tast/testing"
	"chromiumos/tast/testing/hwdep"
)

func init() {
	testing.AddTest(&testing.Test{
		Func: TestdepsHwrun,
		HardwareDepsForAll: map[string]hwdep.Deps{
			"":    hwdep.D(hwdep.InternalDisplay()),
			"cd1": hwdep.D(hwdep.InternalDisplay()),
		},
		Desc:     "Ensure DUTs hardware dependencies with Primary:octopus-sparky360 companion:hana-hana will pass",
		Contacts: []string{"seewaifu@chromium.org", "yichiyan@google.com", "tast-owners@google.com"},
	})
}

func TestdepsHwrun(ctx context.Context, s *testing.State) {
	cl, err := rpc.Dial(ctx, s.DUT(), s.RPCHint())
	if err != nil {
		s.Fatal("Failed to connect to the RPC service on the DUT: ", err)
	}
	defer cl.Close(ctx)

	companionDUT := s.CompanionDUT("cd1")
	if companionDUT == nil {
		s.Fatal("Failed to get companion DUT cd1")
	}
	companionCl, err := rpc.Dial(ctx, companionDUT, s.RPCHint())
	if err != nil {
		s.Fatal("Failed to connect to the RPC service on the companion DUT: ", err)
	}
	defer companionCl.Close(ctx)
}
