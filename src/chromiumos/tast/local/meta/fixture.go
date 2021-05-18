// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package meta contains support code for Tast meta tests.
package meta

import (
	"context"
	"path/filepath"

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
