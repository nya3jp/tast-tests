// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package example

import (
	"context"
	"strings"
	"time"

	"chromiumos/tast/testing"
)

type helloPre struct{ path string }

var helloPreVal = &helloPre{}

// HelloPre returns a precondition to prepare a file with the word "Hello" written on it.
func HelloPre() testing.Precondition {
	return helloPreVal
}
func (*helloPre) String() string {
	return "example_hello_pre"
}
func (*helloPre) Timeout() time.Duration {
	return 1 * time.Minute
}

// Prepare does the preparation documented in HelloPre(), and returns the remote filepath.
func (p *helloPre) Prepare(ctx context.Context, s *testing.State) interface{} {
	d := s.DUT()
	if p.path == "" {
		b, err := d.Command("mktemp").Output(ctx)
		if err != nil {
			s.Fatal("prepare: ", err)
		}
		p.path = string(b)
	}
	cmd := d.Command("tee", p.path)
	cmd.Stdin = strings.NewReader("Hello\n")
	if err := cmd.Run(ctx); err != nil {
		s.Fatal("prepare: ", err)
	}
	return p.path
}
func (p *helloPre) Close(ctx context.Context, s *testing.State) {
	d := s.DUT()
	if err := d.Command("rm", p.path).Run(ctx); err != nil {
		s.Error("close: ", err)
	}
}
