// Copyright 2022 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package manager

import (
	"context"
	"time"

	"chromiumos/tast/testing"
)

// Timeout for methods of Tast fixture.
const (
	setUpTimeout    = 1 * time.Minute
	tearDownTimeout = 10 * time.Second
	resetTimeout    = 1 * time.Second
	postTestTimeout = 1 * time.Second
)

func init() {
	testing.AddFixture(&testing.Fixture{
		Name:     "callboxManagedFixture",
		Desc:     "Cellular fixture with a Callbox managed by a Callbox Manager",
		Contacts: []string{
			// None yet, fixture is still preliminary
		},
		Impl:            &TestFixture{},
		SetUpTimeout:    setUpTimeout,
		ResetTimeout:    resetTimeout,
		PostTestTimeout: postTestTimeout,
		TearDownTimeout: tearDownTimeout,
		ServiceDeps:     []string{},
		Vars:            []string{"callboxManager", "callbox"},
	})
}

// TestFixture is the test fixture used for callboxManagedFixture fixtures.
type TestFixture struct {
	fcm                  *forwardedCallboxManager
	CallboxManagerClient *CallboxManagerClient
	Vars                 fixtureVars
}

type fixtureVars struct {
	CallboxManager string
	Callbox        string
}

// SetUp sets up the test fixture and connects to the CallboxManager.
func (tf *TestFixture) SetUp(ctx context.Context, s *testing.FixtState) interface{} {
	// Parse Vars
	if callbox, ok := s.Var("callbox"); ok && callbox != "" {
		testing.ContextLog(ctx, "callbox: ", callbox)
		tf.Vars.Callbox = callbox
	} else {
		s.Fatal("Fixture variable 'callbox' is required")
	}
	if callboxManager, ok := s.Var("callboxManager"); ok && callboxManager != "" {
		testing.ContextLogf(ctx, "callboxManager: %s", callboxManager)
		tf.Vars.CallboxManager = callboxManager
	} else if callboxManager := callboxManagerByCallbox(tf.Vars.Callbox); callboxManager != "" {
		testing.ContextLogf(ctx, "callboxManager: %s (deduced from callbox)", callboxManager)
		tf.Vars.CallboxManager = callboxManager
	} else {
		s.Fatalf("Fixture variable 'callboxManager' is required with callbox %q", tf.Vars.Callbox)
	}

	// Initialize CallboxManagerClient
	if tf.Vars.CallboxManager == labProxyHostname {
		// Tunnel to Callbox Manager on labProxyHostname
		dut := s.DUT()
		var err error
		tf.fcm, err = newForwardToLabCallboxManager(ctx, dut.KeyDir(), dut.KeyFile())
		if err != nil {
			s.Fatalf("Failed to open tunnel to Callbox Manager on %q, err: %v", labProxyHostname, err)
		}
		tf.CallboxManagerClient = &CallboxManagerClient{
			baseURL:        "http://" + tf.fcm.LocalAddress(),
			defaultCallbox: tf.Vars.Callbox,
		}
	} else {
		// Callbox Manager directly accessible
		tf.CallboxManagerClient = &CallboxManagerClient{
			baseURL:        "http://" + tf.Vars.CallboxManager,
			defaultCallbox: tf.Vars.Callbox,
		}
	}

	return tf
}

var callboxManagerByCallboxLookup = map[string]string{
	"chromeos1-donutlab-callbox1.cros": labProxyHostname,
	"chromeos1-donutlab-callbox2.cros": labProxyHostname,
	"chromeos1-donutlab-callbox3.cros": labProxyHostname,
	"chromeos1-donutlab-callbox4.cros": labProxyHostname,
}

func callboxManagerByCallbox(callbox string) string {
	if callboxManager, ok := callboxManagerByCallboxLookup[callbox]; ok && callboxManager != "" {
		return callboxManager
	}
	return ""
}

// Reset does nothing currently, but is required for the test fixture.
func (tf *TestFixture) Reset(ctx context.Context) error {
	return nil
}

// PreTest does nothing currently, but is required for the test fixture.
func (tf *TestFixture) PreTest(ctx context.Context, s *testing.FixtTestState) {

}

// PostTest does nothing currently, but is required for the test fixture.
func (tf TestFixture) PostTest(ctx context.Context, s *testing.FixtTestState) {

}

// TearDown shuts down the tunnel to the CallboxManager if one was created.
func (tf *TestFixture) TearDown(ctx context.Context, s *testing.FixtState) {
	if tf.fcm != nil {
		if err := tf.fcm.Close(ctx); err != nil {
			s.Fatal("Failed to close tunnel to CallboxManager during tear down: ", err)
		}
	}
}
