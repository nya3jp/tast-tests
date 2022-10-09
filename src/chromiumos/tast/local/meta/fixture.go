// Copyright 2021 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package meta contains support code for Tast meta tests.
package meta

import (
	"context"
	"path/filepath"
	"reflect"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"

	"chromiumos/tast/framework/protocol"
	"chromiumos/tast/fsutil"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddFixture(&testing.Fixture{
		Name:     "metaLocalDataFilesFixture",
		Desc:     "Demonstrate how to use data files in fixtures",
		Contacts: []string{"oka@chromium.org", "tast-owner@google.com"},
		Data: []string{
			"fixture_data_internal.txt",
			"fixture_data_external.txt",
		},
		Impl: dataFileFixture{},
	})
	testing.AddFixture(&testing.Fixture{
		Name:     "metaLocalFixtureDUTFeature",
		Desc:     "Demonstrate how to access DUT Features in fixtures",
		Contacts: []string{"seewaifu@chromium.org", "tast-owner@google.com"},
		Data:     []string{},
		Impl:     &dutFeatureFixture{},
	})
}

type dataFileFixture struct{}

func (dataFileFixture) SetUp(ctx context.Context, s *testing.FixtState) interface{} {
	for _, fn := range []string{
		"fixture_data_internal.txt",
		"fixture_data_external.txt",
	} {
		s.Log("Copying ", fn)
		if err := fsutil.CopyFile(s.DataPath(fn), filepath.Join(s.OutDir(), fn)); err != nil {
			s.Errorf("Failed copying %s: %s", fn, err)
		}
	}
	return nil
}
func (dataFileFixture) Reset(ctx context.Context) error {
	return nil
}
func (dataFileFixture) PreTest(ctx context.Context, s *testing.FixtTestState)  {}
func (dataFileFixture) PostTest(ctx context.Context, s *testing.FixtTestState) {}
func (dataFileFixture) TearDown(ctx context.Context, s *testing.FixtState)     {}

type dutFeatureFixture struct {
	feature *protocol.DUTFeatures
}

func (dff *dutFeatureFixture) SetUp(ctx context.Context, s *testing.FixtState) interface{} {
	dff.feature = s.Features("")
	return dff.feature
}
func (dff *dutFeatureFixture) Reset(ctx context.Context) error {
	return nil
}
func (dff *dutFeatureFixture) PreTest(ctx context.Context, s *testing.FixtTestState) {
	feature := s.Features("")
	allowUnexported := func(reflect.Type) bool { return true }
	if diff := cmp.Diff(feature, dff.feature, cmpopts.EquateEmpty(), cmp.Exporter(allowUnexported)); diff != "" {
		s.Logf("Got unexpected feature in PreTest (-got +want): %s", diff)
		s.Fatal("Got unexpected feature in PreTest")
	}
}
func (dff *dutFeatureFixture) PostTest(ctx context.Context, s *testing.FixtTestState) {
	feature := s.Features("")
	allowUnexported := func(reflect.Type) bool { return true }
	if diff := cmp.Diff(feature, dff.feature, cmpopts.EquateEmpty(), cmp.Exporter(allowUnexported)); diff != "" {
		s.Logf("Got unexpected feature in PostTest (-got +want): %s", diff)
		s.Fatal("Got unexpected feature in PostTest")
	}
}
func (dff *dutFeatureFixture) TearDown(ctx context.Context, s *testing.FixtState) {
	feature := s.Features("")
	allowUnexported := func(reflect.Type) bool { return true }
	if diff := cmp.Diff(feature, dff.feature, cmpopts.EquateEmpty(), cmp.Exporter(allowUnexported)); diff != "" {
		s.Logf("Got unexpected feature in TearDown (-got +want): %s", diff)
		s.Fatal("Got unexpected feature in TearDown")
	}
}
