// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package policy

import (
	"context"
	"io/ioutil"
	"os"

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
	if _, err := os.Stat(ready.ClearPoliciesLogLocation); err == nil {
		// No error means file found and we need to report failure.
		contents, err := ioutil.ReadFile(ready.ClearPoliciesLogLocation)

		if err != nil {
			s.Fatalf("Failed to open %q: %v", ready.ClearPoliciesLogLocation, err)
		}

		s.Errorf("Found error log for ready.go: %s", string(contents))
	} else if !os.IsNotExist(err) {
		s.Fatalf("Failed to check for the existence of %q", ready.ClearPoliciesLogLocation)
	}
}
