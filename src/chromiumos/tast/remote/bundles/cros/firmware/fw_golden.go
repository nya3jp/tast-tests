// Copyright 2022 The ChromiumOS Authors.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package firmware

import (
	"context"

	"github.com/golang/protobuf/ptypes/empty"

	fwCommon "chromiumos/tast/common/firmware"
	"chromiumos/tast/remote/firmware"
	"chromiumos/tast/remote/firmware/fixture"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func: FWGolden,
		Desc: "Exercises critical functionality used to keep a well-known model healthy, i.e. the golden model",
		Contacts: []string{
			"kmshelton@chromium.org",     // Test author
			"cros-base-os-ep@google.com", // Backup mailing list
		},
		// TODO(kmshelton): Move to firmware_smoke after firmware_unstable verification
		Attr:        []string{"group:firmware", "firmware_unstable"},
		Fixture:     fixture.NormalMode,
		ServiceDeps: []string{"tast.cros.firmware.BiosService", "tast.cros.firmware.UtilsService"},
	})
}

func FWGolden(ctx context.Context, s *testing.State) {
	h := s.FixtValue().(*fixture.Value).Helper

	r := h.Reporter
	board, err := r.Board(ctx)
	if err != nil {
		s.Fatal("Failed to report board: ", err)
	}
	model, err := r.Model(ctx)
	if err != nil {
		s.Fatal("Failed to report model: ", err)
	}
	s.Logf("Reported board=%s, model=%s", board, model)

	if err := h.RequireBiosServiceClient(ctx); err != nil {
		s.Fatal("Requiring BiosServiceClient: ", err)
	}
	bs := h.BiosServiceClient
	flags, err := bs.GetGBBFlags(ctx, &empty.Empty{})
	if err != nil {
		s.Fatal("Failed to get GBB flags: ", err)
	}
	s.Log("Clear GBB flags: ", flags.Clear)
	s.Log("Set GBB flags:   ", flags.Set)

	if err := h.RequireRPCUtils(ctx); err != nil {
		s.Fatal("Requiring UtilsServiceClient: ", err)
	}
	us := h.RPCUtils
	_, err = us.BlockingSync(ctx, &empty.Empty{})
	if err != nil {
		s.Fatal("Failed to perform a blocking sync: ", err)
	}

	ms, err := firmware.NewModeSwitcher(ctx, h)
	if err != nil {
		s.Fatal("Creating mode switcher: ", err)
	}
	if err := h.RequireConfig(ctx); err != nil {
		s.Fatal("Requiring config")
	}
	s.Log("Mode switcher type: ", h.Config.ModeSwitcherType)
	if err := ms.RebootToMode(ctx, fwCommon.BootModeRecovery); err != nil {
		s.Fatal("Failed to boot to recovery mode: ", err)
	}
}
