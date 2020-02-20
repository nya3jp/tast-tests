// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package policy

import (
	"context"
	"io/ioutil"
	"os"
	"strings"

	"chromiumos/tast/local/ready"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func: ReadyErrorLog,
		Desc: "Check that ready.go successfully removed existing policies",
		Contacts: []string{
			"vsavu@google.com", // Test author
			"nya@chromium.org",
		},
		SoftwareDeps: []string{},
		Attr:         []string{"group:mainline", "informational"},
	})
}

func ReadyErrorLog(ctx context.Context, s *testing.State) {
	contents, err := ioutil.ReadFile(ready.ClearPoliciesLogLocation)
	if err != nil {
		if os.IsNotExist(err) {
			return // Test passed as error log is missing.
		}

		s.Fatalf("Failed to read error log %q", ready.ClearPoliciesLogLocation)
	}

	for _, err := range strings.Split(string(contents), "\n") {
		s.Errorf("Found error log for ready.go: %s", err)
	}
}
