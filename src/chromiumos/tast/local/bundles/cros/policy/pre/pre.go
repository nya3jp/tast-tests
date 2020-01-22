// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package pre

import (
	"context"
	"io/ioutil"
	"os"
	"path/filepath"
	"time"

	"chromiumos/tast/fsutil"
	"chromiumos/tast/local/bundles/cros/policy/policyutil"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/policy"
	"chromiumos/tast/local/policy/fakedms"
	"chromiumos/tast/testing"
	"chromiumos/tast/timing"
)

// UserPoliciesHelper is returned by the precondition and used by tests to
// interact with Chrome and FakeDMS.
type UserPoliciesHelper struct {
	FakeDMS *fakedms.FakeDMS
	Chrome  *chrome.Chrome
}

// Cleanup is a helper that calls policyutil.ResetChrome
func (h *UserPoliciesHelper) Cleanup(ctx context.Context) error {
	ctx, st := timing.Start(ctx, "user_policy_cleanup")
	defer st.End()

	return policyutil.ResetChrome(ctx, h.FakeDMS, h.Chrome)
}

// ServeBlobAndRefresh is a helper that calls policyutil.ServeBlobAndRefresh
func (h *UserPoliciesHelper) ServeBlobAndRefresh(ctx context.Context, pb *fakedms.PolicyBlob) error {
	return policyutil.ServeBlobAndRefresh(ctx, h.FakeDMS, h.Chrome, pb)
}

// ServeAndRefresh is a helper that calls policyutil.ServeAndRefresh
func (h *UserPoliciesHelper) ServeAndRefresh(ctx context.Context, ps []policy.Policy) error {
	return policyutil.ServeAndRefresh(ctx, h.FakeDMS, h.Chrome, ps)
}

// preImpl implements both testing.Precondition and testing.preconditionImpl.
type preImpl struct {
	helper           *UserPoliciesHelper
	fdmsDirectory    string
	fakeDMSCtxCancel func()
}

// newPrecondition creates a new precondition that can be shared by tests.
func newPrecondition() *preImpl {
	return &preImpl{
		helper: nil,
	}
}

// User starts Chrome and FakeDMS and allows tests to update the policies.
var User = newPrecondition()

func (p *preImpl) String() string         { return "user_policy" }
func (p *preImpl) Timeout() time.Duration { return 60 * time.Second }

// Prepare is called by the test framework at the beginning of every test using this precondition.
// It returns a PreData containing the current state that can be used by the test.
func (p *preImpl) Prepare(ctx context.Context, s *testing.State) interface{} {
	if p.helper != nil {
		if err := p.helper.Cleanup(ctx); err == nil {
			return p.helper
		}

		// Cleanup failed; restart FakeDMS and Chrome for this test.
		p.helper.FakeDMS.Stop(ctx)
		chrome.Unlock()
		if err := p.helper.Chrome.Close(ctx); err != nil {
			s.Log("Failed to close Chrome: ", err)
		}
	}

	p.helper = &UserPoliciesHelper{}

	ctx, st := timing.Start(ctx, "user_policy_precondition_setup")
	defer st.End()

	// Start FakeDMS.
	tmpdir, err := ioutil.TempDir("", "fdms-")
	if err != nil {
		s.Fatal("Failed to create fmds temp dir: ", err)
	}

	// TODO(crbug.com/1046244): Remove this and use correct context.
	fakeDMSCtx, fakeDMSCtxCancel := context.WithCancel(context.Background()) // NOLINT

	testing.ContextLogf(ctx, "FakeDMS starting in %s", tmpdir)
	fdms, err := fakedms.New(fakeDMSCtx, tmpdir)
	if err != nil {
		s.Fatal("Failed to start FakeDMS: ", err)
	}

	p.helper.FakeDMS = fdms
	p.fakeDMSCtxCancel = fakeDMSCtxCancel
	p.fdmsDirectory = tmpdir

	pb := fakedms.NewPolicyBlob()
	if err := p.helper.FakeDMS.WritePolicyBlob(pb); err != nil {
		s.Fatal("Failed to write policies to FakeDMS: ", err)
	}

	// Start a Chrome instance that will fetch policies from the FakeDMS.
	cr, err := chrome.New(ctx,
		chrome.Auth("tast-user@managedchrome.com", "test0000", "gaia-id"),
		chrome.DMSPolicy(fdms.URL))
	if err != nil {
		s.Fatal("Chrome login failed: ", err)
	}

	p.helper.Chrome = cr

	chrome.Lock()

	return p.helper
}

func (p *preImpl) copyFakeDMSLog(ctx context.Context, testDir string) error {
	return fsutil.CopyFile(filepath.Join(p.fdmsDirectory, "fakedms.log"), filepath.Join(testDir, "fakedms.log"))
}

// Close is called by the test framework after the last test that uses this precondition.
func (p *preImpl) Close(ctx context.Context, s *testing.State) {
	if p.helper == nil {
		return
	}

	ctx, st := timing.Start(ctx, "user_policy_precondition_close")
	defer st.End()

	if p.helper.FakeDMS != nil {
		p.helper.FakeDMS.Stop(ctx)

		p.fakeDMSCtxCancel()
	}

	if p.helper.Chrome != nil {
		chrome.Unlock()

		if err := p.helper.Chrome.Close(ctx); err != nil {
			s.Error("Failed to close Chrome: ", err)
		}
	}

	// TODO(crbug.com/1049532): Copy log and policy.json for each test.
	if err := p.copyFakeDMSLog(ctx, s.OutDir()); err != nil {
		s.Error("Failed to copy FakeDMS log: ", err)
	}

	if err := os.RemoveAll(p.fdmsDirectory); err != nil {
		s.Error("Failed to remove FakeDMS directory: ", err)
	}

	p.helper = nil
}
