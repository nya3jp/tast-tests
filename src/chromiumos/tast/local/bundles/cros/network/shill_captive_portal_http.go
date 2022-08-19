// Copyright 2022 The ChromiumOS Authors.
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

func tempRedirectHandler(url string) func(http.ResponseWriter, *http.Request) {
	return func(rw http.ResponseWriter, req *http.Request) {
		http.Redirect(rw, req, url, http.StatusTemporaryRedirect)
	}
}

func redirectWithNoLocationHandler(rw http.ResponseWriter, req *http.Request) {
	rw.WriteHeader(http.StatusFound)
}

func tempRedirectWithNoLocationHandler(rw http.ResponseWriter, req *http.Request) {
	rw.WriteHeader(http.StatusTemporaryRedirect)
}

func noContentHandler(rw http.ResponseWriter, req *http.Request) {
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
				ServiceState:         shillconst.ServiceStateRedirectFound,
				HTTPResponseHandler:  redirectHandler(redirectURL),
				HTTPSResponseHandler: nil,
			},
		}, {
			Name: "portalsuspected",
			Val: &params{
				ServiceState:         shillconst.ServiceStatePortalSuspected,
				HTTPResponseHandler:  redirectWithNoLocationHandler,
				HTTPSResponseHandler: nil,
			},
		}, {
			Name: "online",
			Val: &params{
				ServiceState:         shillconst.ServiceStateOnline,
				HTTPResponseHandler:  noContentHandler,
				HTTPSResponseHandler: noContentHandler,
			},
		}, {
			Name: "noconnectivity",
			Val: &params{
				ServiceState:         shillconst.ServiceStateNoConnectivity,
				HTTPResponseHandler:  nil,
				HTTPSResponseHandler: nil,
			},
		}, {
			Name: "tempredirectfound",
			Val: &params{
				ServiceState:         shillconst.ServiceStateRedirectFound,
				HTTPResponseHandler:  tempRedirectHandler(redirectURL),
				HTTPSResponseHandler: nil,
			},
		}, {
			Name: "tempredirectnolocation",
			Val: &params{
				ServiceState:         shillconst.ServiceStatePortalSuspected,
				HTTPResponseHandler:  tempRedirectWithNoLocationHandler,
				HTTPSResponseHandler: nil,
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
