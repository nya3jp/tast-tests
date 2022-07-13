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
	"chromiumos/tast/errors"
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
	redirectURL    = "http://www.example.com"
	httpsPortalURL = "https://www.example.com"
	sslCrtPath     = "/etc/ssl/certs"
	tmpCrtPath     = "/tmp/test_certs"
	tmpCrtFilePath = tmpCrtPath + "/cert.pem"
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

	if err := m.SetProperty(ctx, shillconst.ManagerPropertyPortalHTTPSURL, httpsPortalURL); err != nil {
		s.Fatal("Failed to set portal httpsurl: ", err)
	}
	cleanupCtx := ctx
	ctx, cancel := ctxutil.Shorten(ctx, 10*time.Second)
	defer cancel()

	params := s.Param().(*params)
	var certs certificate.CertStore
	if params.HTTPS {
		testing.ContextLog(ctx, "Installing temporary certs for TLS validation")
		certs = certificate.TestCert3()
		certCleanup, err := installCerts(ctx, certs.CACred.Cert)
		if err != nil {
			s.Fatal("Failed to set up temp certs: ", err)
		}
		defer certCleanup(cleanupCtx)
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

func installCerts(ctx context.Context, cert string) (func(context.Context), error) {
	if err := testexec.CommandContext(ctx, "mkdir", tmpCrtPath).Run(); err != nil {
		return nil, errors.Wrap(err, "failed to make tmp directory")
	}

	if err := os.WriteFile(tmpCrtFilePath, []byte(cert), 0644); err != nil {
		return nil, errors.Wrap(err, "failed to write file")
	}

	if err := testexec.CommandContext(ctx, "c_rehash", tmpCrtPath).Run(); err != nil {
		return nil, errors.Wrap(err, "failed to rehash")
	}

	if err := testexec.CommandContext(ctx, "mount", "-o", "bind", tmpCrtPath, sslCrtPath).Run(); err != nil {
		return nil, errors.Wrap(err, "failed to bind mount")
	}

	return func(cleanupCtx context.Context) {
		if err := testexec.CommandContext(cleanupCtx, "umount", sslCrtPath).Run(); err != nil {
			testing.ContextLog(ctx, "Failed to unmount bind: ", err)
		}
		if err := testexec.CommandContext(cleanupCtx, "rm", "-rf", tmpCrtPath).Run(); err != nil {
			testing.ContextLog(ctx, "Failed to delete tmp directory: ", err)
		}
	}, nil
}
