// Copyright 2018 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package meta

import (
	"encoding/json"
	"reflect"

	"chromiumos/tast/remote/bundles/cros/meta/tastrun"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func: ListTests,
		Desc: "Verifies that the tast command can list tests",
	})
}

func ListTests(s *testing.State) {
	// This test executes tast with -build=false to run already-installed copies of these helper tests.
	// If it is run manually with "tast run -build=true", the tast-remote-tests-cros package should be
	// built for the host and tast-local-tests-cros should be deployed to the DUT first.
	testNames := []string{"meta.LocalFiles", "meta.RemoteFiles"}
	out, err := tastrun.Run(s, "list", []string{"-build=false", "-json"}, testNames)
	if err != nil {
		s.Fatal("Failed to run tast: ", err)
	}

	var tests []testing.Test
	if err := json.Unmarshal(out, &tests); err != nil {
		s.Fatal("Failed to unmarshal listed tests: ", err)
	}
	actTestNames := make([]string, len(tests))
	for i, t := range tests {
		actTestNames[i] = t.Name
	}
	if !reflect.DeepEqual(actTestNames, testNames) {
		s.Errorf("Got tests %v; want %v", actTestNames, testNames)
	}
}
