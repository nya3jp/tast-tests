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
	"chromiumos/tast/dut"
	"chromiumos/tast/errors"
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
	resources := &proxyRetainTestResources{
		dut:         s.DUT(),
		rpcHint:     s.RPCHint(),
		manifestKey: s.RequiredVar("ui.signinProfileTestExtensionManifestKey"),
		configs: &network.ProxyConfigs{
			HttpHost:  "localhost",
			HttpPort:  "123",
			HttpsHost: "localhost",
			HttpsPort: "456",
			SocksHost: "socks5://localhost",
			SocksPort: "8080",
		},
	}

	if _, err := proxySettingsHelper(ctx, resources, setupProxy); err != nil {
		s.Fatal("Failed to setup proxy: ", err)
	}

	if err := s.DUT().Reboot(ctx); err != nil {
		s.Fatal("Failed to reboot: ", err)
	}

	resultConfigs, err := proxySettingsHelper(ctx, resources, fetchProxy)
	if err != nil {
		s.Fatal("Failed to fetch proxy configs: ", err)
	}

	if diff := cmp.Diff(resultConfigs, resources.configs, protocmp.Transform()); diff != "" {
		s.Fatalf("Unexpected proxy values (-want +got): %s", diff)
	}
}

type proxyRetainTestResources struct {
	dut         *dut.DUT
	rpcHint     *testing.RPCHint
	manifestKey string
	configs     *network.ProxyConfigs
}

type proxySettingType int

const (
	setupProxy proxySettingType = iota
	fetchProxy
)

func proxySettingsHelper(ctx context.Context, resources *proxyRetainTestResources, proxySettingType proxySettingType) (_ *network.ProxyConfigs, retErr error) {
	cleanupCtx := ctx
	ctx, cancel := ctxutil.Shorten(ctx, 10*time.Second)
	defer cancel()

	client, err := rpc.Dial(ctx, resources.dut, resources.rpcHint)
	if err != nil {
		return nil, errors.Wrap(err, "failed to connect to the RPC service on the DUT")
	}
	defer client.Close(cleanupCtx)

	proxySettingsClient := network.NewProxySettingServiceClient(client.Conn)

	if _, err := proxySettingsClient.New(ctx, &network.NewRequest{ManifestKey: resources.manifestKey, ClearProxySettings: proxySettingType == setupProxy}); err != nil {
		return nil, errors.Wrap(err, "failed to create a new proxy setting service")
	}
	defer func(ctx context.Context) {
		cleanup := retErr != nil
		if proxySettingType == fetchProxy {
			// Cleanup the proxy configs after the works are all done.
			cleanup = true
		}
		proxySettingsClient.Close(ctx, &network.CloseRequest{Cleanup: cleanup})
	}(cleanupCtx)

	switch proxySettingType {
	case setupProxy:
		if _, err := proxySettingsClient.Setup(ctx, resources.configs); err != nil {
			return nil, errors.Wrap(err, "failed to setup proxy")
		}
		return nil, nil
	case fetchProxy:
		results, err := proxySettingsClient.FetchConfigurations(ctx, &empty.Empty{})
		if err != nil {
			return nil, errors.Wrap(err, "failed to setup proxy")
		}
		return results, nil
	default:
		return nil, errors.New("unrecognized proxy-setting-type")
	}
}
