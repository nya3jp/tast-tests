// Copyright 2022 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package network

import (
	"context"
	"net/http"
	"time"

	"chromiumos/tast/common/shillconst"
	"chromiumos/tast/ctxutil"
	"chromiumos/tast/local/network/virtualnet"
	"chromiumos/tast/local/network/virtualnet/subnet"
	"chromiumos/tast/local/shill"
	"chromiumos/tast/testing"
)

type params struct {
	ServiceState         string
	HTTPResponseHandler  func(rw http.ResponseWriter, req *http.Request)
	HTTPSResponseHandler func(rw http.ResponseWriter, req *http.Request)
	ProxyConfig          string
}

var (
	redirectURL    = "http://www.example.com"
	httpsPortalURL = "https://www.example.com"
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
				ServiceState:         shillconst.ServiceStateRedirectFound,
				HTTPResponseHandler:  redirectHandler(redirectURL),
				HTTPSResponseHandler: nil,
				ProxyConfig:          "",
			},
		}, {
			Name: "proxyconfigenabled",
			Val: &params{
				ServiceState:         shillconst.ServiceStateOnline,
				HTTPResponseHandler:  redirectHandler(redirectURL),
				HTTPSResponseHandler: nil,
				ProxyConfig:          "ProxyConfig",
			},
		}, {
			Name: "checkportal",
			Val: &params{
				ServiceState:         shillconst.ServiceStateOnline,
				HTTPResponseHandler:  redirectHandler(redirectURL),
				HTTPSResponseHandler: nil,
				ProxyConfig:          "ProxyConfig",
			},
		}, {
			Name: "portalsuspected",
			Val: &params{
				ServiceState:         shillconst.ServiceStatePortalSuspected,
				HTTPResponseHandler:  redirectWithNoLocationHandler,
				HTTPSResponseHandler: nil,
				ProxyConfig:          "",
			},
		}, {
			Name: "online",
			Val: &params{
				ServiceState:         shillconst.ServiceStateOnline,
				HTTPResponseHandler:  noContentHandler,
				HTTPSResponseHandler: noContentHandler,
				ProxyConfig:          "",
			},
		}, {
			Name: "noconnectivity",
			Val: &params{
				ServiceState:         shillconst.ServiceStateNoConnectivity,
				HTTPResponseHandler:  nil,
				HTTPSResponseHandler: nil,
				ProxyConfig:          "",
			},
		}, {
			Name: "redirectfoundtempredirect",
			Val: &params{
				ServiceState:         shillconst.ServiceStateRedirectFound,
				HTTPResponseHandler:  tempRedirectHandler(redirectURL),
				HTTPSResponseHandler: nil,
				ProxyConfig:          "",
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
	opts := virtualnet.EnvOptions{
		Priority:                   5,
		NameSuffix:                 "",
		EnableDHCP:                 true,
		EnableDNS:                  true,
		RAServer:                   false,
		HTTPSServerResponseHandler: params.HTTPSResponseHandler,
		HTTPServerResponseHandler:  params.HTTPResponseHandler,
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

	//testing for ProxyConfig or CheckPortal - no portal state to send in this case
	if params.ProxyConfig != nil {
		if err := service.SetProperty(ctx, shillconst.ServicePropertyProxyConfig, params.ProxyConfig); err != nil {
			s.Fatal("Portal detection disabled by ProxyConfig service: ", err)
		}
	}

	//	if err := m.SetProperty(ctx, shillconst.ServicePropertyCheckPortal, "test"); err != nil {
	//		s.Fatal("Portal detection disabled by CheckPortal service: ", err)
	//	}

	s.Log("Make service restart portal detector")
	if err := m.RecheckPortal(ctx); err != nil {
		s.Fatal("Failed to invoke RecheckPortal on shill: ", err)
	}

	timeoutCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	s.Logf("Check if service state is %q", params.ServiceState)
	var expectedServiceState = []interface{}{
		params.ServiceState,
	}
	_, err = pw.ExpectIn(timeoutCtx, shillconst.ServicePropertyState, expectedServiceState)
	if err != nil {
		s.Fatal("Service state is unexpected: ", err)
	}
}
