// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package firmware

import (
	"context"
	"time"

	"chromiumos/tast/remote/firmware"
	"chromiumos/tast/testing"
)

type hasAPFlashOverCCD struct {
	helper *firmware.Helper
}

func init() {
	testing.AddTest(&testing.Test{
		Func:     ServoGBBFlags,
		Desc:     "Verifies GBB flags state can be obtained and manipulated via the servo interface",
		Timeout:  8 * time.Minute,
		Contacts: []string{"cros-fw-engprod@google.com", "jbettis@google.com"},
		Data:     []string{firmware.ConfigFile},
		Attr:     []string{"group:firmware", "firmware_experimental"},
		Vars:     []string{"servo"},
		Pre:      &hasAPFlashOverCCD{},
	})
}

func (p *hasAPFlashOverCCD) Prepare(ctx context.Context, s *testing.PreState) interface{} {
	p.helper = firmware.NewHelper(s.DUT(), s.RPCHint(), s.DataPath(firmware.ConfigFile), s.RequiredVar("servo"))
	if err := p.helper.RequireConfig(ctx); err != nil {
		s.Fatal("Failed to create firmware config: ", err)
	}
	if err := p.helper.RequireServo(ctx); err != nil {
		s.Fatal("Failed to connect to servo: ", err)
	}
	return p.helper
}

func (p *hasAPFlashOverCCD) Close(ctx context.Context, s *testing.PreState) {
	p.helper.Close(ctx)
}

func (p *hasAPFlashOverCCD) String() string {
	return "hasAPFlashOverCCD"
}

func (p *hasAPFlashOverCCD) Timeout() time.Duration {
	return 1 * time.Minute
}

func ServoGBBFlags(ctx context.Context, s *testing.State) {
	h := firmware.NewHelper(s.DUT(), s.RPCHint(), s.DataPath(firmware.ConfigFile), s.RequiredVar("servo"))
	defer h.Close(ctx)

	if err := h.RequireConfig(ctx); err != nil {
		s.Fatal("Failed to create firmware config: ", err)
	}

	if err := h.RequireServo(ctx); err != nil {
		s.Fatal("Failed to connect to servo: ", err)
	}

	if h.Config.ApFlashCCDProgrammer == "" {
		s.Fatal("DUT does not have ApFlashCCDProgrammer ", h.Config)
	}

	// old, err := bs.GetGBBFlags(ctx, &empty.Empty{})
	// if err != nil {
	// 	s.Fatal("initial GetGBBFlags failed: ", err)
	// }
	// s.Log("Current GBB flags: ", old)

	// req := pb.GBBFlagsState{Set: old.Clear, Clear: old.Set}
	// if _, err = bs.ClearAndSetGBBFlags(ctx, &req); err != nil {
	// 	s.Fatal("initial ClearAndSetGBBFlags failed: ", err)
	// }
	// ctxForCleanup := ctx
	// // 150 seconds is a ballpark estimate, adjust as needed.
	// ctx, cancel := ctxutil.Shorten(ctx, 150*time.Second)
	// defer cancel()

	// checker := checkers.New(h)
	// defer func(ctx context.Context) {
	// 	if _, err := bs.ClearAndSetGBBFlags(ctx, old); err != nil {
	// 		s.Fatal("ClearAndSetGBBFlags to restore original values failed: ", err)
	// 	}

	// 	if err := checker.GBBFlags(ctx, *old); err != nil {
	// 		s.Fatal("all flags should have been restored: ", err)
	// 	}
	// }(ctxForCleanup)

	// if err := checker.GBBFlags(ctx, req); err != nil {
	// 	s.Fatal("all flags should have been toggled: ", err)
	// }
}
