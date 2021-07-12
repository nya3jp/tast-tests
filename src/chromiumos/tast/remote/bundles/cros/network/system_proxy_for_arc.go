// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package network

import (
	"context"
	"encoding/json"
	"fmt"
	"regexp"
	"time"

	"github.com/golang/protobuf/ptypes/empty"

	"chromiumos/tast/common/policy"
	"chromiumos/tast/common/policy/fakedms"
	"chromiumos/tast/ctxutil"
	"chromiumos/tast/remote/policyutil"
	"chromiumos/tast/rpc"
	"chromiumos/tast/services/cros/network"
	ps "chromiumos/tast/services/cros/policy"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func: SystemProxyForArc,
		Desc: "Test that ARC++ apps can successfully connect to the remote host through the system-proxy daemon",
		Contacts: []string{
			"acostinas@google.com", // Test author
			"chromeos-commercial-networking@google.com",
			"hugobenichi@google.com",
		},
		Attr:         []string{"group:enrollment"},
		SoftwareDeps: []string{"reboot", "vm_host", "chrome"},
		ServiceDeps:  []string{"tast.cros.policy.PolicyService", "tast.cros.network.ProxyService"},
		Timeout:      12 * time.Minute,
		Params: []testing.Param{{
			ExtraSoftwareDeps: []string{"android_p"},
		}, {
			Name:              "vm",
			ExtraSoftwareDeps: []string{"android_vm"},
		}},
	})
}

func SystemProxyForArc(ctx context.Context, s *testing.State) {
	defer func(ctx context.Context) {
		if err := policyutil.EnsureTPMAndSystemStateAreReset(ctx, s.DUT()); err != nil {
			s.Fatal("Failed to reset TPM: ", err)
		}
	}(ctx)

	ctx, cancel := ctxutil.Shorten(ctx, 3*time.Minute)
	defer cancel()

	if err := policyutil.EnsureTPMAndSystemStateAreReset(ctx, s.DUT()); err != nil {
		s.Fatal("Failed to reset TPM: ", err)
	}

	cl, err := rpc.Dial(ctx, s.DUT(), s.RPCHint(), "cros")
	if err != nil {
		s.Fatal("Failed to connect to the RPC service on the DUT: ", err)
	}
	defer cl.Close(ctx)

	// Start an HTTP proxy instance on the DUT which requires username and password authentication.
	proxyClient := network.NewProxyServiceClient(cl.Conn)
	response, err := proxyClient.StartServer(ctx,
		&network.StartServerRequest{
			Port:            3129,
			AuthCredentials: &network.AuthCredentials{Username: "testuser", Password: "testpwd"},
		})
	if err != nil {
		s.Fatal("Failed to start a local proxy on the DUT: ", err)
	}

	pb := fakedms.NewPolicyBlob()
	//Configure the proxy on the DUT via policy to point to the local proxy instance started via the `ProxyService`.
	pb.AddPolicy(&policy.ProxySettings{
		Val: &policy.ProxySettingsValue{
			ProxyMode:       "fixed_servers",
			ProxyServer:     fmt.Sprintf("http://%s", response.HostAndPort),
			ProxyBypassList: "",
		}})
	// Start system-proxy.
	pb.AddPolicy(&policy.SystemProxySettings{
		Val: &policy.SystemProxySettingsValue{
			SystemProxyEnabled:           true,
			SystemServicesUsername:       "",
			SystemServicesPassword:       "",
			PolicyCredentialsAuthSchemes: []string{},
		}})
	pb.AddPolicy(&policy.ArcEnabled{Val: true})

	pJSON, err := json.Marshal(pb)
	if err != nil {
		s.Fatal("Failed to serialize policies: ", err)
	}

	pc := ps.NewPolicyServiceClient(cl.Conn)
	if _, err := pc.EnrollUsingChrome(ctx, &ps.EnrollUsingChromeRequest{
		PolicyJson: pJSON,
		ExtraArgs:  "--arc-availability=officially-supported",
	}); err != nil {
		s.Fatal("Failed to enroll using chrome: ", err)
	}
	defer pc.StopChromeAndFakeDMS(ctx, &empty.Empty{})

	if _, err := pc.CreateArcInstance(ctx, &empty.Empty{}); err != nil {
		s.Fatal("Failed to start ARC: ", err)
	}
	rez, err := pc.VerifyArcAppConnectivity(ctx,
		&ps.VerifyArcAppConnectivityRequest{
			Url:            "https://www.google.com/",
			UseSystemProxy: true,
			ProxyUsername:  "testuser",
			ProxyPassword:  "testpwd",
		})
	if err != nil {
		s.Fatal("Failed to test ARC app connectivity: ", err)
	}
	// System-proxy has an address in the 100.115.92.0/24 subnet (assigned by patchpanel) and listens on port 3128.
	expectedProxy := regexp.MustCompile("100.115.92.[0-9]+:3128")
	if !expectedProxy.Match([]byte(rez.Proxy)) {
		s.Fatalf("The ARC++ app is not using the system-proxy daemon: %s", rez.Proxy)
	}
}
