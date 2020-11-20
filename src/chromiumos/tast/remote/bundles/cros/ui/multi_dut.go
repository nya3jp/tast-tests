// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package ui

import (
	"context"

	"github.com/golang/protobuf/ptypes/empty"

	"chromiumos/tast/dut"
	"chromiumos/tast/errors"
	"chromiumos/tast/rpc"
	"chromiumos/tast/services/cros/example"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func: MultiDUT,
		Desc: "Checks basic multi-DUT connection",
		Contacts: []string{
			"chromeos-sw-engprod@google.com",
		},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"chrome"},
		ServiceDeps:  []string{"tast.cros.example.ChromeService"},
		Vars:         []string{"secondaryTarget"},
	})
}

// MultiDUT tests that we can get a handle to a secondary DUT and connect to Chrome.
func MultiDUT(ctx context.Context, s *testing.State) {
	d1 := s.DUT()

	secondaryTarget, ok := s.Var("secondaryTarget")
	if !ok {
		s.Fatal("Need to provide a secondaryTarget argument to 'tast run' command")
	}
	s.Log("Connecting to secondary DUT: ", secondaryTarget)

	d2, err := d1.NewSecondaryDevice(secondaryTarget)
	if err != nil {
		s.Fatal("Failed to create secondary device: ", err)
	}
	if err := d2.Connect(ctx); err != nil {
		s.Fatal("Failed to connect to d2: ", err)
	}

	if err := testChromeOnDUT(ctx, s, d1); err != nil {
		s.Fatal("Failed to launch Settings on primary DUT: ", err)
	}

	if err := testChromeOnDUT(ctx, s, d2); err != nil {
		s.Fatal("Failed to launch Settings on secondary DUT: ", err)
	}
}

func testChromeOnDUT(ctx context.Context, s *testing.State, d *dut.DUT) error {
	// Connect to the gRPC server on the DUT.
	cl, err := rpc.Dial(ctx, d, s.RPCHint(), "cros")
	if err != nil {
		return errors.Wrap(err, "failed to connect to the RPC service on the DUT")
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
		return errors.Wrapf(err, "failed to eval %s", expr)
	}
	s.Logf("Eval(%q) = %s", expr, res.ValueJson)
	return nil
}
