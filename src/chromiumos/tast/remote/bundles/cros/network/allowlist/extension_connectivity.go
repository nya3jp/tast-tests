// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package allowlist

import (
	"context"
	"time"

	"github.com/golang/protobuf/ptypes/empty"

	"chromiumos/tast/rpc"
	"chromiumos/tast/services/cros/network"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func: ExtensionConnectivity,
		Desc: "Test that extensions work behind a firewall configured according to our support page",
		Contacts: []string{
			"acostinas@google.com", // Test author
			"chromeos-commercial-networking@google.com",
		},
		Attr:         []string{"group:mainline", "informational"},
		Data:         []string{"allowlist_ssl_inspection.json"},
		SoftwareDeps: []string{"reboot", "chrome", "chrome_internal"},
		ServiceDeps:  []string{"tast.cros.network.AllowlistService"},
		Vars: []string{
			"allowlist.ext_username",
			"allowlist.ext_password",
		},
		Timeout: 12 * time.Minute,
	})
}

// ExtensionConnectivity calls the AllowlistService to setup a firewall and verifies that extensions can be installed.
func ExtensionConnectivity(ctx context.Context, s *testing.State) {
	defer func(ctx context.Context) {
		if err := s.DUT().Reboot(ctx); err != nil {
			s.Fatal("Failed to reboot DUT: ", err)
		}
	}(ctx)

	cl, err := rpc.Dial(ctx, s.DUT(), s.RPCHint(), "cros")
	if err != nil {
		s.Fatal("Failed to connect to the RPC service on the DUT: ", err)
	}
	defer cl.Close(ctx)

	al := network.NewAllowlistServiceClient(cl.Conn)

	if err := SetupFirewall(ctx, s.DataPath("allowlist_ssl_inspection.json"), false, true, &al); err != nil {
		s.Fatal("Failed setup firewall: ", err)
	}

	// Check login works
	user := s.RequiredVar("allowlist.ext_username")
	password := s.RequiredVar("allowlist.ext_password")

	if _, err := al.GaiaLogin(ctx, &network.GaiaLoginRequest{
		Username: user, Password: password}); err != nil {
		s.Fatal("Failed to login: ", err)
	}
	if _, err := al.TestExtensionConnectivity(ctx, &empty.Empty{}); err != nil {
		s.Fatal("Failed to install extension: ", err)
	}

}
