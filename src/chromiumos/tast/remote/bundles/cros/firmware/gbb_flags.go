// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package firmware

import (
	"context"
	"time"

	"github.com/golang/protobuf/ptypes/empty"

	common "chromiumos/tast/common/firmware"
	"chromiumos/tast/ctxutil"
	"chromiumos/tast/remote/firmware"
	"chromiumos/tast/remote/firmware/checkers"
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
		Attr:         []string{"group:firmware", "firmware_smoke"},
		SoftwareDeps: []string{"flashrom"},
	})
}

func GBBFlags(ctx context.Context, s *testing.State) {
	h := firmware.NewHelper(s.DUT(), s.RPCHint(), "", "")
	defer func() {
		if err := h.Close(ctx); err != nil {
			s.Log("Closing helper: ", err)
		}
	}()
	if err := h.RequireBiosServiceClient(ctx); err != nil {
		s.Fatal("Requiring BiosServiceClient: ", err)
	}

	bs := h.BiosServiceClient

	old, err := bs.GetGBBFlags(ctx, &empty.Empty{})
	if err != nil {
		s.Fatal("initial GetGBBFlags failed: ", err)
	}
	s.Log("Current GBB flags: ", old.Set)

	req := pb.GBBFlagsState{Set: common.GBBToggle(old.Set, pb.GBBFlag_DEV_SCREEN_SHORT_DELAY), Clear: common.GBBToggle(old.Clear, pb.GBBFlag_DEV_SCREEN_SHORT_DELAY)}

	if _, err = bs.ClearAndSetGBBFlags(ctx, &req); err != nil {
		s.Fatal("initial ClearAndSetGBBFlags failed: ", err)
	}
	ctxForCleanup := ctx
	// 150 seconds is a ballpark estimate, adjust as needed.
	ctx, cancel := ctxutil.Shorten(ctx, 150*time.Second)
	defer cancel()

	checker := checkers.New(h)
	defer func(ctx context.Context) {
		if _, err := bs.ClearAndSetGBBFlags(ctx, old); err != nil {
			s.Fatal("ClearAndSetGBBFlags to restore original values failed: ", err)
		}

		if err := checker.GBBFlags(ctx, *old); err != nil {
			s.Fatal("all flags should have been restored: ", err)
		}
	}(ctxForCleanup)

	if err := checker.GBBFlags(ctx, req); err != nil {
		s.Fatal("DEV_SCREEN_SHORT_DELAY flag should have been toggled: ", err)
	}
}
