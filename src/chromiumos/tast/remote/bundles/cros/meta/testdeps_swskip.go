// Copyright 2022 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package meta

import (
	"context"

	"chromiumos/tast/rpc"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func: TestdepsSwskip,
		SoftwareDepsForAll: map[string][]string{
			"":    []string{"chrome"},
			"cd1": []string{"watchdog"},
		},
		Desc:     "Ensure DUTs software dependencies with Primary:octopus-sparky360 companion:eve-eve will skip",
		Contacts: []string{"seewaifu@chromium.org", "yichiyan@google.com", "tast-owners@google.com"},
	})
}

func TestdepsSwskip(ctx context.Context, s *testing.State) {
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
