// Copyright 2022 The ChromiumOS Authors.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package chameleond

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"

	pbmanager "go.chromium.org/chromiumos/config/go/test/api/test_libs/chameleond_manager"

	"chromiumos/tast/errors"
	"chromiumos/tast/remote/chameleond/manager"
	"chromiumos/tast/testing"
)

// Timeout for methods of Tast fixture.
const (
	setUpTimeout    = 1 * time.Minute
	postTestTimeout = 10 * time.Second
	tearDownTimeout = 10 * time.Second
)

func init() {
	testing.AddFixture(&testing.Fixture{
		Name: "simpleChameleond",
		Desc: "Test fixture for testing endpoints of the Chameleond Manager Service",
		Contacts: []string{
			"jaredbennett@google.com",
		},
		Impl:            &TestFixture{},
		SetUpTimeout:    setUpTimeout,
		PostTestTimeout: postTestTimeout,
		TearDownTimeout: tearDownTimeout,
		Vars:            []string{"ChameleondCFTServiceAddr", "ChameleondDevices"},
	})
}

// TestFixture is an implementation of FixtureImpl for the bluetoothChameleond
// test fixture.
type TestFixture struct {
	ChameleondHosts []*pbmanager.ChameleondHost
	CMS             *manager.ChameleondManagerServiceClient
	vars            *fixtureVars
}

type fixtureVars struct {
	// ChameleondCFTServiceAddr is a service address for the gRPC service as it
	// may be accessed by Tast (e.g. "localhost:50051"). This is required.
	ChameleondCFTServiceAddr string

	// ChameleondDevices is a comma-separated list of "<host>:<port>" for
	// each accessible Chameleond device running in the testbed. The host refers
	// to the Chameleond device and the port refers to the port on which the
	// chameleond XMLRPC service is exposed on (e.g. "chameleonhost:9992"). The
	// first device in the list will be targeted by default.
	//
	// Note: If the Chameleond Manager CFT Service is run locally and not in the
	// test network, you will need to create an ssh tunnel into the test network
	// to and use "localhost:<tunnelport>".
	//
	// This is not required if Tast and the Chameleond CFT is run by CFT, as then
	// the ChameleondManagerService.GetAvailableChameleondHosts RPC call may be
	// used to get this information. If this variable set, then only the provided
	// Chameleond devices are used and the noted RPC call is not preformed, which
	// allows this to be used without CFT.
	ChameleondDevices string
}

func (v *fixtureVars) ParseChameleonDevices() ([]*pbmanager.ChameleondHost, error) {
	devices := make([]*pbmanager.ChameleondHost, 0)
	for _, device := range strings.Split(v.ChameleondDevices, ",") {
		hostAndPort := strings.Split(device, ":")
		if len(hostAndPort) != 2 {
			return nil, errors.Errorf("device string %q does not match required <host>:<port> format", device)
		}
		host := hostAndPort[0]
		port, err := strconv.ParseInt(hostAndPort[1], 10, 64)
		if err != nil {
			return nil, errors.Wrapf(err, "failed to parse port from device string %q", device)
		}
		devices = append(devices, &pbmanager.ChameleondHost{
			Host: host,
			Port: port,
		})
	}
	return devices, nil
}

// SetUp parses the test fixture variables and sets up the fixture with a
// connected ChameleondManagerServiceClient that is targeted at the first
// Chameleond host.
func (tf *TestFixture) SetUp(ctx context.Context, s *testing.FixtState) interface{} {
	// Collect expected fixture variables.
	var devicesStr string
	if val, ok := s.Var("ChameleondDevices"); ok {
		devicesStr = val
	}
	tf.vars = &fixtureVars{
		ChameleondCFTServiceAddr: s.RequiredVar("ChameleondCFTServiceAddr"),
		ChameleondDevices:        devicesStr,
	}

	var err error

	// Connect to CMS.
	tf.CMS, err = manager.NewChameleondManagerServiceClient(ctx, tf.vars.ChameleondCFTServiceAddr)
	if err != nil {
		s.Fatal("Failed to create new ChameleondManagerServiceClient: ", err)
	}

	// Collect available Chameleond hosts in the testbed.
	if tf.vars.ChameleondDevices != "" {
		// Parse hosts based on provided fixture variable.
		tf.ChameleondHosts, err = tf.vars.ParseChameleonDevices()
		if err != nil {
			s.Fatalf("Failed to parse available Chameleond hosts from ChameleondDevices fixture var value %q: %v", tf.vars.ChameleondDevices, err)
		}
	} else {
		// Retrieve from service.
		resp, err := tf.CMS.ChameleondManagerService.GetAvailableChameleondHosts(ctx, &pbmanager.GetAvailableChameleondHostsRequest{})
		if err != nil {
			s.Fatal("Failed to get available Chameleond hosts: ", err)
		}
		tf.ChameleondHosts = resp.Hosts
	}
	testing.ContextLogf(ctx, "Found %d available Chameleond hosts", len(tf.ChameleondHosts))

	// Target first Chameleond host.
	if len(tf.ChameleondHosts) > 0 {
		target := tf.ChameleondHosts[0]
		hostStr := fmt.Sprintf("%s:%d", target.Host, target.Port)
		testing.ContextLogf(ctx, "Targeting first Chameleond host %q", hostStr)
		_, err := tf.CMS.ChameleondManagerService.SetChameleondTarget(ctx, &pbmanager.SetChameleondTargetRequest{
			Target: target,
		})
		if err != nil {
			s.Fatalf("Failed to target Chameleond host %q: %v", hostStr, err)
		}
		testing.ContextLogf(ctx, "Successfully targeted Chameleond host %q", hostStr)
	} else {
		s.Fatal("Invalid test environment: this testbed has no known available Chameleond hosts")
	}

	return tf
}

// Reset does not do anything, but is necessary to include to implement the
// FixtureImpl interface.
func (tf *TestFixture) Reset(ctx context.Context) error {
	return nil
}

// PreTest does not do anything, but is necessary to include to implement the
// FixtureImpl interface.
func (tf *TestFixture) PreTest(ctx context.Context, s *testing.FixtTestState) {}

// PostTest does not do anything yet, but is necessary to include to implement
// the FixtureImpl interface.
func (tf *TestFixture) PostTest(ctx context.Context, s *testing.FixtTestState) {
	// TODO(jaredbennett) collect cms logs
}

// TearDown cleans up fixture connections.
func (tf *TestFixture) TearDown(ctx context.Context, s *testing.FixtState) {
	if err := tf.CMS.Close(); err != nil {
		s.Fatal("Failed to close ChameleondManagerServiceClient: ", err)
	}
}
