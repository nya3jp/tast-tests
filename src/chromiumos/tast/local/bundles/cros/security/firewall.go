// Copyright 2018 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package security

import (
	"context"
	"io/ioutil"
	"path/filepath"
	"strings"

	"chromiumos/tast/local/testexec"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func: Firewall,
		Desc: "Checks iptables and ip6tables firewall rules",
		Contacts: []string{
			"jorgelo@chromium.org", // Security team
			"derat@chromium.org",   // Tast port author
			"chromeos-security@google.com",
		},
		SoftwareDeps: []string{"firewall"},
	})
}

func Firewall(ctx context.Context, s *testing.State) {
	// Runs prog ("iptables" or "ip6tables") with -S and checks that required rules are present.
	checkRules := func(prog string, required []string) {
		cmd := testexec.CommandContext(ctx, prog, "-S")
		out, err := cmd.Output()
		if err != nil {
			s.Errorf("Running %s failed: %v", prog, err)
			cmd.DumpLog(ctx)
			return
		}
		// Save the full output to aid in debugging.
		if err := ioutil.WriteFile(filepath.Join(s.OutDir(), prog+".txt"), out, 0644); err != nil {
			s.Errorf("Failed to save %v output: %v", prog, err)
		}

		seen := make(map[string]struct{})
		for _, rule := range strings.Split(strings.TrimSpace(string(out)), "\n") {
			seen[strings.TrimSpace(rule)] = struct{}{}
		}
		for _, rule := range required {
			if _, ok := seen[rule]; !ok {
				s.Errorf("Missing %v rule %q", prog, rule)
			}
		}
	}

	checkRules("iptables", []string{
		"-P INPUT DROP",
		"-P FORWARD DROP",
		"-P OUTPUT DROP",
		"-A INPUT -m state --state RELATED,ESTABLISHED -j ACCEPT",
		"-A INPUT -i lo -j ACCEPT",
		"-A INPUT -p icmp -j ACCEPT",
		"-A INPUT -p tcp -m tcp --dport 22 -j ACCEPT",
		"-A INPUT -d 224.0.0.251/32 -p udp -m udp --dport 5353 -j ACCEPT",
		"-A OUTPUT -m state --state NEW,RELATED,ESTABLISHED -j ACCEPT",
		"-A OUTPUT -o lo -j ACCEPT",
	})
	checkRules("ip6tables", []string{
		"-P INPUT DROP",
		"-P FORWARD DROP",
		"-P OUTPUT DROP",
		"-A INPUT -m state --state RELATED,ESTABLISHED -j ACCEPT",
		"-A INPUT -i lo -j ACCEPT",
		"-A INPUT -p ipv6-icmp -j ACCEPT",
		"-A INPUT -p tcp -m tcp --dport 22 -j ACCEPT",
		"-A INPUT -d ff02::fb/128 -p udp -m udp --dport 5353 -j ACCEPT",
		"-A OUTPUT -m state --state NEW,RELATED,ESTABLISHED -j ACCEPT",
		"-A OUTPUT -o lo -j ACCEPT",
		"-A OUTPUT -p ipv6-icmp -j ACCEPT",
	})
}
