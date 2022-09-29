// Copyright 2022 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package network

import (
	"context"
	"net/http"
	"time"

	"chromiumos/tast/common/crypto/certificate"
	"chromiumos/tast/common/shillconst"
	"chromiumos/tast/ctxutil"
	"chromiumos/tast/local/network/virtualnet"
	"chromiumos/tast/local/network/virtualnet/certs"
	"chromiumos/tast/local/network/virtualnet/subnet"
	"chromiumos/tast/local/shill"
	"chromiumos/tast/testing"
)

type params struct {
	serviceState         string
	httpResponseHandler  func(rw http.ResponseWriter, req *http.Request)
	httpsResponseHandler func(rw http.ResponseWriter, req *http.Request)
	proxyConfig          string
}

var (
	redirectURL     = "http://www.example.com"
	httpsPortalURL  = "https://www.example.com"
	testProxyConfig = "test proxy config"
)

func redirectHandler(url string) func(http.ResponseWriter, *http.Request) {
	return func(rw http.ResponseWriter, req *http.Request) {
		http.Redirect(rw, req, url, http.StatusFound)
	}
}

func redirectWithNoLocationHandler(rw http.ResponseWriter, req *http.Request) {
	rw.WriteHeader(http.StatusFound)
}

func noContentHandler(rw http.ResponseWriter, req *http.Request) {
	rw.WriteHeader(http.StatusNoContent)
}

func tempRedirectHandler(url string) func(http.ResponseWriter, *http.Request) {
	return func(rw http.ResponseWriter, req *http.Request) {
		http.Redirect(rw, req, url, http.StatusTemporaryRedirect)
	}
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
				serviceState:         shillconst.ServiceStateRedirectFound,
				httpResponseHandler:  redirectHandler(redirectURL),
				httpsResponseHandler: nil,
				proxyConfig:          "",
			},
		}, {
			Name: "proxyconfig",
			Val: &params{
				serviceState:         shillconst.ServiceStateOnline,
				httpResponseHandler:  redirectHandler(redirectURL),
				httpsResponseHandler: nil,
				proxyConfig:          testProxyConfig,
			},
		}, {
			Name: "portalsuspected",
			Val: &params{
				serviceState:         shillconst.ServiceStatePortalSuspected,
				httpResponseHandler:  redirectWithNoLocationHandler,
				httpsResponseHandler: nil,
				proxyConfig:          "",
			},
		}, {
			Name: "online",
			Val: &params{
				serviceState:         shillconst.ServiceStateOnline,
				httpResponseHandler:  noContentHandler,
				httpsResponseHandler: noContentHandler,
				proxyConfig:          "",
			},
		}, {
			Name: "noconnectivity",
			Val: &params{
				serviceState:         shillconst.ServiceStateNoConnectivity,
				httpResponseHandler:  nil,
				httpsResponseHandler: nil,
				proxyConfig:          "",
			},
		}, {
			Name: "redirectfoundtempredirect",
			Val: &params{
				serviceState:         shillconst.ServiceStateRedirectFound,
				httpResponseHandler:  tempRedirectHandler(redirectURL),
				httpsResponseHandler: nil,
				proxyConfig:          "",
			},
		}},
	})
}

func ShillCaptivePortalHTTP(ctx context.Context, s *testing.State) {
	m, err := shill.NewManager(ctx)
	if err != nil {
		s.Fatal("Failed to create manager proxy: ", err)
	}

	testing.ContextLog(ctx, "Enabling portal detection on ethernet")
	// Relying on shillReset test fixture to undo the enabling of portal detection.
	if err := m.EnablePortalDetection(ctx); err != nil {
		s.Fatal("Enable Portal Detection failed: ", err)
	}

	if err := m.SetProperty(ctx, shillconst.ManagerPropertyPortalHTTPSURL, httpsPortalURL); err != nil {
		s.Fatal("Failed to set portal httpsurl: ", err)
	}

	cleanupCtx := ctx
	ctx, cancel := ctxutil.Shorten(ctx, 10*time.Second)
	defer cancel()

	params := s.Param().(*params)

	var httpsCerts *certs.Certs
	if params.httpsResponseHandler != nil {
		httpsCerts = certs.New(certs.SSLCrtPath, certificate.TestCert3())
		cleanupCerts, err := httpsCerts.InstallTestCerts(ctx)
		if err != nil {
			s.Fatal("Failed to setup certificates: ", err)
		}
		defer cleanupCerts(cleanupCtx)
	}

	opts := virtualnet.EnvOptions{
		Priority:                   5,
		NameSuffix:                 "",
		EnableDHCP:                 true,
		EnableDNS:                  true,
		RAServer:                   false,
		HTTPSServerResponseHandler: params.httpsResponseHandler,
		HTTPServerResponseHandler:  params.httpResponseHandler,
		HTTPSCerts:                 httpsCerts,
	}
	pool := subnet.NewPool()
	service, portalEnv, err := virtualnet.CreateRouterEnv(ctx, m, pool, opts)
	if err != nil {
		s.Fatal("Failed to create a portal env: ", err)
	}
	defer portalEnv.Cleanup(cleanupCtx)

	pw, err := service.CreateWatcher(ctx)
	if err != nil {
		s.Fatal("Failed to create watcher: ", err)
	}
	defer pw.Close(cleanupCtx)

	//testing for ProxyConfig - no portal state to send in this case
	if params.proxyConfig != "" {
		if err := service.SetProperty(ctx, shillconst.ServicePropertyProxyConfig, params.proxyConfig); err != nil {
			s.Fatal("Failed to set ProxyConfig: ", err)
		}
	}

	s.Log("Make service restart portal detector")
	if err := m.RecheckPortal(ctx); err != nil {
		s.Fatal("Failed to invoke RecheckPortal on shill: ", err)
	}

	timeoutCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	s.Logf("Check if service state is %q", params.serviceState)
	var expectedServiceState = []interface{}{
		params.serviceState,
	}
	_, err = pw.ExpectIn(timeoutCtx, shillconst.ServicePropertyState, expectedServiceState)
	if err != nil {
		s.Fatal("Service state is unexpected: ", err)
	}
}
