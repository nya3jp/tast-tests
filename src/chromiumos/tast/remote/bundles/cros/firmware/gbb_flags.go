// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package firmware

import (
	"context"
	"reflect"

	"github.com/golang/protobuf/ptypes/empty"

	"chromiumos/tast/rpc"
	pb "chromiumos/tast/services/cros/firmware"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:        GBBFlags,
		Desc:        "Verifies GBB flags state can be obtained and manipulated on the DUT",
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

	old, err := bs.GetGBBFlags(ctx, &empty.Empty{})

	if err != nil {
		s.Fatal("initial GetGBBFlags failed: ", old)
	}

	s.Log("Current GBB flags: ", old)

	req := pb.GBBFlagsState{Set: old.Clear, Clear: old.Set}
	_, err = bs.ClearSetGBBFlags(ctx, &req)

	if err != nil {
		s.Fatal("initial ClearSetGBBFlags failed: ", err)
	}

	res, err := bs.GetGBBFlags(ctx, &empty.Empty{})

	if err != nil {
		s.Fatal("GetGBBFlags after inital ClearSetGBBFlags failed: ", err)
	}

	if !reflect.DeepEqual(req.Set, res.Set) || !reflect.DeepEqual(req.Clear, res.Clear) {
		s.Fatalf("all flags should have been toggled, got %v, want %v", res, &req)
	}

	_, err = bs.ClearSetGBBFlags(ctx, old)

	if err != nil {
		s.Fatal("ClearSetGBBFlags to restore old values failed: ", err)
	}

	res, err = bs.GetGBBFlags(ctx, &empty.Empty{})

	if err != nil {
		s.Fatal("GetGBBFlags to verify flags have been restored failed: ", err)
	}

	if !reflect.DeepEqual(old.Set, res.Set) || !reflect.DeepEqual(old.Clear, res.Clear) {
		s.Fatalf("all flags should have been restored, got %v, want %v", res, &old)
	}
}
