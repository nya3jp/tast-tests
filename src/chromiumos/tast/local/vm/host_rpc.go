// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package vm

import (
	"context"
	"net"

	"github.com/mdlayher/vsock"
	"google.golang.org/grpc"

	vmtools "chromiumos/vm_tools/vm_rpc"
)

// StartupListenerServer is struct to manage an instance of a StartupListener
// gRPC server.
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
	startupServer := StartupListenerServer{}
	startupServer.server = grpc.NewServer()
	startupServer.rec = make(chan struct{}, 1)
	startupServer.port = vsockPort

	vmtools.RegisterStartupListenerServer(startupServer.server, &startupServer)
	return &startupServer, nil
}

// Start creates the vsock port listener and starts the gRPC server in a goroutine.
func (s *StartupListenerServer) Start() error {
	var err error
	s.lis, err = vsock.Listen(s.port)
	if err != nil {
		return err
	}

	go s.server.Serve(s.lis)
	return nil
}

// Stop will close the vsock port lisener and stops the gRPC server.
func (s *StartupListenerServer) Stop() {
	s.lis.Close()
	s.server.Stop()
}

// WaitReady will block until the VmReady call is received. If the message is
// not received before the context timeout, an error is returned.
func (s *StartupListenerServer) WaitReady(ctx context.Context) error {
	select {
	case <-s.rec:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

// VmReady is the implementation of the StartupListenerServer gRPC stub. It
// sends a signal through an empty channel when a message is received.
// Note: golint flags this function because Vm is not fully capatalized, however
// this is due to the proto definition.
func (s *StartupListenerServer) VmReady(ctx context.Context, msg *vmtools.EmptyMessage) (*vmtools.EmptyMessage, error) {
	s.rec <- struct{}{}
	return &vmtools.EmptyMessage{}, nil
}
