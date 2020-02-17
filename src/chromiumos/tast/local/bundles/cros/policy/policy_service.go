// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package policy

import (
	"context"
	"encoding/json"
	"io/ioutil"
	"os"

	"github.com/golang/protobuf/ptypes/empty"
	"google.golang.org/grpc"

	"chromiumos/tast/common/policy/fakedms"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/policy/externaldata"
	ppb "chromiumos/tast/services/cros/policy"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddService(&testing.Service{
		Register: func(srv *grpc.Server, s *testing.ServiceState) {
			ppb.RegisterPolicyServiceServer(srv, &PolicyService{
				s:   s,
				eds: nil,
			})
		},
	})
}

// PolicyService implements tast.cros.policy.PolicyService.
type PolicyService struct { // NOLINT
	s *testing.ServiceState

	chrome     *chrome.Chrome
	fakeDMS    *fakedms.FakeDMS
	fakeDMSDir string

	eds *externaldata.Server
}

func (c *PolicyService) EnrollUsingChrome(ctx context.Context, req *ppb.PolicyBlob) (*empty.Empty, error) {
	testing.ContextLogf(ctx, "Enrolling using Chrome with policy %s", string(req.PolicyBlob))

	tmpdir, err := ioutil.TempDir("", "fdms-")
	if err != nil {
		return nil, errors.Wrap(err, "failed to create temp dir")
	}

	// fakedms.New starts a  background process that outlives the current context.
	fdms, err := fakedms.New(context.Background(), tmpdir) // NOLINT
	if err != nil {
		return nil, errors.Wrap(err, "failed to start FakeDMS")
	}

	c.fakeDMS = fdms
	c.fakeDMSDir = tmpdir

	if err := fdms.WritePolicyBlobRaw(req.PolicyBlob); err != nil {
		c.fakeDMS.Stop(ctx)
		c.fakeDMS = nil
		return nil, errors.Wrap(err, "failed to write policy blob")
	}

	authOpt := chrome.Auth("tast-user@managedchrome.com", "test0000", "gaia-id")
	cr, err := chrome.New(ctx, authOpt, chrome.DMSPolicy(fdms.URL), chrome.EnterpriseEnroll())
	if err != nil {
		c.fakeDMS.Stop(ctx)
		c.fakeDMS = nil
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
	if c.fakeDMS == nil {
		return nil, errors.New("fake DMS server not started")
	}

	c.fakeDMS.Stop(ctx)
	c.fakeDMS = nil

	if err := os.RemoveAll(c.fakeDMSDir); err != nil {
		return nil, errors.Wrap(err, "failed to remove temporary directory")
	}

	if err := c.chrome.Close(ctx); err != nil {
		return nil, errors.Wrap(err, "failed to close chrome")
	}
	c.chrome = nil

	return &empty.Empty{}, nil
}

func (c *PolicyService) StartExternalDataServer(ctx context.Context, req *empty.Empty) (*empty.Empty, error) {
	if c.eds != nil {
		return nil, errors.New("URL Policy server already started")
	}

	eds, err := externaldata.NewServer(ctx)
	if err != nil {
		return nil, errors.Wrap(err, "failed to start policy HTTP server")
	}

	c.eds = eds

	return &empty.Empty{}, nil
}

func (c *PolicyService) StopExternalDataServer(ctx context.Context, req *empty.Empty) (*empty.Empty, error) {
	if c.eds == nil {
		return nil, errors.New("URL Policy server not started")
	}

	if err := c.eds.Stop(ctx); err != nil {
		return nil, errors.Wrap(err, "failed to stop URL Policy server")
	}
	c.eds = nil

	return &empty.Empty{}, nil
}

func (c *PolicyService) ServePolicyData(ctx context.Context, req *ppb.ServePolicyDataRequest) (*ppb.ServePolicyDataResponse, error) {
	if c.eds == nil {
		return nil, errors.New("URL Policy server not started")
	}

	url, hash := c.eds.ServePolicyData(req.Contents)

	return &ppb.ServePolicyDataResponse{
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
	if err := tconn.Eval(ctx, req.Expression, &result); err != nil {
		return nil, errors.Wrap(err, "failed to run javascript")
	}

	encoded, err := json.Marshal(result)
	if err != nil {
		return nil, errors.Wrap(err, "failed to encode result")
	}

	return &ppb.EvalOnTestAPIConnResponse{
		Result: encoded,
	}, nil
}

func (c *PolicyService) EvalPromiseOnTestAPIConn(ctx context.Context, req *ppb.EvalOnTestAPIConnRequest) (*ppb.EvalOnTestAPIConnResponse, error) {
	tconn, err := c.chrome.TestAPIConn(ctx)
	if err != nil {
		return nil, errors.Wrap(err, "failed to create test API connection")
	}

	var result json.RawMessage
	if err := tconn.EvalPromise(ctx, req.Expression, &result); err != nil {
		return nil, errors.Wrap(err, "failed to run javascript")
	}

	encoded, err := json.Marshal(result)
	if err != nil {
		return nil, errors.Wrap(err, "failed to encode result")
	}

	return &ppb.EvalOnTestAPIConnResponse{
		Result: encoded,
	}, nil
}
