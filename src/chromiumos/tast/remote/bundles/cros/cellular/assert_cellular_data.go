// Copyright 2022 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package cellular

import (
	"context"
	"sync"
	"time"

	"github.com/golang/protobuf/ptypes/empty"

	"chromiumos/tast/errors"
	"chromiumos/tast/exec"
	"chromiumos/tast/remote/cellular/callbox/manager"
	"chromiumos/tast/rpc"
	"chromiumos/tast/services/cros/example"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func: AssertCellularData,
		Desc: "Asserts that cellular data works. The test establishes a connection to the appropriate CMW500 callbox. Then it asserts that the cellular data connection provided to it matches the data connection provided by ethernet. Any differences are considered an error. If the cellular data connection is not provided, the second curl will throw an exception",
		Contacts: []string{
			"latware@google.com",
			"chromeos-cellular-team@google.com",
		},
		Attr:         []string{},
		ServiceDeps:  []string{"tast.cros.example.ChromeService"},
		SoftwareDeps: []string{"chrome"},
		Fixture:      "callboxManagedFixture",
		Timeout:      5 * time.Minute,
	})
}

func AssertCellularData(ctx context.Context, s *testing.State) {
	// Connect to the gRPC server on the DUT.
	cl, err := rpc.Dial(ctx, s.DUT(), s.RPCHint())
	if err != nil {
		s.Fatal("Failed to connect to the RPC service on the DUT: ", err)
	}
	defer cl.Close(ctx)

	cr := example.NewChromeServiceClient(cl.Conn)

	if _, err := cr.New(ctx, &empty.Empty{}); err != nil {
		s.Fatal("Failed to start Chrome: ", err)
	}
	defer cr.Close(ctx, &empty.Empty{})

	const expr = "chrome.i18n.getUILanguage()"
	req := &example.EvalOnTestAPIConnRequest{Expr: expr}
	res, err := cr.EvalOnTestAPIConn(ctx, req)
	if err != nil {
		s.Fatalf("Failed to eval %s: %v", expr, err)
	}
	s.Logf("Eval(%q) = %s", expr, res.ValueJson)

	tf := s.FixtValue().(*manager.TestFixture)
	dutConn := s.DUT().Conn()

	// Disable and then re-enable cellular on DUT
	if err := dutConn.CommandContext(ctx, "dbus-send", []string{
		"--system",
		"--fixed",
		"--print-reply",
		"--dest=org.chromium.flimflam",
		"/",
		"org.chromium.flimflam.Manager.DisableTechnology",
		"string:cellular",
	}...).Run(exec.DumpLogOnError); err != nil {
		s.Fatal("Failed to disable DUT cellular: ", err)
	}

	// Preform callbox simulation
	if err := tf.CallboxManagerClient.ConfigureCallbox(ctx, &manager.ConfigureCallboxRequestBody{
		Hardware:     "CMW",
		CellularType: "LTE",
		ParameterList: []string{
			"band", "2",
			"bw", "20",
			"mimo", "2x2",
			"tm", "1",
			"pul", "0",
			"pdl", "high",
		},
	}); err != nil {
		s.Fatal("Failed to configure callbox: ", err)
	}
	// Allow for cellular simulation to start before turning on mobile data
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		if err := tf.CallboxManagerClient.BeginSimulation(ctx, nil); err != nil {
			s.Fatal("Failed to begin callbox simulation: ", err)
		}
	}()
	// Functionality that will be required to poll not yet available, see b/229419538
	testing.Sleep(ctx, time.Second * 10)
	// Turn on mobile data to force a connection
	if err := dutConn.CommandContext(ctx, "dbus-send", []string{
		"--system",
		"--fixed",
		"--print-reply",
		"--dest=org.chromium.flimflam",
		"/",
		"org.chromium.flimflam.Manager.EnableTechnology",
		"string:cellular",
	}...).Run(exec.DumpLogOnError); err != nil {
		s.Fatal("Failed to enable DUT cellular: ", err)
	}
	// Required due to bug using esim as primary, see b/229421807
	if err := testing.Poll(ctx, func(ctx context.Context) error {
		if err := dutConn.CommandContext(ctx, "mmcli", []string{
			"-m",
			"any",
			"--set-primary-sim-slot=2",
		}...).Run(exec.DumpLogOnError); err != nil {
			return errors.New("error in switching primary sim")
		}
		return err
	}, &testing.PollOptions{Interval: time.Second * 5, Timeout: time.Second * 10}); err != nil {
		s.Fatal("Failed to switch primary sim: ", err)
	}

	wg.Wait()

	// Assert cellular connection on DUT can connect to a URL like ethernet can
	testURL := "google.com"
	ethernetResult, err := dutConn.CommandContext(ctx, "curl", "--interface", "eth0", testURL).Output()
	if err != nil {
		s.Fatalf("Failed to curl %q on DUT using ethernet interface: %v", testURL, err)
	}

	if err := testing.Poll(ctx, func(ctx context.Context) error {
		_, err := dutConn.CommandContext(ctx, "curl", "--interface", "rmnet_data0", testURL).Output()
		if err != nil {
			errors.New("failed to curl on DUT using cellular interface")
		}
		return err
	}, &testing.PollOptions{Interval: time.Second * 10, Timeout: time.Second * 200}); err != nil {
		s.Fatalf("Failed to curl %q on DUT using cellular interface: %v", testURL, err)
	}

	cellularResult, err := dutConn.CommandContext(ctx, "curl", "--interface", "rmnet_data0", testURL).Output()
	if err != nil {
		s.Fatalf("Failed to curl %q on DUT using cellular interface: %v", testURL, err)
	}
	ethernetResultStr := string(ethernetResult)
	cellularResultStr := string(cellularResult)
	if ethernetResultStr != cellularResultStr {
		s.Fatal("Ethernet and cellular curl output not equal")
	}
}
