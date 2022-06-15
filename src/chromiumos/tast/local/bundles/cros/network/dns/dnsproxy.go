// Copyright 2022 The ChromiumOS Authors.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package dns

import (
	"context"
	"strconv"
	"strings"
	"time"

	"chromiumos/tast/common/testexec"
	"chromiumos/tast/ctxutil"
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
	rules []rule
}

// do installs or deletes the blocking rules.
func (b Block) do(ctx context.Context, op blockOp) []error {
	var errs []error
	for _, r := range b.rules {
		r.op = op.String()
		if err := testexec.CommandContext(ctx, r.cmd(), r.args()...).Run(testexec.DumpLogOnError); err != nil {
			errs = append(errs, errors.Wrapf(err, "failed to modify block rule: %+v", r))
			if op == opInsert {
				break
			}
		}
	}
	return errs
}

// Run runs |f| after inserting blocking rules which will always be automatically reverted.
// The functions returns any errors that occur during blocking (only).
func (b Block) Run(ctx context.Context, f func(context.Context)) (errs []error) {
	// Defer deleting the block first to try to cleanup any leftovers in the event insert fails.
	ctxCleanup := ctx
	ctx, cancel := ctxutil.Shorten(ctx, 5*time.Second)
	defer cancel()
	defer func() {
		errs = append(errs, b.do(ctxCleanup, opDelete)...)
	}()
	// Insert the blocking rules.
	if errs := b.do(ctx, opInsert); len(errs) > 0 {
		return errs
	}
	f(ctx)
	return nil
}

type ipCmd struct {
	v6 bool
	ns string
}

func (c ipCmd) cmd() string {
	if c.ns != "" {
		return "ip"
	}
	return c.iptables()
}

func (c ipCmd) iptables() string {
	if c.v6 {
		return "ip6tables"
	}
	return "iptables"
}

func (c ipCmd) args() []string {
	if c.ns == "" {
		return []string{}
	}
	return []string{"netns", "exec", c.ns, c.iptables()}
}

type rule struct {
	ipc    ipCmd
	op     string
	chain  string
	dest   string
	dport  int
	proto  string
	oif    string
	owner  string
	target string
}

func (r rule) cmd() string {
	return r.ipc.cmd()
}

func (r rule) args() []string {
	args := append(r.ipc.args(), []string{r.op, r.chain}...)
	if r.proto != "" {
		args = append(args, []string{"-p", r.proto}...)
	}
	if r.dest != "" {
		args = append(args, []string{"-d", r.dest}...)
	}
	if r.dport > 0 {
		args = append(args, []string{"--dport", strconv.Itoa(r.dport)}...)
	}
	if r.oif != "" {
		args = append(args, []string{"-o", r.oif}...)
	}
	if r.owner != "" {
		args = append(args, []string{"-m", "owner", "--uid-owner", r.owner}...)
	}
	return append(args, []string{"-j", r.target, "-w"}...)
}

func newPlaintextDropRules(nss, ifs []string) []rule {
	var rules []rule
	r := rule{
		chain:  "OUTPUT",
		dport:  53,
		target: "DROP",
	}
	for _, v6 := range []bool{false, true} {
		r.ipc.v6 = v6
		for _, p := range []string{"udp", "tcp"} {
			r.proto = p
			// Block dnsproxy namespaces.
			for _, ns := range nss {
				r.ipc.ns = ns
				rules = append(rules, r)
				r.ipc.ns = ""
			}
			// Block host.
			for _, i := range ifs {
				r.oif = i
				rules = append(rules, r)
				r.oif = ""
			}
			// Block Chrome.
			r.owner = "chronos"
			rules = append(rules, r)
			r.owner = ""
		}
	}
	return rules
}

func newDoHDropRules(nss, ifs []string) []rule {
	var rules []rule
	r := rule{
		chain:  "OUTPUT",
		proto:  "tcp",
		dport:  443,
		target: "DROP",
	}
	for _, v6 := range []bool{false, true} {
		r.ipc.v6 = v6
		// Block dnsproxy namespaces.
		for _, ns := range nss {
			r.ipc.ns = ns
			rules = append(rules, r)
			r.ipc.ns = ""
		}
		// Block host.
		for _, i := range ifs {
			r.oif = i
			rules = append(rules, r)
			r.oif = ""
		}
		// Block Chrome.
		r.owner = "chronos"
		rules = append(rules, r)
		r.owner = ""
	}

	return rules
}

func newVPNDropRules(ns string) []rule {
	var rules []rule
	r := rule{
		chain:  "FORWARD",
		target: "DROP",
	}
	r.ipc.ns = ns
	for _, v6 := range []bool{false, true} {
		r.ipc.v6 = v6
		r.dport = 53
		for _, p := range []string{"udp", "tcp"} {
			r.proto = p
			rules = append(rules, r)
		}
		r.proto = "tcp"
		r.dport = 443
		rules = append(rules, r)
	}
	return rules
}

func newDoHVPNDropRules(ns string) []rule {
	var rules []rule
	r := rule{
		chain:  "FORWARD",
		proto:  "tcp",
		dport:  443,
		target: "DROP",
	}
	r.ipc.ns = ns
	for _, v6 := range []bool{false, true} {
		r.ipc.v6 = v6
		rules = append(rules, r)
	}
	return rules
}

// NewPlaintextBlock creates a Block that will block any UDP or TCP packets egressing from
// the namespaces in |nss| or interface in |ifs| on port 53.
func NewPlaintextBlock(nss, ifs []string) *Block {
	return &Block{
		rules: newPlaintextDropRules(nss, ifs),
	}
}

// NewDoHBlock creates a Block that will block any TCP packets egressing from
// the namespaces in |nss| or interface in |ifs| on port 443.
func NewDoHBlock(nss, ifs []string) *Block {
	return &Block{
		rules: newDoHDropRules(nss, ifs),
	}
}

// NewVPNBlock creates a Block that will block any UDP and TCP packets egressing from
// the namespaces |ns| on ports 53 and 443.
func NewVPNBlock(ns string) *Block {
	return &Block{
		rules: newVPNDropRules(ns),
	}
}

// NewDoHVPNBlock creates a Block that will block any TCP packets egressing from
// the namespaces |ns| on port 443.
func NewDoHVPNBlock(ns string) *Block {
	return &Block{
		rules: newDoHVPNDropRules(ns),
	}
}
