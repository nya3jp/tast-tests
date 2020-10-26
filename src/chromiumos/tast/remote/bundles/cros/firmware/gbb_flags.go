// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package firmware

import (
	"context"

	"github.com/golang/protobuf/ptypes/empty"

	"chromiumos/tast/rpc"
	pb "chromiumos/tast/services/cros/firmware"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:        GBBFlags,
		Desc:        "Verifies GBB flags state can be obtained from the DUT",
		Contacts:    []string{"cros-fw-engprod@google.com", "aluo@google.com"},
		ServiceDeps: []string{"tast.cros.firmware.BiosService"},
		Attr:        []string{"group:mainline", "informational"},
	})
}

func GBBFlags(ctx context.Context, s *testing.State) {
	cl, err := rpc.Dial(ctx, s.DUT(), s.RPCHint(), "cros")
	if err != nil {
		s.Fatal("Failed to connect to the RPC service on the DUT: ", err)
	}
	defer cl.Close(ctx)

	bs := pb.NewBiosServiceClient(cl.Conn)

	res, err := bs.GetGBBFlags(ctx, &empty.Empty{})
	if err != nil {
		s.Fatal("GetGBBFlags failed: ", err)
	}

	s.Log("Current GBB flags: ", res)
}
