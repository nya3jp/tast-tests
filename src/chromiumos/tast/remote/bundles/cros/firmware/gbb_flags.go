// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package firmware

import (
	"context"
	"reflect"
	"time"

	"github.com/golang/protobuf/ptypes/empty"

	"chromiumos/tast/ctxutil"
	"chromiumos/tast/errors"
	"chromiumos/tast/rpc"
	pb "chromiumos/tast/services/cros/firmware"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         GBBFlags,
		Desc:         "Verifies GBB flags state can be obtained and manipulated on the DUT",
		Timeout:      8 * time.Minute,
		Contacts:     []string{"cros-fw-engprod@google.com", "aluo@google.com"},
		ServiceDeps:  []string{"tast.cros.firmware.BiosService"},
		Attr:         []string{"group:mainline", "informational", "group:firmware", "firmware_smoke"},
		SoftwareDeps: []string{"flashrom"},
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
		s.Fatal("initial GetGBBFlags failed: ", err)
	}
	s.Log("Current GBB flags: ", old)

	req := pb.GBBFlagsState{Set: old.Clear, Clear: old.Set}
	if _, err = bs.ClearAndSetGBBFlags(ctx, &req); err != nil {
		s.Fatal("initial ClearAndSetGBBFlags failed: ", err)
	}
	ctxForCleanup := ctx
	// 150 seconds is a ballpark estimate, adjust as needed.
	ctx, cancel := ctxutil.Shorten(ctx, 150*time.Second)
	defer cancel()
	defer func(ctx context.Context) {
		if _, err := bs.ClearAndSetGBBFlags(ctx, old); err != nil {
			s.Fatal("ClearAndSetGBBFlags to restore original values failed: ", err)
		}

		if err := checkGBBFlags(ctx, bs, *old); err != nil {
			s.Fatal("all flags should have been restored: ", err)
		}
	}(ctxForCleanup)

	if err := checkGBBFlags(ctx, bs, req); err != nil {
		s.Fatal("all flags should have been toggled: ", err)
	}
}

func checkGBBFlags(ctx context.Context, bs pb.BiosServiceClient, want pb.GBBFlagsState) error {
	if res, err := bs.GetGBBFlags(ctx, &empty.Empty{}); err != nil {
		return errors.Wrap(err, "could not get GBB flags")
	} else if !reflect.DeepEqual(want.Set, res.Set) || !reflect.DeepEqual(want.Clear, res.Clear) {
		return errors.Errorf("got %v, want %v", res, &want)
	}
	return nil
}
