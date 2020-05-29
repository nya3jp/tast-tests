// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package meta

import (
	"context"
	"fmt"
	"os"

	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:     Stdio,
		Desc:     "Ensures that accessing stdin/stdout does not harm test execution",
		Contacts: []string{"nya@chromium.org", "tast-owners@google.com"},
		Attr:     []string{"group:mainline"},
	})
}

func Stdio(ctx context.Context, s *testing.State) {
	var r string
	if _, err := fmt.Scan(&r); err == nil {
		s.Error("fmt.Scan succeeded unexpectedly")
	}
	if _, err := fmt.Print("foo"); err == nil {
		s.Error("fmt.Print succeeded unexpectedly")
	}
	if _, err := fmt.Fprint(os.Stderr, "foo"); err == nil {
		s.Error("fmt.Fprint succeeded unexpectedly")
	}
}
