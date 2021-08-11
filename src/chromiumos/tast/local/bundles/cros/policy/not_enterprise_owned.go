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
		Func: NotEnterpriseOwned,
		Desc: "Check that the DUT is not enterprise owned",
		Contacts: []string{
			"vsavu@google.com", // Test author
			"nya@chromium.org",
			"chromeos-commercial-remote-management@google.com",
		},
		Attr: []string{"group:mainline"},
	})
}

func NotEnterpriseOwned(ctx context.Context, s *testing.State) {
	contents, err := ioutil.ReadFile(ready.EnterpriseOwnedLogLocation)
	if os.IsNotExist(err) {
		return // Test passed as error log is missing.
	}

	if err != nil {
		s.Fatalf("Failed to read error log %q: %v", ready.ClearPoliciesLogLocation, err)
	}

	for _, line := range strings.Split(string(contents), "\n") {
		s.Errorf("Found error log for ready.go: %s", line)
	}
}
