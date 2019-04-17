// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package firmware

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	"chromiumos/tast/dut"
	"chromiumos/tast/remote/servo"
	"chromiumos/tast/testing"
)

const (
	ecUARTCmd = "ec_uart_cmd"
	ecRegexp  = "ec_uart_regexp"
)

func init() {
	testing.AddTest(&testing.Test{
		Func: ECSanity,
		Desc: "Check Google EC support",
		Contacts: []string{
			"ningappa.tirakannavar@intel.com",
			"kasaiah.bogineni@intel.com",
		},
		Attr: []string{"informational"},
	})
}

// uartCmd sends the given command to EC console.
func uartCmd(ctx context.Context, s *servo.Servo, command string) error {
	uartRegexp(ctx, s, []string{})
	if err := s.SetNoCheck(ctx, ecUARTCmd, command); err != nil {
		return err
	}
	return nil
}

// uartRegexp sets the regexps.
func uartRegexp(ctx context.Context, s *servo.Servo, patterns []string) {
	var expr string
	if len(patterns) == 0 {
		expr = "None"
	} else {
		// Convert Go string array to Python string array literal.
		var escaped []string
		for _, r := range patterns {
			escaped = append(escaped, strconv.Quote(r))
		}
		expr = fmt.Sprintf("[%s]", strings.Join(escaped, ", "))
	}
	s.SetNoCheck(ctx, ecUARTCmd, expr)

}

// ecOutput accepts the EC command and paterns, gives the output.
func ecOutput(ctx context.Context, s *servo.Servo, command string, patterns []string) (string, error) {
	uartRegexp(ctx, s, patterns)
	s.SetNoCheck(ctx, ecUARTCmd, command)
	ecoutput, err := s.Get(ctx, ecUARTCmd)
	if err != nil {
		return "", err
	}
	return ecoutput, nil
}

func ECSanity(ctx context.Context, s *testing.State) {
	d, ok := dut.FromContext(ctx)
	if !ok {
		s.Fatal("failed to get DUT")
	}

	svo, err := servo.Default(ctx)
	if err != nil {
		s.Fatal("failed to obtain servo: ", err)
	}
	regexps := []string{`Chip:\s+([^\r\n]*)\r\n`, `RO:\s+([^\r\n]*)\r\n`, `RW_?[AB]?:\s+([^\r\n]*)\r\n`, `Build:\s+([^\r\n]*)\r\n`}
	val, err := ecOutput(ctx, svo, "version", regexps)
	if err != nil {
		s.Fatal("failed to get version: ", err)
	}
	s.Logf("EC version is %q", val)
	if val == "None" {
		s.Error("ec console not enabled")
	}
	if _, err := d.Run(ctx, "ectool version"); err != nil {
		s.Error("no support for Google EC: ", err)
	}
}
