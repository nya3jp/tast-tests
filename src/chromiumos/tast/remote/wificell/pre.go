// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package wificell

import (
	"context"
	"fmt"
	"strings"
	"time"

	"chromiumos/tast/dut"
	"chromiumos/tast/testing"
	"chromiumos/tast/timing"
)

// TFFeatures is enum for any extra features needed for precondition.
type TFFeatures uint8

const (
	// TFFeaturesNone reporesents a default value.
	TFFeaturesNone TFFeatures = 0
	// TFFeaturesCapture is a feature that spawns packet capturer in TestFixture.
	TFFeaturesCapture = 1 << iota
	// TFFeaturesRouters allows to configure more than one router.
	TFFeaturesRouters
	// TFFeaturesAttenuator feature facilitates attenuator handling.
	TFFeaturesAttenuator
	// TFFeaturesCiscoController enables use of Cisco controller and access points.
	TFFeaturesCiscoController
)

// String returns name component corresponding to enum value(s).
func (enum TFFeatures) String() string {
	if enum == 0 {
		return "default"
	}
	var ret []string
	if enum&TFFeaturesCapture != 0 {
		ret = append(ret, "capture")
		// Punch out the bit to check for weird values later.
		enum ^= TFFeaturesCapture
	}
	if enum&TFFeaturesRouters != 0 {
		ret = append(ret, "routers")
		enum ^= TFFeaturesRouters
	}
	if enum&TFFeaturesAttenuator != 0 {
		ret = append(ret, "attenuator")
		enum ^= TFFeaturesAttenuator
	}
	if enum&TFFeaturesCiscoController != 0 {
		ret = append(ret, "cisco")
		enum ^= TFFeaturesCiscoController
	}
	// Catch weird cases. Like when somebody extends enum, but forgets to extend this.
	if enum != 0 {
		panic(fmt.Sprintf("Invalid TFFeatures enum, residual bits :%d", enum))
	}

	return strings.Join(ret, "&")
}

type testFixturePreImpl struct {
	name     string
	features TFFeatures
	tf       *TestFixture
}

// String returns a short, underscore-separated name for the precondition.
func (p *testFixturePreImpl) String() string {
	return "wificell_test_fixture_" + p.name
}

// Timeout returns the amount of time dedicated to prepare and close the precondition.
func (p *testFixturePreImpl) Timeout() time.Duration {
	// When connecting to devices with slower SSH, e.g. in lab, the first
	// Prepare might take up to 30 seconds. Also, it is possible that
	// something is broken and we'll have to reconstruct the TestFixture.
	// Set a sufficient timeout for these cases.
	return 2 * time.Minute
}

// companionName facilitates obtaining of the companion device's hostname.
func companionName(s *testing.PreState, suffix string) string {
	name, err := s.DUT().CompanionDeviceHostname(suffix)
	if err != nil {
		s.Fatal("Unable to synthesize name, err: ", err)
	}
	return name
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
		err := p.tf.Reinit(ctx)
		if err == nil { // Reinit succeeded, we're done.
			return p.tf
		}
		// Reinit failed, close the old TestFixture.
		s.Log("Try recreating the TestFixture as it failed to re-initialize, err: ", err)
		if err := p.tf.Close(ctx); err != nil {
			s.Log("Failed to close the broken TestFixture, err: ", err)
		}
		p.tf = nil
		// Fallthrough the creation of TestFixture.
	}

	// Create TestFixture.
	var ops []TFOption
	// Read router/pcap variable. If not available or empty, NewTestFixture
	// will fall back to Default{Router,Pcap}Host.
	if p.features&TFFeaturesRouters != 0 {
		if routers, ok := s.Var("routers"); ok && routers != "" {
			s.Log("routers: ", routers)
			slice := strings.Split(routers, ",")
			if len(slice) < 2 {
				s.Fatal("You must provide at least two router names")
			}
			ops = append(ops, TFRouter(slice...))
		} else {
			routers := []string{
				companionName(s, dut.CompanionSuffixRouter),
				// Use AP named as packet capturer as the second router
				// when TFFeaturesRouters is set.
				companionName(s, dut.CompanionSuffixPcap),
			}
			s.Log("companion routers: ", routers)
			ops = append(ops, TFRouter(routers...))
		}
	} else {
		router, ok := s.Var("router")
		if ok && router != "" {
			s.Log("router: ", router)
			ops = append(ops, TFRouter(router))
		} // else: let TestFixture resolve the name.
	}
	pcap, ok := s.Var("pcap")
	if ok && pcap != "" {
		s.Log("pcap: ", pcap)
		ops = append(ops, TFPcap(pcap))
	} // else: let TestFixture resolve the name.
	// Read attenuator variable.
	if p.features&TFFeaturesAttenuator != 0 {
		atten, ok := s.Var("attenuator")
		if !ok || atten == "" {
			// Attenuator is not typical companion, so we synthesize its name here.
			atten = companionName(s, "-attenuator")
		}
		s.Log("attenuator: ", atten)
		ops = append(ops, TFAttenuator(atten))
	}
	// Read ciscoctrl variable.
	if p.features&TFFeaturesCiscoController != 0 {
		cisco, ok := s.Var("ciscoctrl")
		if !ok || cisco == "" {
			// Only one setup currently, no naming scheme to be used
			cisco = "user:passwd:chromeos1-ap-controller"
		}
		s.Log("ciscoctrl: ", cisco)
		ops = append(ops, TFCiscoCtrl(cisco))
	}
	// Enable capturing.
	if p.features&TFFeaturesCapture != 0 {
		ops = append(ops, TFCapture(true))
	}
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
func newTestFixturePre(name string, features TFFeatures) *testFixturePreImpl {
	return &testFixturePreImpl{
		name:     name,
		features: features,
	}
}

var testFixturePre = make(map[TFFeatures]*testFixturePreImpl)

// TestFixturePreWithFeatures returns a precondition of wificell TestFixture with
// a set of features enabled.
func TestFixturePreWithFeatures(features TFFeatures) testing.Precondition {
	if _, ok := testFixturePre[features]; !ok {
		testFixturePre[features] = newTestFixturePre(features.String(), features)
	}
	return testFixturePre[features]
}

// TestFixturePre returns a precondition of wificell TestFixture.
func TestFixturePre() testing.Precondition {
	return TestFixturePreWithFeatures(TFFeaturesNone)
}

// TestFixturePreWithCapture returns a precondition of wificell TestFixture with
// TFCapture(true) option.
func TestFixturePreWithCapture() testing.Precondition {
	return TestFixturePreWithFeatures(TFFeaturesCapture)
}
