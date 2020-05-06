// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package firmware

import (
	"context"
	"strings"

	"github.com/golang/protobuf/ptypes/empty"

	"chromiumos/tast/remote/firmware"
	"chromiumos/tast/rpc"
	fwpb "chromiumos/tast/services/cros/firmware"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:        Config,
		Desc:        "Verifies that remote tests can load fw-testing-configs properly",
		Contacts:    []string{"chromeos-engprod@google.com"},
		Data:        firmware.ConfigDatafiles(),
		ServiceDeps: []string{"tast.cros.firmware.UtilsService"},
		Attr:        []string{"group:mainline", "informational"},
	})
}

func Config(ctx context.Context, s *testing.State) {
	// Connect to the gRPC server on the DUT.
	cl, err := rpc.Dial(ctx, s.DUT(), s.RPCHint(), "cros")
	if err != nil {
		s.Fatal("Failed to connect to the RPC: ", err)
	}
	defer cl.Close(ctx)

	utils := fwpb.NewUtilsServiceClient(cl.Conn)

	// Platform-specific behavior:
	// Verify that this DUT can load its config file by name.
	platformResponse, err := utils.Platform(ctx, &empty.Empty{})
	if err != nil {
		s.Fatal("Error during Platform: ", err)
	}
	platform := strings.ToLower(platformResponse.Platform)
	cfg, err := firmware.NewConfig(s.DataPath(firmware.ConfigDir), platform)
	if err != nil {
		s.Fatal("Error during NewConfig: ", err)
	}

	// Verify that the resulting config's "platform" attribute matches the platform (or variant) returned by RPC.
	platform = strings.Split(platform, "-")[0]
	platform = strings.Split(platform, "_")[strings.Count(platform, "_")]
	if cfg.Platform != platform {
		s.Errorf("Unexpected Platform value; got %s, want %s", cfg.Platform, platform)
	}
}
