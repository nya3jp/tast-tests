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
		Attr: []string{"informational"},
	})
}

func ListTests(s *testing.State) {
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
