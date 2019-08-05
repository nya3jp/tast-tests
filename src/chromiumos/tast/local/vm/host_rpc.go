// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package vm

import (
	"context"
	"net"

	"github.com/mdlayher/vsock"
	"google.golang.org/grpc"

	"chromiumos/tast/errors"
	vmtools "chromiumos/vm_tools/vm_rpc"
)

// StartupListenerServer is struct to manage an instance of a StartupListener
// gRPC server. It is designed to listen for a single VmReady call from a VM at
// the provided CID.
type StartupListenerServer struct {
	port   uint32
	rec    chan struct{}
	lis    net.Listener
	server *grpc.Server
}

// NewStartupListenerServer accepts a vsock port to listen on as a parameter,
// and returns a new StatupListenerServer struct. At this point the server has
// not been started.
func NewStartupListenerServer(vsockPort uint32) (*StartupListenerServer, error) {
	s := StartupListenerServer{
		server: grpc.NewServer(),
		rec:    make(chan struct{}),
		port:   vsockPort,
	}

	vmtools.RegisterStartupListenerServer(s.server, &s)
	return &s, nil
}

// Start creates the vsock port listener and starts the gRPC server in a goroutine.
func (s *StartupListenerServer) Start() error {
	var err error
	if s.lis, err = vsock.Listen(s.port); err != nil {
		return errors.Wrapf(err, "unable to start listening on vsock port %d", s.port)
	}

	go s.server.Serve(s.lis)
	return nil
}

// Stop will stop the gRPC server.
func (s *StartupListenerServer) Stop() {
	s.server.Stop()
}

// WaitReady will block until the VmReady call is received. If the message is
// not received before the context timeout, an error is returned.
func (s *StartupListenerServer) WaitReady(ctx context.Context) error {
	select {
	case <-s.rec:
		return nil
	case <-ctx.Done():
		return errors.Wrap(ctx.Err(), "context timeout waiting for server to be ready")
	}
}

// VmReady is the implementation of the StartupListenerServer gRPC stub. It
// sends a signal through an empty channel when a message is received.
// Note: golint flags this function because Vm is not fully capitalized, however
// this is due to the proto definition.
func (s *StartupListenerServer) VmReady(ctx context.Context, msg *vmtools.EmptyMessage) (*vmtools.EmptyMessage, error) {
	close(s.rec)
	return &vmtools.EmptyMessage{}, nil
}
