// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package example

import (
	"context"

	"github.com/golang/protobuf/ptypes/empty"

	"chromiumos/tast/rpc"
	"chromiumos/tast/services/cros/example"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         GRPC,
		Desc:         "Demonstrates how to use gRPC support to run Go code on DUT",
		Contacts:     []string{"nya@chromium.org", "tast-users@chromium.org"},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"chrome"},
		ServiceDeps:  []string{"tast.cros.example.Chrome"},
	})
}

func GRPC(ctx context.Context, s *testing.State) {
	// Connect to the gRPC server on the DUT.
	cl, err := rpc.Dial(ctx, s.DUT(), s.RPCHint(), "cros")
	if err != nil {
		s.Fatal("Failed to connect to the RPC service on the DUT: ", err)
	}
	defer cl.Close(ctx)

	cr := example.NewChromeClient(cl.Conn)

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
}
