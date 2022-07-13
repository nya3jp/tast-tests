// Copyright 2022 The ChromiumOS Authors.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package network

import (
	"context"
	"net/http"
	"time"

	"chromiumos/tast/common/crypto/certificate"
	"chromiumos/tast/common/pkcs11/netcertstore"
	"chromiumos/tast/common/shillconst"
	"chromiumos/tast/ctxutil"
	"chromiumos/tast/local/bundles/cros/network/virtualnet"
	"chromiumos/tast/local/bundles/cros/network/virtualnet/subnet"
	"chromiumos/tast/local/hwsec"
	"chromiumos/tast/local/shill"
	"chromiumos/tast/testing"
)

type params struct {
	ServiceState        string
	HTTPResponseHandler func(rw http.ResponseWriter, req *http.Request)
	HTTPSServer         bool
}

var (
	redirectURL = "http://www.foo.com"
)

func redirectHandler(url string) func(http.ResponseWriter, *http.Request) {
	return func(rw http.ResponseWriter, req *http.Request) {
		http.Redirect(rw, req, url, http.StatusFound)
	}
}

func redirectWithNoLocationHandler(rw http.ResponseWriter, req *http.Request) {
	rw.WriteHeader(http.StatusFound)
}

func okHandler(rw http.ResponseWriter, req *http.Request) {
	rw.WriteHeader(http.StatusNoContent)
}

func init() {
	testing.AddTest(&testing.Test{
		Func:     ShillCaptivePortalHTTP,
		Desc:     "Ensures that setting up a virtual ethernet pair with a DNS server that points portal detection queries to an http server that responds via the handler. This results in a service state of |ServiceState| via the params for the ethernet service",
		Contacts: []string{"michaelrygiel@google.com", "cros-network-health-team@google.com"},
		Attr:     []string{"group:mainline", "informational"},
		Fixture:  "shillReset",
		Params: []testing.Param{{
			Name: "redirectfound",
			Val: &params{
				ServiceState:        shillconst.ServiceStateRedirectFound,
				HTTPResponseHandler: redirectHandler(redirectURL),
				HTTPSServer:         false,
			},
		}, {
			Name: "portalsuspected",
			Val: &params{
				ServiceState:        shillconst.ServiceStatePortalSuspected,
				HTTPResponseHandler: redirectWithNoLocationHandler,
				HTTPSServer:         false,
			},
		}, {
			Name: "online",
			Val: &params{
				ServiceState:        shillconst.ServiceStateOnline,
				HTTPResponseHandler: okHandler,
				HTTPSServer:         true,
			},
		}},
	})
}

func ShillCaptivePortalHTTP(ctx context.Context, s *testing.State) {
	m, err := shill.NewManager(ctx)
	if err != nil {
		s.Fatal("Failed to create manager proxy: ", err)
	}

	if err := m.SetProperty(ctx, "PortalHttpsUrl", "thing.com"); err != nil {
		s.Fatal("Failed to set portal httpsurl: ", err)
	}

	testing.ContextLog(ctx, "Enabling portal detection on ethernet")
	// Relying on shillReset test fixture to undo the enabling of portal detection.
	if err := m.EnablePortalDetection(ctx); err != nil {
		s.Fatal("Enable Portal Detection failed: ", err)

	}

	params := s.Param().(*params)
	opts := virtualnet.EnvOptions{
		Priority:                  5,
		NameSuffix:                "",
		EnableDHCP:                true,
		RAServer:                  false,
		HTTPSServer:               params.HTTPSServer,
		HTTPServerResponseHandler: params.HTTPResponseHandler,
		AddressesToForceGateway:   []string{"www.gstatic.com", "www.google.com"},
	}
	pool := subnet.NewPool()
	service, portalEnv, err := virtualnet.CreateRouterEnv(ctx, m, pool, opts)
	if err != nil {
		s.Fatal("Failed to create a portal env: ", err)
	}
	cleanupCtx := ctx
	ctx, cancel := ctxutil.Shorten(ctx, 5*time.Second)
	defer cancel()
	defer portalEnv.Cleanup(cleanupCtx)

	if params.HTTPSServer {
		certs := certificate.TestCert1()
		runner := hwsec.NewCmdRunner()
		certStore, err := netcertstore.CreateStore(ctx, runner)
		if err != nil {
			s.Fatal("Failed to create store: ", err)
			return
		}
		_, err = certStore.InstallCertKeyPair(ctx, certs.ClientCred.PrivateKey, certs.ClientCred.Cert)
		if err != nil {
			s.Fatal("Failed to install cert key pair: ", err)
			return
		}
	}

	pw, err := service.CreateWatcher(ctx)
	if err != nil {
		s.Fatal("Failed to create watcher: ", err)
	}
	defer pw.Close(cleanupCtx)

	s.Log("Make service restart portal detector")
	if err := m.RecheckPortal(ctx); err != nil {
		s.Fatal("Failed to invoke RecheckPortal on shill: ", err)
	}

	timeoutCtx, cancel := context.WithTimeout(ctx, 35*time.Second)
	defer cancel()

	serviceProps, err := service.GetProperties(ctx)
	if err != nil {
		s.Fatal("Failed to get service properties")
	}

	s.Logf("Check if service state is %q", params.ServiceState)
	var expectedServiceState = []interface{}{
		params.ServiceState,
	}
	_, err = pw.ExpectIn(timeoutCtx, shillconst.ServicePropertyState, expectedServiceState)
	if err != nil {
		state, err := serviceProps.GetString(shillconst.ServicePropertyState)
		if err != nil {
			s.Fatal("Failed to get service state")
		}
		if state != params.ServiceState {
			s.Fatalf("Unexpected Service.State: %q", state)
		}
		s.Fatal("Service state is unexpected: ", err)
	}
}
