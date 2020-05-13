// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package meta

import (
	"context"
	"io/ioutil"
	"path/filepath"

	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:     LocalVars,
		Desc:     "Helper test that inspects a runtime variable",
		Contacts: []string{"nya@chromium.org", "tast-owners@google.com"},
		Vars:     []string{"meta.LocalVars.var"},
		// This test is called by remote tests in the meta package.
	})
}

func LocalVars(ctx context.Context, s *testing.State) {
	p := filepath.Join(s.OutDir(), "var.txt")
	if err := ioutil.WriteFile(p, []byte(s.RequiredVar("meta.LocalVars.var")), 0644); err != nil {
		s.Error("Failed to write variable: ", err)
	}
}
