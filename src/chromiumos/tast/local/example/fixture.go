// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package example is used to provide examples for new functionality.
package example

import (
	"context"
	"time"

	"chromiumos/tast/testing"
)

const fixtureTimeout = time.Second
const varName = "example.AccessVars.FixtureVar"

func init() {
	testing.AddFixture(&testing.Fixture{
		Name:            "exampleFixtureWithVar",
		Desc:            "An example to use var in fixture",
		Contacts:        []string{"seewaifu@chromium.org"},
		Impl:            &varInFixture{name: varName},
		Vars:            []string{varName},
		SetUpTimeout:    fixtureTimeout,
		ResetTimeout:    fixtureTimeout,
		TearDownTimeout: fixtureTimeout,
	})

}

// VarData holds value of the variable that specify this Fixture.
type VarData struct {
	Name string
	Val  string
}

// varInFixture is a fixture to start Chrome with a runtime variable.
type varInFixture struct {
	name string
}

func (f *varInFixture) SetUp(ctx context.Context, s *testing.FixtState) interface{} {
	return &VarData{Name: f.name, Val: s.RequiredVar(f.name)}
}

func (f *varInFixture) TearDown(ctx context.Context, s *testing.FixtState) {}

func (f *varInFixture) Reset(ctx context.Context) error {
	return nil
}

func (f *varInFixture) PreTest(ctx context.Context, s *testing.FixtTestState) {}

func (f *varInFixture) PostTest(ctx context.Context, s *testing.FixtTestState) {}
