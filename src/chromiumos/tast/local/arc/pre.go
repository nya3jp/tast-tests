// Copyright 2018 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package arc

import (
	"context"
	"time"

	"chromiumos/tast/local/chrome"
	"chromiumos/tast/testing"
)

type ARCer interface {
	ARC() *ARC
}

type readyPre struct {
	c *chrome.Chrome // TODO: Use Chrome precondition
	a *ARC
}

func (p *readyPre) ARC() *ARC {
	return p.a
}

func (p *readyPre) Prepare(ctx context.Context, s *testing.State) {
	if s.RunningTest() {
		s.Fatal("Tests cannot call Prepare")
	}

	if p.healthy() {
		return
	}

	p.closeInternal(ctx, s)

	var err error
	if p.c, err = chrome.New(ctx, chrome.ARCEnabled()); err != nil {
		s.Fatal("Failed creating Chrome: ", err)
	}
	if p.a, err = New(ctx, s.OutDir()); err != nil {
		s.Fatal("Failed creating ARC: ", err)
	}
}

func (p *readyPre) healthy() bool {
	if p.c == nil || p.a == nil {
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
	if p.a != nil {
		if err := p.a.Close(); err != nil {
			s.Error("Failed to close ARC: ", err)
		}
		p.a = nil
	}
	if p.c != nil {
		if err := p.c.Close(ctx); err != nil {
			s.Error("Failed to close Chrome: ", err)
		}
		p.c = nil
	}
}

func (p *readyPre) Timeout() time.Duration {
	return BootTimeout
}

func (p *readyPre) String() string {
	return "arc"
}

var ready readyPre

func Ready() testing.Precondition {
	return &ready
}

func Get(s *testing.State) *ARC {
	if a, ok := s.Pre().(ARCer); ok {
		return a.ARC()
	}
	s.Fatalf("No ARC in precondition: %#v", s.Pre())
	return nil
}
