// Copyright 2018 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package ui

import (
	"context"
	"time"

	"chromiumos/tast/local/arc"
	"chromiumos/tast/testing"
)

type Devicer interface {
	Device() *Device
}

type readyPre struct {
	arcReady testing.Precondition
	dev      *Device
}

func (p *readyPre) Device() *Device {
	return p.dev
}

func (p *readyPre) Prepare(ctx context.Context, s *testing.State) {
	if s.RunningTest() {
		s.Fatal("Tests cannot call Prepare")
	}

	p.arcReady.Prepare(ctx, s)

	if p.healthy() {
		return
	}

	p.closeInternal(ctx, s)

	var err error
	if p.dev, err = NewDevice(ctx, arc.Get(s)); err != nil {
		s.Fatal("Failed creating Device: ", err)
	}
}

func (p *readyPre) healthy() bool {
	if p.dev == nil {
		return false
	}
	// TODO: implement
	return true
}

func (p *readyPre) Close(ctx context.Context, s *testing.State) {
	if s.RunningTest() {
		s.Fatal("Tests cannot call Close")
	}
	p.closeInternal(ctx, s)
}

func (p *readyPre) closeInternal(ctx context.Context, s *testing.State) {
	if p.dev != nil {
		if err := p.dev.Close(); err != nil {
			s.Error("Failed to close Device: ", err)
		}
		p.dev = nil
	}
}

func (p *readyPre) Timeout() time.Duration {
	return StartTimeout
}

func (p *readyPre) String() string {
	return "arc_ui"
}

var ready = readyPre{arcReady: arc.Ready()}

func Ready() testing.Precondition {
	return &ready
}

func GetDevice(s *testing.State) *Device {
	if a, ok := s.Pre().(Devicer); ok {
		return a.Device()
	}
	s.Fatalf("No Device in precondition: %#v", s.Pre())
	return nil
}
