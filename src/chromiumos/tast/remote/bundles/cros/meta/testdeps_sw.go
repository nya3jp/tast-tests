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
		Func: TestdepsSw,
		SoftwareDepsForAll: map[string][]string{
			"":    []string{"octopus"},
			"cd1": []string{"hana"},
		},
		Desc:     "Ensure DUTs dependencies with primary:octopus-sparky360 companion:hana-hana will pass, either or both changed will skip",
		Contacts: []string{"seewaifu@chromium.org", "yichiyan@google.com", "tast-owners@google.com"},
	})
}

func TestdepsSw(ctx context.Context, s *testing.State) {
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
