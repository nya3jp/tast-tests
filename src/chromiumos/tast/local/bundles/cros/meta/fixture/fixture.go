// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package fixture contains fixture implementations used in meta tests.
package fixture

import (
	"context"

	"chromiumos/tast/errors"
	"chromiumos/tast/testing"
)

type parent struct {
	resetCnt int
}

func (f *parent) SetUp(ctx context.Context, s *testing.FixtState) interface{} {
	s.Log("meta.Parent: SetUp")
	return "meta.Parent"
}

func (f *parent) Reset(ctx context.Context) error {
	f.resetCnt++
	if f.resetCnt%2 == 0 {
		testing.ContextLog(ctx, "meta.Parent: Reset (Error)")
		return errors.New("failed to reset")
	}
	testing.ContextLog(ctx, "meta.Parent: Reset (OK)")
	return nil
}

func (f *parent) PreTest(ctx context.Context, s *testing.FixtTestState) {
	s.Log("meta.Parent: PreTest")
}

func (f *parent) PostTest(ctx context.Context, s *testing.FixtTestState) {
	s.Log("meta.Parent: PostTest")
}

func (f *parent) TearDown(ctx context.Context, s *testing.FixtState) {
	s.Log("meta.Parent: TearDown")
}

type child struct {
	name string
}

func (f *child) SetUp(ctx context.Context, s *testing.FixtState) interface{} {
	s.Logf("%s: SetUp", f.name)
	return f.name
}

func (f *child) Reset(ctx context.Context) error {
	testing.ContextLogf(ctx, "%s: Reset", f.name)
	return nil
}

func (f *child) PreTest(ctx context.Context, s *testing.FixtTestState) {
	s.Logf("%s: PreTest", f.name)
}

func (f *child) PostTest(ctx context.Context, s *testing.FixtTestState) {
	s.Logf("%s: PostTest", f.name)
}

func (f *child) TearDown(ctx context.Context, s *testing.FixtState) {
	s.Logf("%s: TearDown", f.name)
}

func init() {
	testing.AddFixture(&testing.Fixture{
		Name: "meta.Parent",
		Impl: &parent{},
	})
	testing.AddFixture(&testing.Fixture{
		Name:   "meta.Child1",
		Impl:   &child{name: "meta.Child1"},
		Parent: "meta.Parent",
	})
	testing.AddFixture(&testing.Fixture{
		Name:   "meta.Child2",
		Impl:   &child{name: "meta.Child2"},
		Parent: "meta.Parent",
	})
}
