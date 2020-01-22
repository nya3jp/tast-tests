// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package pre

import (
	"context"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"
	"time"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/ash"
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

// UpdatePolicies updates the policies of FakeDMS and refreshes them in Chrome
func (h *UserPoliciesHelper) UpdatePolicies(ctx context.Context, pb *fakedms.PolicyBlob) error {
	if err := h.FakeDMS.Ping(ctx); err != nil {
		return errors.Wrap(err, "failed to ping FakeDMS")
	}

	if err := h.FakeDMS.WritePolicyBlob(pb); err != nil {
		return errors.Wrap(err, "failed to write policies to FakeDMS")
	}

	tconn, err := h.Chrome.TestAPIConn(ctx)
	if err != nil {
		return errors.Wrap(err, "failed to create Test API connection")
	}

	// Refresh policies.
	if err := tconn.EvalPromise(ctx, `tast.promisify(chrome.autotestPrivate.refreshEnterprisePolicies)();`, nil); err != nil {
		return errors.Wrap(err, "failed to refresh policies")
	}

	return nil
}

// Cleanup resets the state of Chrome and FakeDMS
func (h *UserPoliciesHelper) Cleanup(ctx context.Context) error {
	ctx, st := timing.Start(ctx, "user_policy_cleanup")
	defer st.End()

	if err := h.Chrome.ResetState(ctx); err != nil {
		return errors.Wrap(err, "failed to reset chrome")
	}

	if err := h.UpdatePolicies(ctx, fakedms.NewPolicyBlob()); err != nil {
		return errors.Wrap(err, "failed to clear policies")
	}

	tconn, err := h.Chrome.TestAPIConn(ctx)
	if err != nil {
		return errors.Wrap(err, "failed to create Test API connection")
	}

	windows, err := ash.GetAllWindows(ctx, tconn)
	if err != nil {
		return errors.Wrap(err, "failed to get windows")
	}

	for _, window := range windows {
		if err := window.CloseWindow(ctx, tconn); err != nil {
			return errors.Wrap(err, "failed to close window")
		}
	}

	return nil
}

// preImpl implements both testing.Precondition and testing.preconditionImpl.
type preImpl struct {
	helper        *UserPoliciesHelper
	fdmsDirectory string
}

// NewPrecondition creates a new precondition that can be shared by tests.
func NewPrecondition() *preImpl {
	return &preImpl{
		helper: nil,
	}
}

// Standard starts Chrome and FakeDMS and allows tests to update the policies
var Standard = NewPrecondition()

func (p *preImpl) String() string         { return "user_policy" }
func (p *preImpl) Timeout() time.Duration { return 60 * time.Second }

// Prepare is called by the test framework at the beginning of every test using this precondition.
// It returns a PreData containing the current state that can be used by the test.
func (p *preImpl) Prepare(ctx context.Context, s *testing.State) interface{} {
	if p.helper != nil {
		return p.helper
	}

	p.helper = &UserPoliciesHelper{}

	ctx, st := timing.Start(ctx, "user_policy_precondition_setup")
	defer st.End()

	// Start FakeDMS.
	tmpdir, err := ioutil.TempDir("", "fdms-")
	if err != nil {
		s.Fatal("Failed to create fmds temp dir: ", err)
	}

	testing.ContextLogf(ctx, "FakeDMS starting in %s", tmpdir)
	fmds, err := fakedms.New(ctx, tmpdir)
	if err != nil {
		s.Fatal("Failed to start FakeDMS: ", err)
	}

	p.helper.FakeDMS = fmds
	p.fdmsDirectory = tmpdir

	pb := fakedms.NewPolicyBlob()
	if err := p.helper.FakeDMS.WritePolicyBlob(pb); err != nil {
		s.Fatal("Failed to write policies to FakeDMS: ", err)
	}

	// Start a Chrome instance that will fetch policies from the FakeDMS.
	cr, err := chrome.New(ctx,
		chrome.Auth("tast-user@managedchrome.com", "test0000", "gaia-id"),
		chrome.DMSPolicy(fmds.URL))
	if err != nil {
		s.Fatal("Chrome login failed: ", err)
	}

	p.helper.Chrome = cr

	chrome.Lock()

	return p.helper
}

func (p *preImpl) copyFakeDMSLog(ctx context.Context, testDir string) error {
	source, err := os.Open(filepath.Join(p.fdmsDirectory, "fakedms.log"))
	if err != nil {
		return errors.Wrap(err, "failed to open FakeDMS log")
	}
	defer source.Close()

	destination, err := os.Create(filepath.Join(testDir, "fakedms.log"))
	if err != nil {
		return errors.Wrap(err, "failed to create FakeDMS log copy")
	}
	defer destination.Close()

	if count, err := io.Copy(destination, source); err != nil {
		return errors.Wrap(err, "failed to copy FakeDMS log data")
	} else if count == 0 {
		return errors.New("FakeDMS log empty")
	}

	return nil
}

// Close is called by the test framework after the last test that uses this precondition.
func (p *preImpl) Close(ctx context.Context, s *testing.State) {
	if p.helper == nil {
		return
	}

	ctx, st := timing.Start(ctx, "user_policy_precondition_cleanup")
	defer st.End()

	if p.helper.FakeDMS != nil {
		p.helper.FakeDMS.Stop(ctx)
	}

	if p.helper.Chrome != nil {
		chrome.Unlock()

		if err := p.helper.Chrome.Close(ctx); err != nil {
			s.Error("Failed to close Chdrome: ", err)
		}
	}

	if err := p.copyFakeDMSLog(ctx, s.OutDir()); err != nil {
		s.Error("Failed to copy FakeDMS log: ", err)
	}

	if err := os.RemoveAll(p.fdmsDirectory); err != nil {
		s.Error("Failed to remove FakeDMS directory: ", err)
	}

	p.helper = nil
}
