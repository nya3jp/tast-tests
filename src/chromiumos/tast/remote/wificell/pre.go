// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package wificell

import (
	"context"
	"time"

	"chromiumos/tast/rpc"
	"chromiumos/tast/services/cros/network"
	"chromiumos/tast/testing"
)

type logPreImpl struct {
	rpc *rpc.Client
	cl  network.LogClient
}

var testLogPre = &logPreImpl{}

func TestLogPre() testing.Precondition {
	return testLogPre
}

// String returns a short, underscore-separated name for the precondition.
func (p *logPreImpl) String() string {
	return "log_pre"
}

// Timeout returns the amount of time dedicated to prepare and close the precondition.
func (p *logPreImpl) Timeout() time.Duration {
	return 15 * time.Second
}

func (p *logPreImpl) Prepare(ctx context.Context, s *testing.State) interface{} {
	if p.cl != nil {
		return p.cl
	}
	rpc, err := rpc.Dial(s.PreCtx(), s.DUT(), s.RPCHint(), "cros")
	if err != nil {
		s.Fatal("Failed to dial rpc: ", err)
	}
	p.rpc = rpc
	p.cl = network.NewLogClient(p.rpc.Conn)
	return p.cl
}

func (p *logPreImpl) Close(ctx context.Context, s *testing.State) {
	if p.rpc != nil {
		p.rpc.Close(ctx)
	}
}
