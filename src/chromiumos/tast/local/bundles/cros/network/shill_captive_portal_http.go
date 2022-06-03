// Copyright 2022 The ChromiumOS Authors.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package network

import (
	"context"
	"time"

	"chromiumos/tast/common/shillconst"
	"chromiumos/tast/ctxutil"
	"chromiumos/tast/local/bundles/cros/network/virtualnet"
	"chromiumos/tast/local/bundles/cros/network/virtualnet/subnet"
	"chromiumos/tast/local/shill"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:     ShillCaptivePortalHTTP,
		Desc:     "Ensures that setting up a virtual ethernet pair with a DNS server that points portal detection queries to an http server that responds with '302 Redirect' with a Redirect URL. This results in a service state of 'redirect-found' for the virtural ethernet service",
		Contacts: []string{"michaelrygiel@google.com", "cros-networking@google.com"},
		Attr:     []string{"group:mainline", "informational"},
		Fixture:  "shillReset",
	})
}

func ShillCaptivePortalHTTP(ctx context.Context, s *testing.State) {
	m, err := shill.NewManager(ctx)
	if err != nil {
		s.Fatal("Failed to create manager proxy: ", err)
	}

	testing.ContextLog(ctx, "Enabling portal detection on ethernet")
	// Relying on shillReset test fixture to undo the enabling of portal detection.
	_, err = m.EnablePortalDetectionOnTechnology(ctx, shillconst.TypeEthernet)
	if err != nil {
		s.Fatal("Enable Portal Detection failed: ", err)
	}

	opts := virtualnet.EnvOptions{
		Priority:              5,
		NameSuffix:            "",
		EnableDHCP:            true,
		RAServer:              false,
		HTTPServer:            true,
		AddressToForceGateway: "www.gstatic.com",
	}
	pool := subnet.NewPool()
	service, portalEnv, err := virtualnet.CreateRouterEnv(ctx, m, pool, opts)
	if err != nil {
		s.Fatal("Failed to create a portal env: ", err)
	}
	cleanupContext := ctx
	ctx, cancel := ctxutil.Shorten(ctx, 5*time.Second)
	defer cancel()
	defer portalEnv.Cleanup(cleanupContext)

	pw, err := service.CreateWatcher(ctx)
	if err != nil {
		s.Fatal("Failed to create watcher: ", err)
	}
	defer pw.Close(cleanupContext)
	s.Log("Make service restart portal detector")
	if err := m.RecheckPortal(ctx); err != nil {
		s.Fatal("Failed to invoke RecheckPortal on shill: ", err)
	}

	timeoutCtx, cancel := context.WithTimeout(ctx, 35*time.Second)
	defer cancel()
	s.Log("Check if service state is 'redirect-found'")
	var ServiceRedirectState = []interface{}{
		shillconst.ServiceStateRedirectFound,
	}
	_, err = pw.ExpectIn(timeoutCtx, shillconst.ServicePropertyState, ServiceRedirectState)
	if err != nil {
		s.Fatal("Service state is unexpected: ", err)
	}
}
