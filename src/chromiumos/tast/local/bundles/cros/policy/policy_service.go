// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package policy

import (
	"context"
	"encoding/json"
	"io/ioutil"
	"os"
	"path/filepath"

	"github.com/golang/protobuf/ptypes/empty"
	"google.golang.org/grpc"

	"chromiumos/tast/common/policy/fakedms"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/policyutil/externaldata"
	ppb "chromiumos/tast/services/cros/policy"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddService(&testing.Service{
		Register: func(srv *grpc.Server, s *testing.ServiceState) {
			ppb.RegisterPolicyServiceServer(srv, &PolicyService{
				s:              s,
				extensionConns: make(map[string]*chrome.Conn),
			})
		},
	})
}

// PolicyService implements tast.cros.policy.PolicyService.
type PolicyService struct { // NOLINT
	s *testing.ServiceState

	chrome         *chrome.Chrome
	extensionConns map[string]*chrome.Conn
	extensionDirs  []string
	fakeDMS        *fakedms.FakeDMS
	fakeDMSDir     string

	eds *externaldata.Server
}

// EnrollUsingChrome starts a FakeDMS insstance that serves the provided policies and
// enrolls the device. Specified user is logged in after this function completes.
func (c *PolicyService) EnrollUsingChrome(ctx context.Context, req *ppb.EnrollUsingChromeRequest) (*empty.Empty, error) {
	testing.ContextLogf(ctx, "Enrolling using Chrome with policy %s", string(req.PolicyJson))

	var opts []chrome.Option

	ok := false

	for _, extension := range req.Extensions {
		extDir, err := ioutil.TempDir("", "tast-extensions-")
		if err != nil {
			return nil, errors.Wrap(err, "failed to create temp dir")
		}
		defer func(ctx context.Context) {
			if !ok {
				if err := os.RemoveAll(extDir); err != nil {
					testing.ContextLogf(ctx, "Failed to delete %s: %v", extDir, err)
				}
			}
		}(ctx)

		c.extensionDirs = append(c.extensionDirs, extDir)

		for _, file := range extension.Files {
			if err := ioutil.WriteFile(filepath.Join(extDir, file.Name), file.Contents, 0644); err != nil {
				return nil, errors.Wrapf(err, "failed to write %s for %s", file.Name, extension.Id)
			}
		}

		if extID, err := chrome.ComputeExtensionID(extDir); err != nil {
			return nil, errors.Wrap(err, "failed to compute extension id")
		} else if extID != extension.Id {
			return nil, errors.Errorf("unexpected extension id: got %s; want %s", extID, extension.Id)
		}

		opts = append(opts, chrome.UnpackedExtension(extDir))
	}

	tmpdir, err := ioutil.TempDir("", "fdms-")
	if err != nil {
		return nil, errors.Wrap(err, "failed to create temp dir")
	}
	c.fakeDMSDir = tmpdir
	defer func() {
		if !ok {
			if err := os.RemoveAll(c.fakeDMSDir); err != nil {
				testing.ContextLogf(ctx, "Failed to delete %s: %v", c.fakeDMSDir, err)
			}
			c.fakeDMSDir = ""
		}
	}()

	// fakedms.New starts a background process that outlives the current context.
	fdms, err := fakedms.New(context.Background(), tmpdir) // NOLINT
	if err != nil {
		return nil, errors.Wrap(err, "failed to start FakeDMS")
	}
	c.fakeDMS = fdms
	defer func() {
		if !ok {
			c.fakeDMS.Stop(ctx)
			c.fakeDMS = nil
		}
	}()

	if err := fdms.WritePolicyBlobRaw(req.PolicyJson); err != nil {
		return nil, errors.Wrap(err, "failed to write policy blob")
	}

	user := req.Username
	if user == "" {
		user = "tast-user@managedchrome.com"
	}
	opts = append(opts, chrome.Auth(user, "test0000", "gaia-id"))
	opts = append(opts, chrome.DMSPolicy(fdms.URL))
	opts = append(opts, chrome.EnterpriseEnroll())

	cr, err := chrome.New(ctx, opts...)
	if err != nil {
		return nil, errors.Wrap(err, "failed to start chrome")
	}

	c.chrome = cr

	ok = true

	return &empty.Empty{}, nil
}

// UpdatePolicies updates the policies served by FakeDMS and refreshes them in Chrome.
func (c *PolicyService) UpdatePolicies(ctx context.Context, req *ppb.UpdatePoliciesRequest) (*empty.Empty, error) {
	if c.fakeDMS == nil {
		return nil, errors.New("fake DMS server not started")
	}

	// Write policies
	if err := c.fakeDMS.WritePolicyBlobRaw(req.PolicyJson); err != nil {
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

// CheckChromeAndFakeDMS verifies that Chrome and FakeDMS are still running and responsive.
func (c *PolicyService) CheckChromeAndFakeDMS(ctx context.Context, req *empty.Empty) (*empty.Empty, error) {
	if c.fakeDMS == nil {
		return nil, errors.New("fake DMS server not started")
	}

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

// StopChromeAndFakeDMS stops Chrome and FakeDMS.
func (c *PolicyService) StopChromeAndFakeDMS(ctx context.Context, req *empty.Empty) (*empty.Empty, error) {
	var lastErr error

	for id, conn := range c.extensionConns {
		if err := conn.Close(); err != nil {
			testing.ContextLogf(ctx, "Failed to close connection to extension %s", id)
			lastErr = errors.Wrapf(err, "failed to close connection to extension %s", id)
		}
	}

	if c.fakeDMS == nil {
		return nil, errors.New("fake DMS server not started")
	}

	c.fakeDMS.Stop(ctx)
	c.fakeDMS = nil

	if err := os.RemoveAll(c.fakeDMSDir); err != nil {
		testing.ContextLog(ctx, "Failed to remove temporary directory: ", err)
		lastErr = errors.Wrap(err, "failed to remove temporary directory")
	}

	if err := c.chrome.Close(ctx); err != nil {
		testing.ContextLog(ctx, "Failed to close chrome: ", err)
		lastErr = errors.Wrap(err, "failed to close chrome")
	}
	c.chrome = nil

	for _, extDir := range c.extensionDirs {
		if err := os.RemoveAll(extDir); err != nil {
			testing.ContextLog(ctx, "Failed to remove extension directory: ", err)
			lastErr = errors.Wrap(err, "failed to remove extension directory")
		}
	}

	return &empty.Empty{}, lastErr
}

// StartExternalDataServer starts  an instance of externaldata.Server.
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

// ServePolicyData serves the provided data and returns the url and hash that need to be providied to the policy.
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

// StopExternalDataServer stops the instance of externaldata.Server.
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

func (c *PolicyService) connToExtension(ctx context.Context, id string) (*chrome.Conn, error) {
	if val, ok := c.extensionConns[id]; ok {
		return val, nil
	}

	bgURL := chrome.ExtensionBackgroundPageURL(id)
	conn, err := c.chrome.NewConnForTarget(ctx, chrome.MatchTargetURL(bgURL))
	if err != nil {
		return nil, errors.Wrapf(err, "failed to connect to extension at %s", bgURL)
	}

	c.extensionConns[id] = conn

	return conn, nil
}

func (c *PolicyService) EvalStatementInExtension(ctx context.Context, req *ppb.EvalInExtensionRequest) (*empty.Empty, error) {
	conn, err := c.connToExtension(ctx, req.ExtensionId)
	if err != nil {
		return nil, errors.Wrap(err, "failed to create connection to extension")
	}

	if err := conn.Eval(ctx, req.Expression, nil); err != nil {
		return nil, errors.Wrap(err, "failed to run javascript")
	}

	return &empty.Empty{}, nil
}

func (c *PolicyService) EvalPromiseInExtension(ctx context.Context, req *ppb.EvalInExtensionRequest) (*ppb.EvalInExtensionResponse, error) {
	conn, err := c.connToExtension(ctx, req.ExtensionId)
	if err != nil {
		return nil, errors.Wrap(err, "failed to create connection to extension")
	}

	var result json.RawMessage
	if err := conn.EvalPromise(ctx, req.Expression, &result); err != nil {
		return nil, errors.Wrap(err, "failed to run javascript")
	}

	encoded, err := json.Marshal(result)
	if err != nil {
		return nil, errors.Wrap(err, "failed to encode result")
	}

	return &ppb.EvalInExtensionResponse{
		Result: encoded,
	}, nil
}
