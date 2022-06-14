// Copyright 2022 The ChromiumOS Authors.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package dns

import (
	"context"
	"strings"

	"chromiumos/tast/common/testexec"
	"chromiumos/tast/errors"
)

// ProxyNamespaces returns all network namespaces used by the dnsproxyd process.
func ProxyNamespaces(ctx context.Context) ([]string, error) {
	out, err := testexec.CommandContext(ctx, "ip", "netns", "list").Output(testexec.DumpLogOnError)
	if err != nil {
		return nil, err
	}

	var nss []string
	for _, o := range strings.Split(strings.TrimSpace(string(out)), "\n") {
		ns := strings.Fields(o)[0]
		ss, err := testexec.CommandContext(ctx, "ip", "netns", "exec", ns, "ss", "-lptun").Output(testexec.DumpLogOnError)
		if err != nil {
			return nil, err
		}
		if strings.Contains(string(ss), "dnsproxyd") {
			nss = append(nss, ns)
		}
	}
	return nss, nil
}

type blockOp int

const (
	opInsert = iota
	opDelete
)

func (o blockOp) String() string {
	return []string{"-I", "-D"}[o]
}

// Block is a mechanism for using ip rules-based blocking in a scoped/safe way.
type Block struct {
	f func(context.Context, blockOp) []error
}

func modifyPlaintextDropRules(ctx context.Context, op string, nss, ifs []string) []error {
	var errs []error
	for _, cmd := range []string{"iptables", "ip6tables"} {
		for _, ns := range nss {
			if err := testexec.CommandContext(ctx, "ip", "netns", "exec", ns, cmd, op, "OUTPUT", "-p", "udp", "--dport", "53", "-j", "DROP", "-w").Run(testexec.DumpLogOnError); err != nil {
				errs = append(errs, errors.Wrapf(err, "failed to modify UDP plaintext DNS block rule on %s", ns))
			}
			if err := testexec.CommandContext(ctx, "ip", "netns", "exec", ns, cmd, op, "OUTPUT", "-p", "tcp", "--dport", "53", "-j", "DROP", "-w").Run(testexec.DumpLogOnError); err != nil {
				errs = append(errs, errors.Wrapf(err, "failed to modify TCP plaintext DNS block rule on %s", ns))
			}
		}
		for _, i := range ifs {
			if err := testexec.CommandContext(ctx, cmd, op, "OUTPUT", "-p", "udp", "--dport", "53", "-o", i, "-j", "DROP", "-w").Run(testexec.DumpLogOnError); err != nil {
				errs = append(errs, errors.Wrapf(err, "failed to modify UDP plaintext DNS block rule for %s", i))
			}
			if err := testexec.CommandContext(ctx, cmd, op, "OUTPUT", "-p", "tcp", "--dport", "53", "-o", i, "-j", "DROP", "-w").Run(testexec.DumpLogOnError); err != nil {
				errs = append(errs, errors.Wrapf(err, "failed to modify TCP plaintext DNS block rule for %s", i))
			}
		}
		if err := testexec.CommandContext(ctx, cmd, op, "OUTPUT", "-p", "udp", "--dport", "53", "-m", "owner", "--uid-owner", "chronos", "-j", "DROP", "-w").Run(testexec.DumpLogOnError); err != nil {
			errs = append(errs, errors.Wrap(err, "failed to modify UDP plaintext DNS block rule for Chrome"))
		}
		if err := testexec.CommandContext(ctx, cmd, op, "OUTPUT", "-p", "tcp", "--dport", "53", "-m", "owner", "--uid-owner", "chronos", "-j", "DROP", "-w").Run(testexec.DumpLogOnError); err != nil {
			errs = append(errs, errors.Wrap(err, "failed to modify TCP plaintext DNS block rule for Chrome"))
		}
	}
	return errs
}

func modifyDoHDropRules(ctx context.Context, op string, nss, physIfs []string) []error {
	var errs []error
	for _, cmd := range []string{"iptables", "ip6tables"} {
		for _, ns := range nss {
			if err := testexec.CommandContext(ctx, "ip", "netns", "exec", ns, cmd, op, "OUTPUT", "-p", "tcp", "--dport", "443", "-j", "DROP", "-w").Run(testexec.DumpLogOnError); err != nil {
				errs = append(errs, errors.Wrapf(err, "failed to modify secure DNS block rule on %s", ns))
			}
		}
		for _, ifname := range physIfs {
			if err := testexec.CommandContext(ctx, cmd, op, "OUTPUT", "-p", "tcp", "--dport", "443", "-o", ifname, "-j", "DROP", "-w").Run(testexec.DumpLogOnError); err != nil {
				errs = append(errs, errors.Wrapf(err, "failed to modify secure DNS block rule for %s", ifname))
			}
		}
		if err := testexec.CommandContext(ctx, cmd, op, "OUTPUT", "-p", "tcp", "--dport", "443", "-m", "owner", "--uid-owner", "chronos", "-j", "DROP", "-w").Run(testexec.DumpLogOnError); err != nil {
			errs = append(errs, errors.Wrap(err, "failed to modify secure DNS block rule for Chrome"))
		}
	}
	return errs
}

func modifyVPNDropRules(ctx context.Context, op, ns string) []error {
	var errs []error
	for _, cmd := range []string{"iptables", "ip6tables"} {
		if err := testexec.CommandContext(ctx, "ip", "netns", "exec", ns, cmd, op, "FORWARD", "-p", "udp", "--dport", "53", "-j", "DROP", "-w").Run(testexec.DumpLogOnError); err != nil {
			errs = append(errs, err)
		}
		if err := testexec.CommandContext(ctx, "ip", "netns", "exec", ns, cmd, op, "FORWARD", "-p", "tcp", "--dport", "53", "-j", "DROP", "-w").Run(testexec.DumpLogOnError); err != nil {
			errs = append(errs, err)
		}
		if err := testexec.CommandContext(ctx, "ip", "netns", "exec", ns, cmd, op, "FORWARD", "-p", "tcp", "--dport", "443", "-j", "DROP", "-w").Run(testexec.DumpLogOnError); err != nil {
			errs = append(errs, err)
		}
	}
	return errs
}

func modifyDoHVPNDropRules(ctx context.Context, op, ns string) []error {
	var errs []error
	for _, cmd := range []string{"iptables", "ip6tables"} {
		if err := testexec.CommandContext(ctx, "ip", "netns", "exec", ns, cmd, op, "FORWARD", "-p", "tcp", "--dport", "443", "-j", "DROP", "-w").Run(testexec.DumpLogOnError); err != nil {
			errs = append(errs, err)
		}
	}
	return errs
}

// NewPlaintextBlock creates a Block that will block any UDP or TCP packets egressing from
// the namespaces in |nss| or interface in |ifs| on port 53, when provided to a function that accepts it.
func NewPlaintextBlock(nss, ifs []string) *Block {
	return &Block{
		f: func(ctx context.Context, op blockOp) []error {
			return modifyPlaintextDropRules(ctx, op.String(), nss, ifs)
		},
	}
}

// NewDoHBlock creates a Block that will block any TCP packets egressing from
// the namespaces in |nss| or interface in |ifs| on port 443, when provided to a function that accepts it.
func NewDoHBlock(nss, ifs []string) *Block {
	return &Block{
		f: func(ctx context.Context, op blockOp) []error {
			return modifyDoHDropRules(ctx, op.String(), nss, ifs)
		},
	}
}

// NewVPNBlock creates a Block that will block any UDP and TCP packets egressing from
// the namespaces |ns| on ports 53 and 443, when provided to a function that accepts it.
func NewVPNBlock(ns string) *Block {
	return &Block{
		f: func(ctx context.Context, op blockOp) []error {
			return modifyVPNDropRules(ctx, op.String(), ns)
		},
	}
}

// NewDoHVPNBlock creates a Block that will block any TCP packets egressing from
// the namespaces |ns| on port 443, when provided to a function that accepts it.
func NewDoHVPNBlock(ns string) *Block {
	return &Block{
		f: func(ctx context.Context, op blockOp) []error {
			return modifyDoHVPNDropRules(ctx, op.String(), ns)
		},
	}
}

// DoWithBlock runs |f| after inserting block |b|. |b| will always be automatically reverted.
// Any errors returned during blocking are passed to |e| for evaluation.
func DoWithBlock(ctx context.Context, b *Block, f func(context.Context), e func([]error)) {
	// Defer deleting the block first to try to cleanup any leftovers in the event insert fails.
	defer func() {
		if errs := b.f(ctx, opDelete); len(errs) > 0 {
			e(errs)
		}
	}()
	// Insert the block
	if errs := b.f(ctx, opInsert); len(errs) > 0 {
		e(errs)
	}
	f(ctx)
}

// ProxyTestCase contains test case for DNS proxy tests.
type ProxyTestCase struct {
	Client    Client
	ExpectErr bool
}

// TestQueryDNSProxy runs a set of test cases for DNS proxy.
func TestQueryDNSProxy(ctx context.Context, tcs []ProxyTestCase, a *arc.ARC, cont *vm.Container, opts *QueryOptions) []error {
	var errs []error
	for _, tc := range tcs {
		err := QueryDNS(ctx, tc.Client, a, cont, opts)
		if err != nil && !tc.ExpectErr {
			errs = append(errs, errors.Wrapf(err, "DNS query failed for %s", GetClientString(tc.Client)))
		}
		if err == nil && tc.ExpectErr {
			errs = append(errs, errors.Errorf("successful DNS query for %s, but expected failure", GetClientString(tc.Client)))
		}
	}
	return errs
}
