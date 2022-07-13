// Copyright 2022 The ChromiumOS Authors.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package network

import (
	"context"
	"net/http"
	"os"
	"time"

	"chromiumos/tast/common/crypto/certificate"
	"chromiumos/tast/common/shillconst"
	"chromiumos/tast/common/testexec"
	"chromiumos/tast/ctxutil"
	"chromiumos/tast/local/bundles/cros/network/virtualnet"
	"chromiumos/tast/local/bundles/cros/network/virtualnet/subnet"
	"chromiumos/tast/local/shill"
	"chromiumos/tast/testing"
)

type params struct {
	ServiceState        string
	HTTPResponseHandler func(rw http.ResponseWriter, req *http.Request)
	HTTPS               bool
}

var (
	redirectURL = "http://www.example.com"
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
				HTTPS:               false,
			},
		}, {
			Name: "portalsuspected",
			Val: &params{
				ServiceState:        shillconst.ServiceStatePortalSuspected,
				HTTPResponseHandler: redirectWithNoLocationHandler,
				HTTPS:               false,
			},
		}, {
			Name: "online",
			Val: &params{
				ServiceState:        shillconst.ServiceStateOnline,
				HTTPResponseHandler: noContentHandler,
				HTTPS:               true,
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

	if err := m.SetProperty(ctx, "PortalHttpsUrl", "https://www.example.com"); err != nil {
		s.Fatal("Failed to set portal httpsurl: ", err)
	}
	cleanupCtx := ctx
	ctx, cancel := ctxutil.Shorten(ctx, 5*time.Second)
	defer cancel()

	params := s.Param().(*params)
	var certs certificate.CertStore
	if params.HTTPS {
		certs = certificate.TestCert3()

		// make temp directory
		if err := testexec.CommandContext(ctx, "mkdir", "/tmp/test_certs").Run(); err != nil {
			s.Fatal("Failed to make tmp directory: ", err)
		}
		// add cert file
		if err := os.WriteFile("/tmp/test_certs/cert.pem", []byte(certs.CACred.Cert), 0644); err != nil {
			s.Fatal("Failed to write file: ", err)
		}
		// c_rehash
		if err := testexec.CommandContext(ctx, "c_rehash", "/tmp/test_certs").Run(); err != nil {
			s.Fatal("Failed to rehash: ", err)
		}
		// bind mount temp directory to /etc/ssl/certs
		if err := testexec.CommandContext(ctx, "mount", "-o", "bind", "/tmp/test_certs", "/etc/ssl/certs").Run(); err != nil {
			s.Fatal("Failed to bind mount: ", err)
		}
		// defer function to umount and delete tmp folder
		defer func() {
			if err := testexec.CommandContext(cleanupCtx, "umount", "/etc/ssl/certs").Run(); err != nil {
				s.Fatal("Failed to unbind mount: ", err)
			}
			if err := testexec.CommandContext(cleanupCtx, "rm", "-rf", "/tmp/test_certs").Run(); err != nil {
				s.Fatal("Failed to unbind mount: ", err)
			}
		}()
	}
	opts := virtualnet.EnvOptions{
		Priority:                  5,
		NameSuffix:                "",
		EnableDHCP:                true,
		EnableDNS:                 true,
		RAServer:                  false,
		ServerCredentials:         &certs.ServerCred,
		HTTPServerResponseHandler: params.HTTPResponseHandler,
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

	timeoutCtx, cancel := context.WithTimeout(ctx, 35*time.Second)
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
