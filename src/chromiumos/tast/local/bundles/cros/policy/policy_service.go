// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package policy

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"

	"github.com/golang/protobuf/ptypes/empty"
	"golang.org/x/net/context"
	"google.golang.org/grpc"

	"chromiumos/tast/common/policy/fakedms"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/chrome"
	ppb "chromiumos/tast/services/cros/policy"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddService(&testing.Service{
		Register: func(srv *grpc.Server, s *testing.ServiceState) {
			ppb.RegisterPolicyServiceServer(srv, &PolicyService{
				s: s,

				urlPolicyServers: make(map[string]*URLPolicyServer),
				counter:          1,
			})
		},
	})
}

// ChromeService implements tast.cros.example.ChromeService.
type PolicyService struct {
	s *testing.ServiceState

	chrome  *chrome.Chrome
	fakeDMS *fakedms.FakeDMS

	urlPolicyServers map[string]*URLPolicyServer
	counter          int
}

func (c *PolicyService) EnrollUsingChrome(ctx context.Context, req *ppb.PolicyBlob) (*empty.Empty, error) {
	testing.ContextLogf(ctx, "Enrolling using Chrome with policy %s", string(req.PolicyBlob))

	tmpdir, err := ioutil.TempDir("", "fdms-")
	if err != nil {
		return nil, errors.Wrap(err, "failed to create temp dir")
	}

	testing.ContextLogf(ctx, "fakedms using dir %s", tmpdir)

	fdms, err := fakedms.New(ctx, tmpdir)
	if err != nil {
		return nil, errors.Wrap(err, "failed to start FakeDMS")
	}

	c.fakeDMS = fdms

	if err := fdms.WritePolicyBlobRaw(req.PolicyBlob); err != nil {
		return nil, errors.Wrap(err, "failed to write policy blob")
	}

	authOpt := chrome.Auth("tast-user@managedchrome.com", "test0000", "gaia-id")
	cr, err := chrome.New(ctx, authOpt, chrome.DMSPolicy(fdms.URL), chrome.EnterpriseEnroll())
	if err != nil {
		return nil, errors.Wrap(err, "failed to start chrome")
	}

	c.chrome = cr

	return &empty.Empty{}, nil
}

func (c *PolicyService) UpdatePolicies(ctx context.Context, req *ppb.PolicyBlob) (*empty.Empty, error) {
	// Write policies
	if err := c.fakeDMS.WritePolicyBlobRaw(req.PolicyBlob); err != nil {
		return nil, errors.Wrap(err, "failed to write policy blob")
	}

	tconn, err := c.chrome.TestAPIConn(ctx)
	if err != nil {
		return nil, errors.Wrap(err, "failed to create test API connection")
	}

	// Refresh policies.
	if err := tconn.EvalPromise(ctx, `tast.promisify(chrome.autotestPrivate.refreshEnterprisePolicies)();`, nil); err != nil {
		return nil, errors.Wrap(err, "failed to refresh policies")
	}

	return &empty.Empty{}, nil
}

func (c *PolicyService) CheckChrome(ctx context.Context, req *empty.Empty) (*empty.Empty, error) {
	tconn, err := c.chrome.TestAPIConn(ctx)
	if err != nil {
		return nil, errors.Wrap(err, "failed to create test API connection")
	}

	// Check Chrome
	result := false
	if err := tconn.Eval(ctx, "true", &result); err != nil {
		return nil, errors.Wrap(err, "failed to interact with Chrome")
	}
	if !result {
		return nil, errors.New("eval 'true' returned false")
	}

	// Check FakeDMS
	if err := c.fakeDMS.Ping(ctx); err != nil {
		return nil, errors.Wrap(err, "failed to ping FakeDMS")
	}

	return &empty.Empty{}, nil
}

func (c *PolicyService) StopChrome(ctx context.Context, req *empty.Empty) (*empty.Empty, error) {
	c.fakeDMS.Stop(ctx)

	if err := c.chrome.Close(ctx); err != nil {
		return nil, errors.Wrap(err, "failed to close chrome")
	}

	return &empty.Empty{}, nil
}

type URLPolicyServer struct {
	httpServer *http.Server
	data       []byte
	path       string
	url        string
}

func (ups *URLPolicyServer) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	w.Write(ups.data)
}

func (c *PolicyService) StartURLPolicyServer(ctx context.Context, req *ppb.StartURLPolicyServerRequest) (*ppb.StartURLPolicyServerResponse, error) {
	c.counter = c.counter + 1
	port := 12345 + c.counter

	srv := URLPolicyServer{
		data:       req.Contents,
		url:        fmt.Sprintf("http://localhost:%d/%d", port, c.counter),
		httpServer: &http.Server{Addr: fmt.Sprintf("localhost:%d", port)},
	}

	go func() {
		if err := srv.httpServer.ListenAndServe(); err != http.ErrServerClosed {
			testing.ContextLog(ctx, "URLPolicyServer HTTP server failed: ", err)
		}
	}()

	c.urlPolicyServers[srv.url] = &srv

	sum := sha256.Sum256(srv.data)

	res := ppb.StartURLPolicyServerResponse{
		Url:  srv.url,
		Hash: hex.EncodeToString(sum[:]),
	}

	return &res, nil
}

func (c *PolicyService) StopURLPolicyServer(ctx context.Context, req *ppb.StopURLPolicyServerRequest) (*empty.Empty, error) {
	srv, ok := c.urlPolicyServers[req.Url]

	if !ok {
		return nil, fmt.Errorf("could not find server for %s", req.Url)
	}

	if err := srv.httpServer.Shutdown(ctx); err != nil {
		return nil, errors.Wrap(err, "failed to shutdown HTTP server")
	}

	delete(c.urlPolicyServers, req.Url)

	return &empty.Empty{}, nil
}

func (c *PolicyService) EvalOnTestAPIConn(ctx context.Context, req *ppb.EvalOnTestAPIConnRequest) (*ppb.EvalOnTestAPIConnResponse, error) {
	tconn, err := c.chrome.TestAPIConn(ctx)
	if err != nil {
		return nil, errors.Wrap(err, "failed to create test API connection")
	}

	var result json.RawMessage
	if req.IsPromise {
		if err := tconn.EvalPromise(ctx, req.Expression, &result); err != nil {
			return nil, errors.Wrap(err, "failed to run javascript promise")
		}
	} else {
		if err := tconn.Eval(ctx, req.Expression, &result); err != nil {
			return nil, errors.Wrap(err, "failed to run javascript")
		}
	}

	encoded, err := json.Marshal(result)
	if err != nil {
		return nil, errors.Wrap(err, "failed to encode result")
	}

	return &ppb.EvalOnTestAPIConnResponse{
		Result: encoded,
	}, nil

}
