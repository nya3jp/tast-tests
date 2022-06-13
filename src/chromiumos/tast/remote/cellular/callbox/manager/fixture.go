// Copyright 2022 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package manager

import (
	"context"
	"sync"
	"time"

	"github.com/golang/protobuf/ptypes/empty"

	"chromiumos/tast/exec"
	"chromiumos/tast/rpc"
	"chromiumos/tast/services/cros/example"
	"chromiumos/tast/ssh"
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

var loggedIn = false

// ConnectToCallbox function handles initial test setup and wraps parameters.
func (tf *TestFixture) ConnectToCallbox(ctx context.Context, s *testing.State, dutConn *ssh.Conn, configureRequestBody *ConfigureCallboxRequestBody, cellularInterface string) interface{} {
	connectionExpected := true
	powerOnNextIndex := false
	for _, s := range configureRequestBody.ParameterList {
		if powerOnNextIndex {
			if s == "disconnected" {
				connectionExpected = false
			}
			break
		}
		if s == "pdl" {
			powerOnNextIndex = true
		}
	}
	if !loggedIn {
		loggedIn = true
		// Connect to the gRPC server on the DUT.
	        ctxForCleanupcl := ctx
		cl, err := rpc.Dial(ctxForCleanupcl, s.DUT(), s.RPCHint())
	        if err != nil {
		        s.Fatal("Failed to connect to the RPC service on the DUT: ", err)
	        }
		defer cl.Close(ctxForCleanupcl)

	        ctxForCleanupcr := ctx
		cr := example.NewChromeServiceClient(cl.Conn)

	        if _, err := cr.New(ctxForCleanupcr, &empty.Empty{}); err != nil {
			s.Fatal("Failed to start Chrome: ", err)
		}
	        defer cr.Close(ctxForCleanupcr, &empty.Empty{})

		const expr = "chrome.i18n.getUILanguage()"
	        req := &example.EvalOnTestAPIConnRequest{Expr: expr}
		res, err := cr.EvalOnTestAPIConn(ctx, req)
	        if err != nil {
		        s.Fatalf("Failed to eval %s: %v, %s", expr, err, res.ValueJson)
	        }
	}

        // Disable and then re-enable cellular on DUT.
        if err := dutConn.CommandContext(ctx, "dbus-send", "--system", "--fixed", "--print-reply", "--dest=org.chromium.flimflam", "/", "org.chromium.flimflam.Manager.DisableTechnology", "string:cellular").Run(exec.DumpLogOnError); err != nil {
                s.Fatal("Failed to disable DUT cellular: ", err)
        }

	// Preform callbox simulation.
        if err := tf.CallboxManagerClient.ConfigureCallbox(ctx, configureRequestBody); err != nil {
                s.Fatal("Failed to configure callbox: ", err)
        }

	// Allow for cellular simulation to start before turning on mobile data.
        var wg sync.WaitGroup
        wg.Add(1)
        go func() {
                defer wg.Done()
                if err := tf.CallboxManagerClient.BeginSimulation(ctx, nil); err != nil {
			if !connectionExpected {
				s.Fatal("Failed to begin callbox simulation: ", err)
			}
                }
        }()
        // TODO(b/229419538): Add functionality to callbox libraries to pull state
        testing.Sleep(ctx, time.Second * 10)
        // Turn on mobile data to force a connection.
        if err := dutConn.CommandContext(ctx, "dbus-send", "--system", "--fixed", "--print-reply", "--dest=org.chromium.flimflam", "/", "org.chromium.flimflam.Manager.EnableTechnology", "string:cellular").Run(exec.DumpLogOnError); err != nil {
                s.Fatal("Failed to enable DUT cellular: ", err)
        }
        // Required due to bug using esim as primary, see b/229421807.
        if err := testing.Poll(ctx, func(ctx context.Context) error {
                return dutConn.CommandContext(ctx, "mmcli", "-m", "any", "--set-primary-sim-slot=2").Run(exec.DumpLogOnError)
        }, &testing.PollOptions{Interval: time.Second * 5, Timeout: time.Second * 15}); err != nil {
                s.Fatal("Failed to switch primary sim: ", err)
        }

        wg.Wait()

	if connectionExpected {
		if err := testing.Poll(ctx, func(ctx context.Context) error {
		        _, err := dutConn.CommandContext(ctx, "curl", "--interface", cellularInterface, "google.com").Output()
			return err
		}, &testing.PollOptions{Interval: time.Second * 10, Timeout: time.Second * 200}); err != nil {
			s.Fatalf("Failed curl %q on DUT using cellular interface: %v", "google.com", err)
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
