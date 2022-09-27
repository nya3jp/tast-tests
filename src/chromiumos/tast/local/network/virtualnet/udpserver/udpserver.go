// Copyright 2022 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package udpserver provides a simple UDP server running inside a virtualnet.Env.
package udpserver

import (
	"context"
	"fmt"
	"net"
	"os"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/network/virtualnet/env"
	"chromiumos/tast/testing"
)

const logPath = "/tmp/udpserver.log"

// New creates a new UDP server. The returned object should be passed to Env.StartServer;
// its lifetime will be managed by the environment. msgLen is the size (in bytes) of the
// internal receive buffer.
func New(fam Family, port, msgLen int, handler MsgHandler) *server {
	return &server{
		fam:     fam.String(),
		port:    port,
		msgLen:  msgLen,
		handler: handler,
	}
}

// MsgHandler defines a function for processing incoming messages and emitting replies.
// If this function returns nil, no reply will be sent.
type MsgHandler func(in []byte) []byte

// Reflector is a type of MsgHandler that simply replies to any message with the same.
func Reflector() MsgHandler {
	return func(in []byte) []byte {
		out := make([]byte, len(in))
		copy(out, in)
		return out
	}
}

// Family describes the IP family for the server.
type Family int

func (f Family) String() string {
	return [...]string{"udp", "udp4", "udp6"}[f]
}

const (
	// UDP corresponds to "udp" as documented in the net package.
	UDP Family = iota
	// UDP4 corresponds to "udp4" as documented in the net package.
	UDP4
	// UDP6 corresponds to "udp6" as documented in the net package.
	UDP6
)

type server struct {
	fam     string
	port    int
	msgLen  int
	conn    *net.UDPConn
	handler MsgHandler
	env     *env.Env
	run     bool
}

// Start enters the netns of the virtualnet environment, starts listening on the desired port,
// and runs the handling loop until Stop is called.
func (s *server) Start(ctx context.Context, env *env.Env) error {
	if s.run {
		return errors.Errorf("%s server already running", s.fam)
	}
	ec := make(chan error)

	go func() {
		cleanup, err := env.EnterNetNS(ctx)
		if err != nil {
			ec <- errors.Wrapf(err, "failed to enter ns %s", env.NetNSName)
			return
		}
		defer cleanup()

		addr, err := net.ResolveUDPAddr(s.fam, fmt.Sprintf(":%d", s.port))
		if err != nil {
			ec <- errors.Wrapf(err, "failed to resolve %s addr", s.fam)
			return
		}
		s.conn, err = net.ListenUDP(s.fam, addr)
		if err != nil {
			ec <- errors.Wrapf(err, "failed to listen on %s network", s.fam)
			return
		}
		ec <- nil
		s.env = env
		s.run = true
		testing.ContextLogf(ctx, "%s server running", s.fam)

		in := make([]byte, s.msgLen)
		for {
			if !s.run {
				return
			}
			_, addr, err := s.conn.ReadFromUDP(in)
			if err != nil {
				if s.run {
					testing.ContextLogf(ctx, "%s read failed: %v", s.fam, err)
				}
				continue
			}
			out := s.handler(in)
			if out == nil {
				continue
			}
			if _, err := s.conn.WriteToUDP(out, addr); err != nil {
				if s.run {
					testing.ContextLogf(ctx, "%s write failed: %v", s.fam, err)
				}
				continue
			}
		}
	}()
	return <-ec
}

// Stops shuts down the handling loop.
func (s *server) Stop(ctx context.Context) error {
	s.run = false
	s.conn.Close()
	return nil
}

// WriteLogs writres logs to the given file.
func (s *server) WriteLogs(_ context.Context, f *os.File) error {
	return s.env.ReadAndWriteLogIfExists(s.env.ChrootPath(logPath), f)
}
