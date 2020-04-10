// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package wificell

import (
	"context"
	"time"

	"github.com/golang/protobuf/ptypes/empty"

	"chromiumos/tast/testing"
	"chromiumos/tast/timing"
)

const (
	// VarRouter is the cmd variable name for router target.
	VarRouter = "router"
)

type testFixturePreImpl struct {
	tf *TestFixture
}

// String returns a short, underscore-separated name for the precondition.
func (p *testFixturePreImpl) String() string {
	return "wificell_test_fixture"
}

// Timeout returns the amount of time dedicated to prepare and close the precondition.
func (p *testFixturePreImpl) Timeout() time.Duration {
	return 15 * time.Second
}

// Prepare initialize the shared TestFixture if not yet created.
func (p *testFixturePreImpl) Prepare(ctx context.Context, s *testing.State) interface{} {
	ctx, st := timing.Start(ctx, p.String()+"_prepare")
	defer st.End()
	if p.tf == nil {
		// Create TestFixture.
		router, _ := s.Var(VarRouter)

		tf, err := NewTestFixture(ctx, s.DUT(), s.RPCHint(), router)
		if err != nil {
			s.Fatal("Failed to set up test fixture err: ", err)
		}
		p.tf = tf
	} else {
		// Properly re-init.
		if _, err := p.tf.WifiClient().InitTestState(ctx, &empty.Empty{}); err != nil {
			s.Fatal("Failed to InitTestState, err: ", err)
		}
	}
	return p.tf
}

// Close releases the resources occupied by TestFixture.
func (p *testFixturePreImpl) Close(ctx context.Context, s *testing.State) {
	ctx, st := timing.Start(ctx, p.String()+"_close")
	defer st.End()
	if p.tf == nil {
		return
	}
	if err := p.tf.Close(ctx); err != nil {
		s.Log("Failed to tear down test fixture, err: ", err)
	}
	p.tf = nil
}

// testFixturePre is the singleton of test fixture precondition.
var testFixturePre = &testFixturePreImpl{}

// TestFixturePre returns a precondition of wificell TestFixture.
func TestFixturePre() testing.Precondition {
	return testFixturePre
}
