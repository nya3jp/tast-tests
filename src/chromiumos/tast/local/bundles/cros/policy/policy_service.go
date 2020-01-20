// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package policy

import (
	"encoding/json"
	"io/ioutil"

	"github.com/golang/protobuf/ptypes/empty"
	"golang.org/x/net/context"
	"google.golang.org/grpc"

	"chromiumos/tast/common/policy/fakedms"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/bundles/cros/policy/url"
	"chromiumos/tast/local/chrome"
	ppb "chromiumos/tast/services/cros/policy"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddService(&testing.Service{
		Register: func(srv *grpc.Server, s *testing.ServiceState) {
			ppb.RegisterPolicyServiceServer(srv, &PolicyService{
				s:   s,
				hps: nil,
			})
		},
	})
}

// PolicyService implements tast.cros.policy.PolicyService.
type PolicyService struct { // NOLINT
	s *testing.ServiceState

	chrome  *chrome.Chrome
	fakeDMS *fakedms.FakeDMS

	hps *url.PolicyServer
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

func (c *PolicyService) StartURLPolicyServer(ctx context.Context, req *empty.Empty) (*empty.Empty, error) {
	if c.hps != nil {
		return nil, errors.New("URL Policy server already started")
	}

	hps, err := url.NewPolicyServer(ctx, 12345, "localhost")
	if err != nil {
		return nil, errors.Wrap(err, "failed to start policy HTTP server")
	}

	c.hps = hps

	return &empty.Empty{}, nil
}

func (c *PolicyService) StopURLPolicyServer(ctx context.Context, req *empty.Empty) (*empty.Empty, error) {
	if c.hps == nil {
		return nil, errors.New("URL Policy server not started")
	}

	if err := c.hps.Stop(ctx); err != nil {
		return nil, errors.Wrap(err, "failed to stop URL Policy server")
	}

	return &empty.Empty{}, nil
}

func (c *PolicyService) ServeURLPolicy(ctx context.Context, req *ppb.ServeURLPolicyRequest) (*ppb.ServeURLPolicyResponse, error) {
	if c.hps == nil {
		return nil, errors.New("URL Policy server not started")
	}

	hash, url, err := c.hps.ServePolicy(req.Contents)
	if err != nil {
		return nil, errors.Wrap(err, "failed to serve policy")
	}

	return &ppb.ServeURLPolicyResponse{
		Url:  url,
		Hash: hash,
	}, nil
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
