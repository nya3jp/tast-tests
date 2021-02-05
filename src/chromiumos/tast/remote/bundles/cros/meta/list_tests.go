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
		Contacts: []string{"nya@chromium.org", "tast-owners@google.com"},
		Attr:     []string{"group:mainline", "group:meta"},
	})
}

func ListTests(ctx context.Context, s *testing.State) {
	testNames := []string{"meta.LocalFiles", "meta.RemoteFiles"}
	stdout, stderr, err := tastrun.Exec(ctx, s, "list", []string{"-json"}, testNames)
	if err != nil {
		lines := strings.Split(strings.TrimSpace(string(stderr)), "\n")
		s.Fatalf("Failed to run tast: %v (last line: %q)", err, lines[len(lines)-1])
	}

	var tests []testing.TestInstance
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
