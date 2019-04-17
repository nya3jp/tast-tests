// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package firmware

import (
	"context"
	"strings"

	"chromiumos/tast/dut"
	"chromiumos/tast/remote/servo"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:     CheckEC,
		Desc:     "Check EC console enable or not",
		Contacts: []string{"jeffcarp@chromium.org", "derat@chromium.org", "tast-users@chromium.org"},
		Attr:     []string{"disabled", "informational"},
	})
}

//CheckEC : Check the EC console enable or disable
func CheckEC(ctx context.Context, s *testing.State) {
	d, ok := dut.FromContext(ctx)
	if !ok {
		s.Fatal("Failed to get DUT")
	}

	svo, err := servo.Default(ctx)
	if err != nil {
		s.Fatal("Servo init error: ", err)
	}
	var regexps = []string{"Chip:\\s+([^\\r\\n]*)\\r\\n", "RO:\\s+([^\\r\\n]*)\\r\\n", "RW_?[AB]?:\\s+([^\\r\\n]*)\\r\\n", "Build:\\s+([^\\r\\n]*)\\r\\n"}
	val, err := svo.SendEcCommandAndGetOutput(ctx, "version", regexps)
	if err != nil {
		s.Fatalf("Failed in SendEcCommandAndGetOutput, command is %s. Error is %v", "version", err)
	}
	if val == "None" {
		s.Error("EC console not enabled")
	}
	s.Logf("Ec version is %s", val)
	output, err := d.Run(ctx, "mosys ec info")
	if err != nil {
		s.Error("Failed to execute 'mosys ec info' command")
	}
	mosysOutput := strings.Trim(string(output), "\n\r\t ")
	s.Logf("mosys ec info is %s", strings.Split(mosysOutput, " ")[len(strings.Split(mosysOutput, " "))-1])
}
