// Copyright 2022 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package network

import (
	"context"
	"time"

	"github.com/golang/protobuf/ptypes/empty"
	"github.com/google/go-cmp/cmp"
	"google.golang.org/protobuf/testing/protocmp"

	"chromiumos/tast/ctxutil"
	"chromiumos/tast/rpc"
	"chromiumos/tast/services/cros/network"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         ProxyRetainAfterReboot,
		LacrosStatus: testing.LacrosVariantUnneeded,
		Desc:         "Tests that the proxy values remain the same after DUT reboots",
		Contacts: []string{
			"lance.wang@cienet.com",
			"cienet-development@googlegroups.com",
			"chromeos-sw-engprod@google.com",
		},
		ServiceDeps:  []string{"tast.cros.network.ProxySettingService"},
		SoftwareDeps: []string{"chrome"},
		VarDeps:      []string{"ui.signinProfileTestExtensionManifestKey"},
		Timeout:      10 * time.Minute,
	})
}

// ProxyRetainAfterReboot tests that the proxy values remain the same after DUT reboots.
func ProxyRetainAfterReboot(ctx context.Context, s *testing.State) {
	var (
		dut         = s.DUT()
		rpcHint     = s.RPCHint()
		manifestKey = s.RequiredVar("ui.signinProfileTestExtensionManifestKey")
	)

	proxyConfigs := &network.ProxyConfigs{
		HttpHost:  "localhost",
		HttpPort:  "123",
		HttpsHost: "localhost",
		HttpsPort: "456",
		SocksHost: "socks5://localhost",
		SocksPort: "8080",
	}

	cleanupCtx := ctx
	ctx, cancel := ctxutil.Shorten(ctx, 10*time.Second)
	defer cancel()

	var proxyRebootClient network.ProxySettingServiceClient
	func() {
		client, err := rpc.Dial(ctx, dut, rpcHint)
		if err != nil {
			s.Fatal("Failed to connect to the RPC service on the DUT: ", err)
		}
		defer client.Close(cleanupCtx)

		proxyRebootClient = network.NewProxySettingServiceClient(client.Conn)

		if _, err := proxyRebootClient.New(ctx, &network.NewRequest{ManifestKey: manifestKey, ClearProxySettings: true}); err != nil {
			s.Fatal("Failed to create a new proxy setting service: ", err)
		}
		defer func(ctx context.Context) {
			proxyRebootClient.Close(ctx, &network.CloseRequest{Cleanup: s.HasError()})
		}(cleanupCtx)

		if _, err := proxyRebootClient.Setup(ctx, proxyConfigs); err != nil {
			s.Fatal("Failed to setup proxy: ", err)
		}
	}()

	if err := s.DUT().Reboot(ctx); err != nil {
		s.Fatal("Failed to reboot: ", err)
	}

	client, err := rpc.Dial(ctx, dut, rpcHint)
	if err != nil {
		s.Fatal("Failed to connect to the RPC service on the DUT: ", err)
	}
	defer client.Close(cleanupCtx)

	proxyRebootClient = network.NewProxySettingServiceClient(client.Conn)

	if _, err := proxyRebootClient.New(ctx, &network.NewRequest{ManifestKey: manifestKey, ClearProxySettings: false}); err != nil {
		s.Fatal("Failed to create a new proxy setting service: ", err)
	}
	defer proxyRebootClient.Close(cleanupCtx, &network.CloseRequest{Cleanup: true})

	returnedConfigs, err := proxyRebootClient.FetchConfigurations(ctx, &empty.Empty{})
	if err != nil {
		s.Fatal("Failed to setup proxy: ", err)
	}

	if diff := cmp.Diff(returnedConfigs, proxyConfigs, protocmp.Transform()); diff != "" {
		s.Fatalf("Unexpected proxy values (-want +got): %s", diff)
	}
}
