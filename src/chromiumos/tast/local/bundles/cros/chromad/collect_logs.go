// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package chromad implements utilities for testing Chromad:
// ChromeOS Active Directory integration.
package chromad

import (
	"context"
	"strconv"
	"strings"

	"chromiumos/tast/local/arc"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/testing"
)

// cmdVars represents the configurable parameters from command line
type cmdVars struct {
	cred       chrome.ChromadCred
	user, pass string
	reset      bool
	extraArgs  []string
}

func init() {
	// Example usage:
	// $ tast run -var=user=<username> -var=pass=<password> <dut ip> chromad.ChromadFlow
	// <username> and <password> are the credentials of the test GAIA account.
	testing.AddTest(&testing.Test{
		Func:         CollectLogs,
		Desc:         "Perform Chromad enrollment and login flow and collect logs",
		Contacts:     []string{"tomdobro@chromium.org", "tast-users@chromium.org"},
		SoftwareDeps: []string{"chrome"},
		Vars: []string{
			"enrollUser", "enrollPass", "domainUser", "domainPass",
			"user", "pass", "extra_args", "reset",
		},
		Params: []testing.Param{{
			Name: "",
		}},
	})
}

// optionalVar returns value of the variable given its name or if variable
// is not defined returns the default value.
// TODO(tomdobro): move to internal testing utils?
func optionalVar(s *testing.State, name, defaultValue string) string {
	val, ok := s.Var(name)
	if !ok {
		return defaultValue
	}
	return val
}

// getVars extracts the testing parameters from testing.State. The user
// provided credentials would override the credentials from config file.
func getVars(s *testing.State) cmdVars {
	var cred chrome.ChromadCred
	cred.EnrollUser = optionalVar(s, "enrollUser", "admin@chromad-muc-tier15.deviceadmin.goog")
	cred.EnrollPass = optionalVar(s, "enrollPass", "test765!")
	cred.DomainUser = optionalVar(s, "domainUser", "feiling@chromad.cros-kir.com")
	cred.DomainPass = optionalVar(s, "domainPass", "ad3pky001!")
	user := optionalVar(s, "user", "test@chromeadm-lab.com")
	pass := optionalVar(s, "pass", "apql765!")

	resetStr, ok := s.Var("reset")
	if !ok {
		resetStr = "false"
	}
	reset, err := strconv.ParseBool(resetStr)
	if err != nil {
		s.Fatal("Failed to parse the variable `reset`: ", err)
	}

	extraArgsStr, ok := s.Var("extra_args")
	if !ok {
		extraArgsStr = ""
	}
	extraArgs := strings.Fields(extraArgsStr)

	return cmdVars{
		cred:      cred,
		user:      user,
		pass:      pass,
		reset:     reset,
		extraArgs: extraArgs,
	}
}

func CollectLogs(ctx context.Context, s *testing.State) {
	vars := getVars(s)
	var opts []chrome.Option

	chromeARCOpt := chrome.ARCDisabled()
	if arc.Supported() {
		chromeARCOpt = chrome.ARCSupported()
	}
	opts = append(opts, chromeARCOpt)
	opts = append(opts, chrome.EnableChromad(vars.cred))
	opts = append(opts, chrome.Auth(vars.user, vars.pass, ""))
	// opts = append(opts, chrome.GAIALogin())

	if !vars.reset {
		opts = append(opts, chrome.KeepState())
	}

	if len(vars.extraArgs) > 0 {
		opts = append(opts, chrome.ExtraArgs(vars.extraArgs...))
	}

	cr, err := chrome.New(ctx, opts...)
	if err != nil {
		s.Fatal("Failed to start Chrome: ", err)
	}
	defer cr.Close(ctx)
}
