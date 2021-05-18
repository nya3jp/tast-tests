// Copyright 2017 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package example

import (
	"context"
	"io/ioutil"
	"strings"

	"chromiumos/tast/testing"
)

func init() {
	testing.AddFixture(&testing.Fixture{
		Name:     "dataFileFixture",
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
	b, err := ioutil.ReadFile(s.DataPath("fixture_data_internal.txt"))
	if err != nil {
		s.Error("Failed to read internal data file")
	} else {
		s.Log("Read internal data: ", strings.TrimRight(string(b), "\n"))
	}

	b, err = ioutil.ReadFile(s.DataPath("fixture_data_external.txt"))
	if err != nil {
		s.Error("Failed to read external data file")
	}
	s.Log("Read external data: ", strings.TrimRight(string(b), "\n"))
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
		Func:     DataFiles,
		Desc:     "Demonstrates how to use data files",
		Contacts: []string{"nya@chromium.org", "tast-owners@google.com"},
		Data: []string{
			"data_files_internal.txt",
			"data_files_external.txt",
		},
		Attr:    []string{"group:mainline"},
		Fixture: "dataFileFixture",
	})
}

func DataFiles(ctx context.Context, s *testing.State) {
	// Read a data file that's directly checked in to this repository in the data/ subdirectory.
	b, err := ioutil.ReadFile(s.DataPath("data_files_internal.txt"))
	if err != nil {
		s.Error("Failed reading internal data file: ", err)
	} else {
		s.Logf("Read internal data file: %q", strings.TrimRight(string(b), "\n"))
	}

	// Read a data file that's stored in Google Cloud Storage and linked by an external link
	// file (*.external) in the data/ subdirectory.
	if b, err = ioutil.ReadFile(s.DataPath("data_files_external.txt")); err != nil {
		s.Error("Failed reading external data file: ", err)
	} else {
		s.Logf("Read external data file: %q", strings.TrimRight(string(b), "\n"))
	}
}
