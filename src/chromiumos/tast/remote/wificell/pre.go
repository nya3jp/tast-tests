// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package wificell

import (
	"context"
	"strings"
	"time"

	"chromiumos/tast/testing"
	"chromiumos/tast/timing"
)

// ExtraFeatures is enum for any extra features needed for precondition.
type ExtraFeatures uint8

const (
	multiRouters ExtraFeatures = 1 << iota
	attenuatorPresent
)

type testFixturePreImpl struct {
	name     string
	extraOps []TFOption
	features ExtraFeatures
	tf       *TestFixture
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
			s.Log("Try recreating the TestFixture as it failed to re-initialize, err: ", err)
			if err := p.tf.Close(ctx); err != nil {
				s.Log("Failed to close the broken TestFixture, err: ", err)
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
	if p.features&multiRouters != 0 {
		if routers, ok := s.Var("routers"); ok && routers != "" {
			s.Log("routers = ", routers)
			slice := strings.Split(routers, ",")
			ops = append(ops, TFRouter(slice...))
		}
	} else if router, ok := s.Var("router"); ok && router != "" {
		s.Log("router = ", router)
		ops = append(ops, TFRouter(router))
	}

	if pcap, ok := s.Var("pcap"); ok && pcap != "" {
		ops = append(ops, TFPcap(pcap))
	}
	if p.features&attenuatorPresent != 0 {
		if atten, ok := s.Var("attenuator"); ok && atten != "" {
			s.Log("attenuator = ", atten)
			ops = append(ops, TFAttenuator(atten))
		}
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
func newTestFixturePre(name string, features ExtraFeatures, extraOps ...TFOption) *testFixturePreImpl {
	return &testFixturePreImpl{
		name:     name,
		extraOps: extraOps,
		features: features,
	}
}

// testFixturePre is the singleton of test fixture precondition.
var testFixturePre = newTestFixturePre("default", 0)

// TestFixturePre returns a precondition of wificell TestFixture.
func TestFixturePre() testing.Precondition {
	return testFixturePre
}

// testFixturePreWithCapture is the singleton of test fixture precondition with
// TFCapture option.
var testFixturePreWithCapture = newTestFixturePre("capture", 0, TFCapture(true))

// TestFixturePreWithCapture returns a precondition of wificell TestFixture with
// TFCapture(true) option.
func TestFixturePreWithCapture() testing.Precondition {
	return testFixturePreWithCapture
}

// testFixturePreWithRouters is the singleton of test fixture precondition with
// multiple routers option.
var testFixturePreWithRouters *testFixturePreImpl

// TestFixturePreWithRouters returns a precondition of wificell TestFixture with
// multiple routers setup.
func TestFixturePreWithRouters() testing.Precondition {
	// Lazy init, we don't want this to be initialized if not used.
	if testFixturePreWithRouters == nil {
		testFixturePreWithRouters = newTestFixturePre("routers", multiRouters)
	}
	return testFixturePreWithRouters
}

// testFixturePreWithAttenuator is the singleton of test fixture precondition with
// attenuator option.
var testFixturePreWithAttenuator *testFixturePreImpl

// TestFixturePreWithAttenuator returns a precondition of wificell TestFixture with
// multiple routers setup.
func TestFixturePreWithAttenuator() testing.Precondition {
	// Lazy init, we don't want this to be initialized if not used.
	if testFixturePreWithAttenuator == nil {
		testFixturePreWithAttenuator = newTestFixturePre("attenuator", attenuatorPresent)
	}
	return testFixturePreWithAttenuator
}
