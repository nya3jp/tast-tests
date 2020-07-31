// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package chrome

import (
	"context"
	"time"

	"chromiumos/tast/errors"
	"chromiumos/tast/testing"
	"chromiumos/tast/timing"
)

func init() {
	testing.AddFixt(&testing.Fixt{
		Name:     "chrome_fixt",
		Impl:     NewFixt("chrome_fixt"),
		Timeout:  1 * time.Minute,
		Desc:     `Fixture to set up Chrome.`,
		Contacts: []string{"oka@chromium.org"},
	})
}

var loggedInFixt = &fixtImpl{}

// fixtImpl implements testing.Fixt.
type fixtImpl struct {
	cr   *Chrome
	name string
}

func NewFixt(name string) *fixtImpl {
	return &fixtImpl{
		name: name,
	}
}

// Prepare is called by the test framework at the beginning of every test using this fixtImpl.
// It returns a *chrome.Chrome that can be used by tests.
func (p *fixtImpl) Prepare(ctx context.Context, s *testing.FixtState) interface{} {
	ctx, st := timing.Start(ctx, "prepare_"+p.name)
	defer st.End()

	ctx, cancel := context.WithTimeout(ctx, LoginTimeout)
	defer cancel()

	cr, err := New(ctx)
	if err != nil {
		s.Fatal("Failed to strat Chrome: ", err)
	}
	Lock()

	return cr
}

func (p *fixtImpl) Adjust(ctx context.Context, s *testing.FixtTestState) error {
	ctx, cancel := context.WithTimeout(ctx, resetTimeout)
	defer cancel()
	ctx, st := timing.Start(ctx, "reset_"+p.name)
	defer st.End()
	if err := p.cr.Responded(ctx); err != nil {
		return errors.Wrap(err, "existing Chrome connection is unusable")
	}
	if err := p.cr.ResetState(ctx); err != nil {
		return errors.Wrap(err, "failed resetting existing Chrome session")
	}
	return nil
}

func (p *fixtImpl) PostTest(ctx context.Context, s *testing.FixtTestState) {
	// Do nothing
}

// Close is called by the test framework after the last test that uses this fixtImpl.
func (p *fixtImpl) Close(ctx context.Context, s *testing.FixtState) {
	Unlock()

	if p.cr == nil {
		return
	}
	if err := p.cr.Close(ctx); err != nil {
		s.Log("Failed to close Chrome connection: ", err)
	}
	p.cr = nil
}
