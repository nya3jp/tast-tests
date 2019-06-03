// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package example

import (
	"context"
	"io"

	"chromiumos/tast/dut"
	"chromiumos/tast/host"
	"chromiumos/tast/rpc"
	svcpb "chromiumos/tast/service/cros"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:     GRPC,
		Desc:     "Demonstrates how to use gRPC support to run Go code on DUT",
		Contacts: []string{"nya@chromium.org", "tast-users@chromium.org"},
		Attr:     []string{"informational"},
	})
}

func GRPC(ctx context.Context, s *testing.State) {
	d, ok := dut.FromContext(ctx)
	if !ok {
		s.Fatal("Failed to get DUT")
	}

	// Connect to the local bundle.
	// TODO(nya): Provide a canonical way to connect to a gRPC server on a DUT.
	h, err := d.Start(ctx, "/usr/local/libexec/tast/bundles/local_pushed/cros -rpc", host.OpenStdin, host.StdoutOnly)
	if err != nil {
		s.Fatal("Failed to connect to the RPC service on the DUT: ", err)
	}
	defer h.Close(ctx)

	conn, err := rpc.NewClientConn(ctx, h.Stdout(), h.Stdin())
	if err != nil {
		s.Fatal("Failed to establish a RPC connection: ", err)
	}
	defer conn.Close()

	// Start a relay logger.
	// TODO(nya): Provide a canonical way to connect to a gRPC server on a DUT.
	logger := rpc.NewLoggingClient(conn)

	logCtx, logCancel := context.WithCancel(ctx)
	defer logCancel()

	cl, err := logger.ReadLogs(logCtx, &rpc.ReadLogsRequest{})
	if err != nil {
		s.Fatal("ReadLogs failed: ", err)
	}

	go func() {
		for {
			res, err := cl.Recv()
			if err != nil {
				if err != io.EOF && logCtx.Err() == nil {
					s.Log("ReadLogs failed: ", err)
				}
				return
			}
			s.Log(res.Msg)
		}
	}()

	// Main part of this example.
	fs := svcpb.NewFileSystemClient(conn)

	const root = "/mnt/stateful_partition"

	res, err := fs.ReadDir(ctx, &svcpb.ReadDirRequest{Dir: root})
	if err != nil {
		s.Fatal("ReadDir failed: ", err)
	}

	s.Logf("List of files at %s on the DUT:", root)
	for _, fi := range res.Files {
		s.Logf("  %s", fi.Name)
	}
}
