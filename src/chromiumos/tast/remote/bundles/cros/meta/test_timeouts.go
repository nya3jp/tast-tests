// Copyright 2020 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package meta

import (
	"context"
	"fmt"
	"strings"
	"time"

	"chromiumos/tast/remote/bundles/cros/meta/tastrun"
	"chromiumos/tast/testing"
)

var (
	tests = []string{
		"meta.LocalFail",
		"meta.LocalPass",
		"meta.RemoteFail",
		"meta.RemotePass",
	}
)

type timeoutParams struct {
	tests   []string
	timeout time.Duration
}

func init() {
	testing.AddTest(&testing.Test{
		Func:     TestTimeouts,
		Desc:     "Verifies that we pass the test timeouts correctly.",
		Contacts: []string{"tast-owners@google.com"},
		Params: []testing.Param{
			{
				Name: "default",
				Val:  timeoutParams{timeout: 120 * time.Second},
			},
		},
	})
}

func TestTimeouts(ctx context.Context, s *testing.State) {
	param := s.Param().(timeoutParams)
	var flags []string
	if param.timeout != 0 {
		f := fmt.Sprintf("%f", param.timeout.Seconds())
		flags = append(flags, "-systemservicestimeout="+f)

	}
	stdout, _, err := tastrun.Exec(ctx, s, "run", flags, tests)
	if err != nil {
		lines := strings.Split(strings.TrimSpace(string(stdout)), "\n")
		s.Fatalf("Failed to run tast: %v (last line: %q)", err, lines[len(lines)-1])
	}
}
