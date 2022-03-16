// Copyright 2022 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package install

import (
	"context"

	"chromiumos/tast/rpc"
	"chromiumos/tast/services/cros/install"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func: OsInstall,
		// TODO
		Desc: "TODO",
		Contacts: []string{
			"nicholasbishop@google.com",
		},
		// TODO
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"chrome"},
		ServiceDeps:  []string{"tast.cros.install.OsInstallService"},
		VarDeps:      []string{"ui.signinProfileTestExtensionManifestKey"},
	})
}

func OsInstall(ctx context.Context, s *testing.State) {
	dut := s.DUT()
	cl, err := rpc.Dial(ctx, dut, s.RPCHint())
	if err != nil {
		s.Fatal("Failed to connect to the RPC service on the DUT: ", err)
	}
	defer cl.Close(ctx)

	req := install.StartOsInstallRequest{
		SigninProfileTestExtensionID: s.RequiredVar("ui.signinProfileTestExtensionManifestKey"),
	}

	client := install.NewOsInstallServiceClient(cl.Conn)
	if _, err := client.StartOsInstall(ctx, &req); err != nil {
		s.Fatal("Failed to start OS install: ", err)
	}
}
