// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package network

import (
	"context"
	"net"
	"strings"
	"time"

	"chromiumos/tast/common/testexec"
	"chromiumos/tast/ctxutil"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/apps"
	"chromiumos/tast/local/arc"
	"chromiumos/tast/local/bundles/cros/network/vpn"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/ui"
	"chromiumos/tast/local/crostini"
	cui "chromiumos/tast/local/crostini/ui"
	"chromiumos/tast/local/input"
	"chromiumos/tast/local/vm"
	"chromiumos/tast/testing"
)

type dnsProxyTestParams struct {
	secureDNS bool
}

func init() {
	testing.AddTest(&testing.Test{
		Func:         DNSProxy,
		Desc:         "Ensure that DNS proxies are working correctly",
		Contacts:     []string{"jasongustaman@google.com", "garrick@google.com", "cros-networking@google.com"},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"chrome", "vm_host", "arc"},
		Params: []testing.Param{{
			Name: "plaintext_dns",
			Val: dnsProxyTestParams{
				secureDNS: false,
			},
			ExtraData:         []string{crostini.GetContainerMetadataArtifact("buster", false), crostini.GetContainerRootfsArtifact("buster", false)},
			ExtraSoftwareDeps: []string{"dlc"},
			ExtraHardwareDeps: crostini.CrostiniStable,
			Timeout:           7 * time.Minute,
		}, {
			Name: "secure_dns",
			Val: dnsProxyTestParams{
				secureDNS: true,
			},
			ExtraData:         []string{crostini.GetContainerMetadataArtifact("buster", false), crostini.GetContainerRootfsArtifact("buster", false)},
			ExtraSoftwareDeps: []string{"dlc"},
			ExtraHardwareDeps: crostini.CrostiniStable,
			Timeout:           7 * time.Minute,
		}},
	})
}

// clientType defines the client resolving DNS.
type clientType int

const (
	systemType clientType = iota
	userType
	chromeType
	crostiniType
	arcType
)

const (
	// Randomly generated domains to be resolved. Different domains are used to avoid caching.
	domainDefault    = "a2ffec2cb85be5e7.com"
	domainDNSBlocked = "da39a3ee5e6b4b0d.com"
	domainVPN        = "c30c8b722af2d577.com"
	domainVPNBlocked = "bff40af49dd97a3d.com"

	// VPN interface name prefix for the test.
	vpnIfnamePrefix = "ppp"

	// DNS-over-HTTPS provider used for the test.
	dohProvider = "https://dns.google/dns-query"
)

// DNSProxy tests DNS functionality with DNS proxy active.
// The tests are done 4 times:
// 1. DNS queries on normal condition.
// 2. DNS queries when DNS packets are blocked.
// 3. DNS queries with VPN on.
// 3. DNS queries with VPN on and blocked.
func DNSProxy(ctx context.Context, s *testing.State) {
	// If the main body of the test times out, we still want to reserve a few
	// seconds to allow for our cleanup code to run.
	cleanupCtx := ctx
	ctx, cancel := ctxutil.Shorten(cleanupCtx, 3*time.Second)
	defer cancel()

	cr, err := chrome.New(ctx, chrome.ARCEnabled(), chrome.EnableFeatures("EnableDnsProxy", "DnsProxyEnableDOH"))
	if err != nil {
		s.Fatal("Failed to start Chrome: ", err)
	}
	defer cr.Close(cleanupCtx)

	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to get test API connection: ", err)
	}

	// Install Crostini.
	opts := crostini.GetInstallerOptions(s, vm.DebianBuster, false /*largeContainer*/, cr.NormalizedUser())
	if _, err := cui.InstallCrostini(ctx, tconn, cr, opts); err != nil {
		s.Fatal("Failed to install Crostini: ", err)
	}
	defer func() {
		vm.UnmountComponent(cleanupCtx)
		if err := vm.DeleteImages(); err != nil {
			testing.ContextLogf(cleanupCtx, "Error deleting images: %q", err)
		}
	}()

	// Get the container.
	cont, err := vm.DefaultContainer(ctx, opts.UserName)
	if err != nil {
		s.Fatal("Failed to connect to container: ", err)
	}

	// Install dig in container.
	if err := installDigInContainer(ctx, cont); err != nil {
		s.Fatal("Failed to install dig in container: ", err)
	}

	// Start ARC.
	a, err := arc.New(ctx, s.OutDir())
	if err != nil {
		s.Fatal("Failed to start ARC: ", err)
	}
	defer a.Close(cleanupCtx)

	// Toggle plain-text DNS or secureDNS depending on test parameter.
	params := s.Param().(dnsProxyTestParams)
	if err := toggleSecureDNS(ctx, cr, tconn, params.secureDNS); err != nil {
		s.Fatal(ctx, "Failed to enable secure DNS: ", err)
	}

	type testCase struct {
		ct        clientType
		expectErr bool
	}

	// By default, DNS query should work.
	var defaultTC = []testCase{
		testCase{ct: systemType, expectErr: false},
		testCase{ct: userType, expectErr: false},
		testCase{ct: chromeType, expectErr: false},
		testCase{ct: crostiniType, expectErr: false},
		testCase{ct: arcType, expectErr: false},
	}
	for _, tc := range defaultTC {
		if err := queryDNS(ctx, tc.ct, a, cont, domainDefault, tc.expectErr); err != nil {
			s.Error("Failed DNS query check: ", err)
		}
	}

	nss, err := dnsProxyNamespaces(ctx)
	if err != nil {
		s.Fatal("Failed to get DNS proxy's network namespaces: ", err)
	}

	physIfs, err := physicalInterfaces(ctx)
	if err != nil {
		s.Fatal(ctx, "Failed to get phyiscal interfaces: ", err)
	}

	// Block plain-text or secure DNS through iptables.
	var unblock func()
	var dnsBlockedTC []testCase
	if params.secureDNS {
		unblock, err = blockSecureDNS(ctx, nss, physIfs)
		dnsBlockedTC = []testCase{
			testCase{ct: systemType, expectErr: true},
			testCase{ct: userType, expectErr: true},
			testCase{ct: crostiniType, expectErr: true}}
	} else {
		unblock, err = blockPlaintextDNS(ctx, nss, physIfs)
		dnsBlockedTC = []testCase{
			testCase{ct: systemType, expectErr: true},
			testCase{ct: userType, expectErr: true},
			testCase{ct: chromeType, expectErr: true},
			testCase{ct: crostiniType, expectErr: true}}
	}
	if err != nil {
		s.Fatal(ctx, "Failed to block DNS: ", err)
	}

	// DNS queries should fail if corresponding DNS packets (plain-text or secure) are dropped.
	for _, tc := range dnsBlockedTC {
		if err := queryDNS(ctx, tc.ct, a, cont, domainDNSBlocked, tc.expectErr); err != nil {
			s.Error("Failed DNS query check: ", err)
		}
	}
	unblock()

	// Connect to VPN.
	config := vpn.Config{
		Type:     vpn.TypeL2TPIPsec,
		AuthType: vpn.AuthTypePSK,
	}
	conn, err := vpn.NewConnection(ctx, config)
	if err != nil {
		s.Fatal("Failed to create connection object: ", err)
	}
	defer func() {
		if err := conn.Cleanup(cleanupCtx); err != nil {
			s.Error("Failed to clean up connection: ", err)
		}
	}()
	if _, err = conn.Start(ctx); err != nil {
		s.Fatal("Failed to connect to VPN server: ", err)
	}

	ns, err := vpnNamespace(ctx)
	if err != nil {
		s.Fatal("Failed to get DNS proxy's network namespaces")
	}

	ifs, err := testexec.CommandContext(ctx, "ls", "/sys/class/net").Output()
	if err != nil {
		s.Fatal("Failed to get network interfaces: ", err)
	}

	var vpnIfname string
	for _, ifname := range strings.Fields(string(ifs)) {
		if strings.HasPrefix(ifname, vpnIfnamePrefix) {
			vpnIfname = ifname
			break
		}
	}
	if vpnIfname == "" {
		s.Fatal("Failed to get VPN interface")
	}

	// Get VPN interface IPv4 address.
	iface, err := net.InterfaceByName(vpnIfname)
	if err != nil {
		s.Fatal("Failed to get interface: ", err)
	}
	addrs, err := iface.Addrs()
	if err != nil {
		s.Fatal("Failed to get interface address: ", err)
	}
	var ip string
	for _, addr := range addrs {
		if ipnet, ok := addr.(*net.IPNet); ok && ipnet.IP.To4() != nil {
			ip = ipnet.IP.String()
			break
		}
	}

	// Setup connectivity for VPN.
	if err := testexec.CommandContext(ctx, "ip", "netns", "exec", ns, "iptables", "-t", "nat", "-A", "POSTROUTING", "-s", ip, "-j", "SNAT", "--to", conn.Server.UnderlayIP).Run(); err != nil {
		s.Fatal("Failed to setup connectivity for VPN: ", err)
	}

	// By default, DNS query should work on VPN.
	for _, tc := range defaultTC {
		if err := queryDNS(ctx, tc.ct, a, cont, domainVPN, tc.expectErr); err != nil {
			s.Error("Failed DNS query check: ", err)
		}
	}

	// Block DNS queries over VPN through iptables.
	if err := blockDNSOverVPN(ctx, ns); err != nil {
		s.Fatal(ctx, "Failed to block DNS over VPN: ", err)
	}

	// DNS queries that should be routed through VPN should fail if DNS queries on the VPN server are blocked.
	vpnBlockedTC := []testCase{
		testCase{ct: systemType, expectErr: false},
		testCase{ct: userType, expectErr: true},
		testCase{ct: chromeType, expectErr: true},
		testCase{ct: crostiniType, expectErr: true}}
	for _, tc := range vpnBlockedTC {
		if err := queryDNS(ctx, tc.ct, a, cont, domainVPNBlocked, tc.expectErr); err != nil {
			s.Error("Failed DNS query check: ", err)
		}
	}
}

// vpnNamespace iterates through available network namespaces and return the namespace with the VPN server.
// VPN namespace is identified by checking if the namespace contains a VPN interface.
func vpnNamespace(ctx context.Context) (string, error) {
	out, err := testexec.CommandContext(ctx, "ip", "netns", "list").Output()
	if err != nil {
		return "", err
	}

	for _, o := range strings.Split(strings.TrimSpace(string(out)), "\n") {
		ns := strings.Fields(o)[0]
		ifnames, err := testexec.CommandContext(ctx, "ip", "netns", "exec", ns, "ls", "/sys/class/net").Output()
		if err != nil {
			return "", nil
		}
		for _, ifname := range strings.Fields(string(ifnames)) {
			if strings.HasPrefix(ifname, vpnIfnamePrefix) {
				return ns, nil
			}
		}
	}
	return "", nil
}

// dnsProxyNamespaces iterates through available network namespaces and return the namespaces with DNS proxy.
// DNS proxy namespaces are identified by checking if the namespace contain a listening process named dnsproxyd.
func dnsProxyNamespaces(ctx context.Context) ([]string, error) {
	// return in the following format
	out, err := testexec.CommandContext(ctx, "ip", "netns", "list").Output()
	if err != nil {
		return nil, err
	}

	var nss []string
	for _, o := range strings.Split(strings.TrimSpace(string(out)), "\n") {
		ns := strings.Fields(o)[0]
		ss, err := testexec.CommandContext(ctx, "ip", "netns", "exec", ns, "ss", "-lptun").Output()
		if err != nil {
			return nil, err
		}
		if strings.Contains(string(ss), "dnsproxyd") {
			nss = append(nss, ns)
		}
	}
	return nss, nil
}

func physicalInterfaces(ctx context.Context) ([]string, error) {
	out, err := testexec.CommandContext(ctx, "/usr/bin/find", "/sys/class/net", "-type", "l", "-not", "-lname", "*virtual*", "-printf", "%f\n").Output()
	if err != nil {
		return nil, errors.Wrap(err, "failed to get physical interfaces")
	}
	return strings.Split(strings.TrimSpace(string(out)), "\n"), nil
}

// blockPlaintextDNS blocks plaintext DNS outbound packets that go through the physical interfaces or DNS proxy namespace.
// Blocking is done by dropping outbound UDP and TCP packets with port 53.
func blockPlaintextDNS(ctx context.Context, nss, physIfs []string) (unblock func(), e error) {
	doUnblock := func() {
		for _, cmd := range []string{"iptables", "ip6tables"} {
			for _, ns := range nss {
				testexec.CommandContext(ctx, "ip", "netns", "exec", ns, cmd, "-F").Run()
			}
			for _, ifname := range physIfs {
				testexec.CommandContext(ctx, cmd, "-D", "OUTPUT", "-p", "udp", "--dport", "53", "-o", ifname, "-j", "DROP").Run()
				testexec.CommandContext(ctx, cmd, "-D", "OUTPUT", "-p", "tcp", "--dport", "53", "-o", ifname, "-j", "DROP").Run()
			}
			testexec.CommandContext(ctx, cmd, "-D", "OUTPUT", "-p", "udp", "--dport", "53", "-m", "owner", "--uid-owner", "chronos", "-j", "DROP").Run()
			testexec.CommandContext(ctx, cmd, "-D", "OUTPUT", "-p", "tcp", "--dport", "53", "-m", "owner", "--uid-owner", "chronos", "-j", "DROP").Run()
		}
	}

	for _, cmd := range []string{"iptables", "ip6tables"} {
		for _, ns := range nss {
			if err := testexec.CommandContext(ctx, "ip", "netns", "exec", ns, cmd, "-I", "OUTPUT", "-p", "udp", "--dport", "53", "-j", "DROP").Run(); err != nil {
				return doUnblock, errors.Wrapf(err, "failed to block UDP plaintext DNS on %s", ns)
			}
			if err := testexec.CommandContext(ctx, "ip", "netns", "exec", ns, cmd, "-I", "OUTPUT", "-p", "tcp", "--dport", "53", "-j", "DROP").Run(); err != nil {
				return doUnblock, errors.Wrapf(err, "failed to block TCP plaintext DNS on %s", ns)
			}
		}
		for _, ifname := range physIfs {
			if err := testexec.CommandContext(ctx, cmd, "-I", "OUTPUT", "-p", "udp", "--dport", "53", "-o", ifname, "-j", "DROP").Run(); err != nil {
				return doUnblock, errors.Wrapf(err, "failed to block UDP plaintext DNS for %s", ifname)
			}
			if err := testexec.CommandContext(ctx, cmd, "-I", "OUTPUT", "-p", "tcp", "--dport", "53", "-o", ifname, "-j", "DROP").Run(); err != nil {
				return doUnblock, errors.Wrapf(err, "failed to block TCP plaintext DNS for %s", ifname)
			}
		}
		if err := testexec.CommandContext(ctx, cmd, "-I", "OUTPUT", "-p", "udp", "--dport", "53", "-m", "owner", "--uid-owner", "chronos", "-j", "DROP").Run(); err != nil {
			return doUnblock, errors.Wrap(err, "failed to block UDP plaintext DNS for Chrome")
		}
		if err := testexec.CommandContext(ctx, cmd, "-I", "OUTPUT", "-p", "tcp", "--dport", "53", "-m", "owner", "--uid-owner", "chronos", "-j", "DROP").Run(); err != nil {
			return doUnblock, errors.Wrap(err, "failed to block TCP plaintext DNS for Chrome")
		}
	}

	return doUnblock, nil
}

// blockSecureDNS blocks secure DNS outbound packets that go through the physical interfaces or DNS proxy namespace.
// Blocking is done by dropping outbound TCP packets with port 443 (HTTPS packets).
func blockSecureDNS(ctx context.Context, nss, physIfs []string) (unblock func(), e error) {
	doUnblock := func() {
		for _, cmd := range []string{"iptables", "ip6tables"} {
			for _, ns := range nss {
				testexec.CommandContext(ctx, "ip", "netns", "exec", ns, cmd, "-F").Run()
			}
			for _, ifname := range physIfs {
				testexec.CommandContext(ctx, cmd, "-D", "OUTPUT", "-p", "tcp", "--dport", "443", "-o", ifname, "-j", "DROP").Run()
			}
			testexec.CommandContext(ctx, cmd, "-D", "OUTPUT", "-p", "tcp", "--dport", "443", "-m", "owner", "--uid-owner", "chronos", "-j", "DROP").Run()
		}
	}

	for _, cmd := range []string{"iptables", "ip6tables"} {
		for _, ns := range nss {
			if err := testexec.CommandContext(ctx, "ip", "netns", "exec", ns, cmd, "-I", "OUTPUT", "-p", "tcp", "--dport", "443", "-j", "DROP").Run(); err != nil {
				return doUnblock, errors.Wrapf(err, "failed to block secure DNS on %s", ns)
			}
		}
		for _, ifname := range physIfs {
			if err := testexec.CommandContext(ctx, cmd, "-I", "OUTPUT", "-p", "tcp", "--dport", "443", "-o", ifname, "-j", "DROP").Run(); err != nil {
				return doUnblock, errors.Wrapf(err, "failed to block secure DNS for %s", ifname)
			}
		}
		if err := testexec.CommandContext(ctx, cmd, "-I", "OUTPUT", "-p", "tcp", "--dport", "443", "-m", "owner", "--uid-owner", "chronos", "-j", "DROP").Run(); err != nil {
			return doUnblock, errors.Wrap(err, "failed to block secure DNS for Chrome")
		}
	}

	return doUnblock, nil
}

// blockDNSOverVPN blocks DNS outbound packets that go through VPN.
// Blocking is done by dropping outbound TCP packets with port 443 (HTTPS packets) and TCP and UDP packets with port 53 (plaintext DNS packets).
func blockDNSOverVPN(ctx context.Context, ns string) error {
	for _, cmd := range []string{"iptables", "ip6tables"} {
		if err := testexec.CommandContext(ctx, "ip", "netns", "exec", ns, cmd, "-I", "FORWARD", "-p", "udp", "--dport", "53", "-j", "DROP").Run(); err != nil {
			return err
		}
		if err := testexec.CommandContext(ctx, "ip", "netns", "exec", ns, cmd, "-I", "FORWARD", "-p", "tcp", "--dport", "53", "-j", "DROP").Run(); err != nil {
			return err
		}
		if err := testexec.CommandContext(ctx, "ip", "netns", "exec", ns, cmd, "-I", "FORWARD", "-p", "tcp", "--dport", "443", "-j", "DROP").Run(); err != nil {
			return err
		}
	}
	return nil
}

func queryDNS(ctx context.Context, ct clientType, a *arc.ARC, cont *vm.Container, domain string, expectErr bool) error {
	var err error
	var name string
	switch ct {
	case systemType:
		name = "system"
		err = testexec.CommandContext(ctx, "dig", domain).Run()
	case userType:
		name = "user"
		err = testexec.CommandContext(ctx, "sudo", "-u", "cups", "dig", domain).Run()
	case chromeType:
		name = "chrome"
		err = testexec.CommandContext(ctx, "sudo", "-u", "chronos", "dig", domain).Run()
	case crostiniType:
		name = "crostini"
		err = cont.Command(ctx, "dig", domain).Run()
	case arcType:
		name = "arc"
		err = a.Command(ctx, "dumpsys", "wifi", "tools", "dns", domain).Run()
	}
	if !expectErr && err != nil {
		return errors.Wrapf(err, "failed to resolve DNS for %s", name)
	}
	if expectErr && err == nil {
		return errors.Wrapf(err, "successfully resolve DNS, but expected failure for %s", name)
	}
	return nil
}

// toggleSecureDNS toggle Chrome OS setting to enable / disable secure DNS.
func toggleSecureDNS(ctx context.Context, cr *chrome.Chrome, tconn *chrome.TestConn, enable bool) error {
	conn, err := apps.LaunchOSSettings(ctx, cr, "chrome://os-settings/osPrivacy")
	if err != nil {
		return errors.Wrap(err, "failed to get connection to OS Settings")
	}
	defer conn.Close()

	if enable {
		tb, err := ui.FindWithTimeout(
			ctx,
			tconn,
			ui.FindParams{Role: ui.RoleTypeToggleButton, Name: "Use secure DNS"},
			5*time.Second,
		)
		if err != nil {
			return errors.Wrap(err, "failed to find secure DNS toggle button")
		}
		defer tb.Release(ctx)

		// Make sure secure DNS is turned on.
		if tb.Checked == ui.CheckedStateFalse {
			if err := tb.LeftClick(ctx); err != nil {
				return errors.Wrap(err, "failed to toggle secure DNS button")
			}
		}

		// Input a custom DoH provider.
		tf, err := ui.FindWithTimeout(
			ctx,
			tconn,
			ui.FindParams{Role: ui.RoleTypeTextField, Name: "Enter custom provider"},
			5*time.Second)
		if err != nil {
			return errors.Wrap(err, "failed to find secure DNS provider text field")
		}
		defer tf.Release(ctx)
		if err := tf.LeftClick(ctx); err != nil {
			return errors.Wrap(err, "failed to left click secure DNS provider text field")
		}

		// Get a handle to the input keyboard
		kb, err := input.Keyboard(ctx)
		if err != nil {
			return errors.Wrap(err, "failed to get keyboard")
		}
		defer kb.Close()
		if err := kb.Accel(ctx, "Ctrl+A"); err != nil {
			return errors.Wrap(err, "failed to clear secure DNS provider text")
		}
		if err := kb.Accel(ctx, "Backspace"); err != nil {
			return errors.Wrap(err, "failed to clear secure DNS provider text")
		}
		if err := kb.Type(ctx, dohProvider); err != nil {
			return errors.Wrap(err, "failed to type secure DNS provider")
		}

		// Find and click "use custom DNS provider" button.
		rg, err := ui.FindWithTimeout(
			ctx,
			tconn,
			ui.FindParams{Role: ui.RoleTypeRadioGroup},
			5*time.Second)
		if err != nil {
			return errors.Wrap(err, "failed to find secure DNS radio group")
		}
		defer rg.Release(ctx)
		rbs, err := rg.Descendants(ctx, ui.FindParams{Role: ui.RoleTypeRadioButton})
		if err != nil {
			return errors.Wrap(err, "failed to get secure DNS radio group descendants")
		}
		defer rbs.Release(ctx)

		var rb *ui.Node
		for _, e := range rbs {
			if e.Name != "With your current service provider" {
				rb = e
			}
		}
		if rb == nil {
			return errors.Wrap(err, "failed to find secure DNS radio button")
		}
		if err := rb.LeftClick(ctx); err != nil {
			return errors.Wrap(err, "failed to left click secure DNS radio button")
		}
		return nil
	}

	// Disable secure DNS, the UI might lag, keep trying until secure DNS is disabled.
	return testing.Poll(ctx, func(ctx context.Context) error {
		tb, err := ui.StableFind(
			ctx,
			tconn,
			ui.FindParams{Role: ui.RoleTypeToggleButton, Name: "Use secure DNS"},
			&testing.PollOptions{Timeout: 5 * time.Second},
		)
		if err != nil {
			return errors.Wrap(err, "failed to find secure DNS toggle button")
		}
		defer tb.Release(ctx)

		if tb.Checked == ui.CheckedStateFalse {
			return nil
		}
		if err := tb.LeftClick(ctx); err != nil {
			return errors.Wrap(err, "failed to toggle secure DNS button")
		}
		return errors.New("failed to toggle secure DNS button")
	}, &testing.PollOptions{Timeout: 60 * time.Second})
}

// installDigInContainer installs dig in container.
func installDigInContainer(ctx context.Context, cont *vm.Container) error {
	// Check whether dig is preinstalled or not.
	if err := cont.Command(ctx, "dig", "-v").Run(); err == nil {
		return nil
	}

	// Run command sudo apt update in container.
	if err := cont.Command(ctx, "sudo", "apt", "update").Run(); err != nil {
		return errors.Wrap(err, "failed to run command sudo apt update in container")
	}

	// Run command sudo apt install dnsutils in container.
	if err := cont.Command(ctx, "sudo", "DEBIAN_FRONTEND=noninteractive", "apt-get", "-y", "install", "dnsutils").Run(testexec.DumpLogOnError); err != nil {
		return errors.Wrap(err, "failed to run command sudo apt install dnsutils in container")
	}

	// Run command dig -v and check the output to make sure vim has been installed successfully.
	if err := cont.Command(ctx, "dig", "-v").Run(); err != nil {
		return errors.Wrap(err, "failed to install dig in container")
	}
	return nil
}
