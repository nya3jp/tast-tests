// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package wificell

import (
	"context"
	"time"

	"chromiumos/tast/testing"
	"chromiumos/tast/timing"
)

type testFixturePreImpl struct {
	name     string
	extraOps []TFOption

	tf *TestFixture
}

// String returns a short, underscore-separated name for the precondition.
func (p *testFixturePreImpl) String() string {
	return "wificell_test_fixture_" + p.name
}

// Timeout returns the amount of time dedicated to prepare and close the precondition.
func (p *testFixturePreImpl) Timeout() time.Duration {
	// When connecting to devices with slower SSH, e.g. in lab, the first
	// Prepare might take up to 30 seconds. Set a sufficient timeout for
	// the case here.
	return time.Minute
}

// Prepare initializes the shared TestFixture if not yet created and returns it
// as precondition value.
// Note that the test framework already reserves p.Timeout() for both Prepare
// and Close, so we don't have to call ReserveForClose. We just have to ensure
// Timeout is long enough.
func (p *testFixturePreImpl) Prepare(ctx context.Context, s *testing.PreState) interface{} {
	ctx, st := timing.Start(ctx, p.String()+"_prepare")
	defer st.End()

	if p.tf != nil {
		if err := p.tf.Reinit(ctx); err != nil {
			// Reinit failed.
			s.Log("Failed to re-initialize TestFixture, err: ", err)
			s.Log("Close current TestFixture and try to re-construct one")
			if err := p.tf.Close(ctx); err != nil {
				s.Log("Failed when closing broken TestFixture, err: ", err)
			}
			p.tf = nil
		} else {
			return p.tf
		}
	}

	// Create TestFixture.
	var ops []TFOption
	// Read router/pcap variable. If not available or empty, NewTestFixture
	// will fall back to Default{Router,Pcap}Host.
	if router, ok := s.Var("router"); ok && router != "" {
		ops = append(ops, TFRouter(router))
	}
	if pcap, ok := s.Var("pcap"); ok && pcap != "" {
		ops = append(ops, TFPcap(pcap))
	}
	ops = append(ops, p.extraOps...)
	tf, err := NewTestFixture(ctx, s.PreCtx(), s.DUT(), s.RPCHint(), ops...)
	if err != nil {
		s.Fatal("Failed to set up test fixture: ", err)
	}
	p.tf = tf

	return p.tf
}

// Close releases the resources occupied by the TestFixture.
func (p *testFixturePreImpl) Close(ctx context.Context, s *testing.PreState) {
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

// newTestFixturePre creates a testFixturePreImpl object to be used as Precondition.
func newTestFixturePre(name string, extraOps ...TFOption) *testFixturePreImpl {
	return &testFixturePreImpl{
		name:     name,
		extraOps: extraOps,
	}
}

// testFixturePre is the singleton of test fixture precondition.
var testFixturePre = newTestFixturePre("default")

// TestFixturePre returns a precondition of wificell TestFixture.
func TestFixturePre() testing.Precondition {
	return testFixturePre
}

// testFixturePreWithCapture is the singleton of test fixture precondition with
// TFCapture option.
var testFixturePreWithCapture = newTestFixturePre("capture", TFCapture(true))

// TestFixturePreWithCapture returns a precondition of wificell TestFixture with
// TFCapture(true) option.
func TestFixturePreWithCapture() testing.Precondition {
	return testFixturePreWithCapture
}
