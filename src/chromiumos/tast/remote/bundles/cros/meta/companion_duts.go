// Copyright 2021 The Chromium OS Authors. All rights reserved.
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
		Func:     CompanionDUTs,
		Desc:     "Ensure DUT and companion DUTs are accessible",
		Contacts: []string{"seewaifu@chromium.org", "tast-owners@google.com"},
		Attr:     []string{"group:mainline", "informational"},
	})
}

// CompanionDUTs ensures DUT and companion DUTs are accessible in test.
func CompanionDUTs(ctx context.Context, s *testing.State) {
	cl, err := rpc.Dial(ctx, s.DUT(), s.RPCHint(), "cros")
	if err != nil {
		s.Fatal("Failed to connect to the RPC service on the DUT: ", err)
	}
	defer cl.Close(ctx)

	companionDUT := s.CompanionDUT("cd1")
	if companionDUT == nil {
		s.Fatal("Failed to get companion DUT cd1")
	}
	// TODO: enable following test in next CL in the chain.
	/*
		companionCl, err := rpc.Dial(ctx, companionDUT, s.RPCHint(), "cros")
		if err != nil {
			s.Fatal("Failed to connect to the RPC service on the companion DUT: ", err)
		}
		defer companionCl.Close(ctx)
	*/

}
