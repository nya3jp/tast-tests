// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package crostini

import (
	"context"

	"chromiumos/tast/rpc"
	"chromiumos/tast/services/cros/security"
	"chromiumos/tast/testing"
)

const adbSideloadingBootLockboxKey = "arc_sideloading_allowed"

func init() {
	testing.AddTest(&testing.Test{
		Func:         AdbSideloading,
		Desc:         "FIXME....Verifies that system comes back after rebooting",
		Contacts:     []string{"victorhsieh@chromium.org"},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"android_p", "tpm2", "reboot"},
		ServiceDeps:  []string{"tast.cros.security.BootLockboxService"},
	})
}

func AdbSideloading(ctx context.Context, s *testing.State) {
	// Connect to the gRPC server on the DUT.
	cl, err := rpc.Dial(ctx, s.DUT(), s.RPCHint(), "cros")
	if err != nil {
		s.Fatal("Failed to connect to the RPC service on the DUT: ", err)
	}
	defer cl.Close(ctx)

	client := security.NewBootLockboxServiceClient(cl.Conn)

	response, err := client.Read(ctx, &security.ReadBootLockboxRequest{adbSideloadingBootLockboxKey})
	if err != nil {
		s.Fatal("Failed to read from boot lockbox: ", err)
	}

	s.Logf("response value: %s",  string(response.Value))

	client.Store(ctx, &security.StoreBootLockboxRequest{adbSideloadingBootLockboxKey, []byte("1")})

	response, err = client.Read(ctx, &security.ReadBootLockboxRequest{adbSideloadingBootLockboxKey})
	if err != nil {
		s.Fatal("Failed to read from boot lockbox: ", err)
	}

	s.Logf("response value: %s",  string(response.Value))
}
