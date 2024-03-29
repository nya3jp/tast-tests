// Copyright 2022 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package manager

import (
	"context"
	"fmt"
	"net"
	"strings"
	"sync"
	"time"

	"github.com/golang/protobuf/ptypes/empty"

	"chromiumos/tast/dut"
	"chromiumos/tast/errors"
	"chromiumos/tast/rpc"
	"chromiumos/tast/services/cros/cellular"
	"chromiumos/tast/ssh"
	"chromiumos/tast/testing"
)

// Timeout for methods of Tast fixture.
const (
	setUpTimeout    = 3 * time.Minute
	tearDownTimeout = 3 * time.Minute
	resetTimeout    = 1 * time.Second
	postTestTimeout = 1 * time.Second
	testURL         = "google.com"
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
		ServiceDeps:     []string{"tast.cros.cellular.RemoteCellularService"},
		Vars:            []string{"callboxManager", "callbox"},
	})
}

// TestFixture is the test fixture used for callboxManagedFixture fixtures.
type TestFixture struct {
	fcm                  *forwardedCallboxManager
	rpcClient            *rpc.Client
	CallboxManagerClient *CallboxManagerClient
	RemoteCellularClient cellular.RemoteCellularServiceClient
	InterfaceName        string
	Vars                 fixtureVars
}

type fixtureVars struct {
	CallboxManager string
	Callbox        string
}

// SetUp sets up the test fixture and connects to the CallboxManager.
func (tf *TestFixture) SetUp(ctx context.Context, s *testing.FixtState) interface{} {
	dut := s.DUT()

	// Parse Vars
	callbox, ok := s.Var("callbox")
	if !ok || callbox == "" {
		testing.ContextLog(ctx, "No callbox specified, deducing from DUT name")
		var err error
		if callbox, err = callboxHostName(dut); err != nil {
			s.Fatal("Failed to determine callbox hostname: ", err)
		}
	}
	tf.Vars.Callbox = callbox
	testing.ContextLogf(ctx, "Using callbox: %q", tf.Vars.Callbox)

	callboxManager, ok := s.Var("callboxManager")
	if !ok {
		testing.ContextLog(ctx, "No callboxManager specified, defaulting to lookup")
		callboxManager = ""
		tf.Vars.CallboxManager = callboxManager
	}
	if callboxManager != "" {
		testing.ContextLogf(ctx, "callboxManager: %s", callboxManager)
		tf.Vars.CallboxManager = callboxManager
	} else if callboxManager := callboxManagerByCallbox(tf.Vars.Callbox); callboxManager != "" {
		testing.ContextLogf(ctx, "callboxManager: %s (deduced from callbox)", callboxManager)
		tf.Vars.CallboxManager = callboxManager
	}

	// Initialize CallboxManagerClient
	if tf.Vars.CallboxManager == labProxyHostname {
		// Tunnel to Callbox Manager on labProxyHostname
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

	cl, err := rpc.Dial(ctx, dut, s.RPCHint())
	if err != nil {
		s.Fatal("Failed to connect to the RPC service on the DUT: ", err)
	}
	tf.rpcClient = cl

	tf.RemoteCellularClient = cellular.NewRemoteCellularServiceClient(cl.Conn)
	if _, err := tf.RemoteCellularClient.SetUp(ctx, &empty.Empty{}); err != nil {
		s.Fatal("Failed to initialize cellular shill service on DUT: ", err)
	}

	if resp, err := tf.RemoteCellularClient.QueryInterface(ctx, &empty.Empty{}); err != nil {
		s.Fatal("Failed to query cellular interface: ", err)
	} else {
		s.Logf("Using cellular interface %q", resp.Name)
		tf.InterfaceName = resp.Name
	}

	return tf
}

// ConnectToCallbox function handles initial test setup and wraps parameters.
func (tf *TestFixture) ConnectToCallbox(ctx context.Context, dutConn *ssh.Conn, configureRequestBody *ConfigureCallboxRequestBody) error {
	// Disable and then re-enable cellular on DUT.
	if _, err := tf.RemoteCellularClient.Disable(ctx, &empty.Empty{}); err != nil {
		return errors.Wrap(err, "failed to disable DUT cellular")
	}

	// Preform callbox simulation.
	if err := tf.CallboxManagerClient.ConfigureCallbox(ctx, configureRequestBody); err != nil {
		return errors.Wrap(err, "failed to configure callbox")
	}

	// Allow for cellular simulation to start before turning on mobile data.
	errCh := make(chan error, 1)
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		if err := tf.CallboxManagerClient.BeginSimulation(ctx, nil); err != nil {
			errCh <- errors.Wrap(err, "failed to begin callbox simulation")
		}
	}()
	// TODO(b/229419538): Add functionality to callbox libraries to pull state
	testing.Sleep(ctx, time.Second*10)
	if _, err := tf.RemoteCellularClient.Enable(ctx, &empty.Empty{}); err != nil {
		return errors.Wrap(err, "failed to enable DUT cellular")
	}

	wg.Wait()
	close(errCh)
	if len(errCh) > 0 {
		return <-errCh
	}

	// now attached but not connected, toggle modem and connect to make sure
	// everything is synced properly between the callbox and DUT
	if err := tf.ToggleConnection(ctx); err != nil {
		return errors.Wrap(err, "failed to toggle cellular connection")
	}

	// verify cellular connection by curling a website
	curlArgs := []string{"-m", "5", "--interface", tf.InterfaceName, testURL}
	retryCount := 0
	if err := testing.Poll(ctx, func(ctx context.Context) error {
		retryCount++
		testing.ContextLogf(ctx, "curling %q attempt: %d", testURL, retryCount)
		if _, err := dutConn.CommandContext(ctx, "curl", curlArgs...).Output(); err != nil {
			return err
		}

		return nil
	}, &testing.PollOptions{Timeout: time.Minute, Interval: time.Second}); err != nil {
		return errors.Wrapf(err, "failed curl %q on DUT using cellular interface", "google.com")
	}
	return nil
}

// ToggleConnection disables and then re-enables the device, and then reconnects to the default cellular service.
func (tf *TestFixture) ToggleConnection(ctx context.Context) error {
	if _, err := tf.RemoteCellularClient.Disable(ctx, &empty.Empty{}); err != nil {
		return errors.Wrap(err, "failed to disable cellular service")
	}
	if _, err := tf.RemoteCellularClient.Enable(ctx, &empty.Empty{}); err != nil {
		return errors.Wrap(err, "failed to enable cellular service")
	}
	if _, err := tf.RemoteCellularClient.Connect(ctx, &empty.Empty{}); err != nil {
		return errors.Wrap(err, "failed to connect to cellular service")
	}
	return nil
}

// callboxHostName derives the hostname of the callbox from the dut's hostname.
//
// Callbox DUT hostnames follow the convention: <callbox_hostname>-host<host_number>
// e.g. a callbox with the name "chromeos1-donutlab-callbox1" may support the following DUTs:
// "chromeos1-donutlab-callbox1-host1", "chromeos1-donutlab-callbox1-host2", ...
func callboxHostName(dut *dut.DUT) (string, error) {
	dutHost := dut.HostName()
	if host, _, err := net.SplitHostPort(dutHost); err == nil {
		dutHost = host
	}

	dutHost = strings.TrimSuffix(dutHost, ".cros")
	if dutHost == "localhost" {
		return "", errors.Errorf("unable to parse hostname from: %q, localhost not supported", dutHost)
	}

	if ip := net.ParseIP(dutHost); ip != nil {
		return "", errors.Errorf("unable to parse hostname from: %q, ip:port format not supported", dutHost)
	}

	hostname := strings.Split(dutHost, "-")
	if len(hostname) < 2 {
		return "", errors.Errorf("unable to parse hostname from: %q, unknown name format", dutHost)
	}

	// CallboxManager expects callbox hostnames to end in .cros
	hostname = hostname[0 : len(hostname)-1]
	return fmt.Sprintf("%s.cros", strings.Join(hostname, "-")), nil
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

// TearDown releases resources held open by the test fixture.
func (tf *TestFixture) TearDown(ctx context.Context, s *testing.FixtState) {
	if tf.fcm != nil {
		if err := tf.fcm.Close(ctx); err != nil {
			s.Error("Failed to close tunnel to CallboxManager: ", err)
		}
	}

	if _, err := tf.RemoteCellularClient.TearDown(ctx, &empty.Empty{}); err != nil {
		s.Error("Failed to tear down cellular remote service: ", err)
	}

	if err := tf.rpcClient.Close(ctx); err != nil {
		s.Error("Failed to close DUT RPC client: ", err)
	}
}
