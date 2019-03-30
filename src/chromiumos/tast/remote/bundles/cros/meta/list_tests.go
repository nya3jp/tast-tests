// Copyright 2018 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package meta

import (
	"context"
	"encoding/json"
	"reflect"
	"strings"

	"chromiumos/tast/remote/bundles/cros/meta/tastrun"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:     ListTests,
		Desc:     "Verifies that the tast command can list tests",
		Contacts: []string{"derat@chromium.org", "tast-users@chromium.org"},
	})
}

func ListTests(ctx context.Context, s *testing.State) {
	// This test executes tast with -build=false to run already-installed copies of these helper tests.
	// If it is run manually with "tast run -build=true", the tast-remote-tests-cros package should be
	// built for the host and tast-local-tests-cros should be deployed to the DUT first.
	testNames := []string{"meta.LocalFiles", "meta.RemoteFiles"}
	stdout, stderr, err := tastrun.Run(ctx, s, "list", []string{"-build=false", "-json"}, testNames)
	if err != nil {
		lines := strings.Split(strings.TrimSpace(string(stderr)), "\n")
		s.Fatalf("Failed to run tast: %v (last line: %q)", err, lines[len(lines)-1])
	}

	var tests []testing.Test
	if err := json.Unmarshal(stdout, &tests); err != nil {
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
