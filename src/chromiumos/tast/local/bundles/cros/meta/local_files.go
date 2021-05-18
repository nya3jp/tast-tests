// Copyright 2018 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package meta

import (
	"context"
	"path/filepath"

	"chromiumos/tast/fsutil"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddFixture(&testing.Fixture{
		Name:     "metaDataFilesFixture",
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

func init() {
	testing.AddTest(&testing.Test{
		Func:     LocalFiles,
		Desc:     "Helper test that uses data and output files",
		Contacts: []string{"nya@chromium.org", "tast-owners@google.com"},
		Data: []string{
			"local_files_internal.txt",
			"local_files_external.txt",
		},
		Fixture: "metaDataFilesFixture",
		// This test is executed by remote tests in the meta package.
	})
}

func LocalFiles(ctx context.Context, s *testing.State) {
	for _, fn := range []string{
		"local_files_internal.txt",
		"local_files_external.txt",
	} {
		s.Log("Copying ", fn)
		if err := fsutil.CopyFile(s.DataPath(fn), filepath.Join(s.OutDir(), fn)); err != nil {
			s.Errorf("Failed copying %s: %s", fn, err)
		}
	}
}
