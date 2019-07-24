// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package network

import (
	"context"
	"fmt"
	"strings"
	"time"

	"chromiumos/tast/local/testexec"
	"chromiumos/tast/local/upstart"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:     TwoShills,
		Desc:     "Verifies that only a single instance of shill can run",
		Contacts: []string{"billyzhao@google.com", "chromeos-kernel-wifi@google.com"},
		Attr:     []string{"informational"},
	})
}

func TwoShills(ctx context.Context, s *testing.State) {
	const timeout time.Duration = 10
	_, _, pid, err := upstart.JobStatus(ctx, "shill")
	if err != nil {
		s.Fatal("Failed to find shill job: ", err)
	} else if pid == 0 {
		s.Fatal("Shill is not running")
	}
	out, err := testexec.CommandContext(ctx, "ip", "route", "show", "default", "match", "0/0", "table", "0").Output(testexec.DumpLogOnError)
	netdev := strings.Split(string(out), " ")[4]
	if len(netdev) < 1 {
		s.Fatal("Unable to determine default network device")
	}
	ctx, cancel := context.WithTimeout(ctx, time.Second)
	defer cancel()
	if err := testexec.CommandContext(ctx, "shill", "--foreground",
		fmt.Sprintf("--device-black-list=%s", netdev)).Run(testexec.DumpLogOnError); err != nil {
		if ctx.Err() == context.DeadlineExceeded {
			s.Fatal("Request to start shill failed: ", err)
		}
	}
}
