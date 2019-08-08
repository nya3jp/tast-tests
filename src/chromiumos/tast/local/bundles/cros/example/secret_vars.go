// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package example

import (
	"context"

	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func: SecretVars,
		// Document: https://chromium.googlesource.com/chromiumos/platform/tast/+/HEAD/docs/writing_tests.md#secret-variables
		Desc:     "Secret variables",
		Contacts: []string{"tast-owners@google.com", "oka@chromium.org"},
		Attr:     []string{"group:mainline", "informational"},
		// example.SecretVars.password is defined in tast-tests-private/vars/example.SecretVars.yaml
		// example.commonVar is defined in tast-tests-private/vars/example.yaml
		Vars: []string{"example.SecretVars.password", "example.commonVar"},
	})
}

func SecretVars(ctx context.Context, s *testing.State) {
	if x := s.RequiredVar("example.SecretVars.password"); x != "passw0rd" {
		// Note: Don't log secrets in real tests.
		s.Errorf(`Got %q, want "passw0rd"`, x)
	}
	if s.RequiredVar("example.commonVar") == "" {
		s.Error("example.commonVar is unexpectedly empty")
	}
}
