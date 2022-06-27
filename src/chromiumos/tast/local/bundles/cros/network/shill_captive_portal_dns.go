// Copyright 2022 The ChromiumOS Authors.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package network

import (
	"context"
	"fmt"
	"net"
	"os"
	"path"
	"regexp"
	"strings"
	"time"

	"chromiumos/tast/common/network/tcpdump"
	"chromiumos/tast/common/shillconst"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/bundles/cros/network/virtualnet"
	"chromiumos/tast/local/bundles/cros/network/virtualnet/subnet"
	localTcpdump "chromiumos/tast/local/network/tcpdump"
	"chromiumos/tast/local/shill"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:     ShillCaptivePortalDNS,
		Desc:     "Ensure that shill sends portal detection probes to the IP address given by dnsmasq",
		Contacts: []string{"tinghaolin@google.com", "cros-network-health-team@google.com"},
		Attr:     []string{},
		Fixture:  "shillReset",
	})
}

const (
	resolvedHost      = "#" // '#' matches any domain in dnsmasq configuration.
	resolveHostToIP   = "129.0.0.1"
	tcpdumpStdErrFile = "pcap-dnsmasq.stderr"
	tcpdumpStdOutFile = "pcap-dnsmasq.stdout"
	tcpdumpFile       = "pcap-dnsmasq.pcap"
)

func ShillCaptivePortalDNS(ctx context.Context, s *testing.State) {
	m, err := shill.NewManager(ctx)
	if err != nil {
		s.Fatal("Failed to create manager proxy: ", err)
	}

	testing.ContextLog(ctx, "Enabling portal detection on ethernet")
	if err := m.SetProperty(ctx, shillconst.ProfilePropertyCheckPortalList, "ethernet"); err != nil {
		s.Fatal("Failed to enable portal detection on ethernet: ", err)
	}

	testing.ContextLog(ctx, "Setting up a netns for router")
	pool := subnet.NewPool()
	svc, router, err := virtualnet.CreateRouterEnv(ctx, m, pool, virtualnet.EnvOptions{
		Priority:        5,
		NameSuffix:      "",
		EnableDHCP:      true,
		RAServer:        false,
		ResolvedHost:    resolvedHost,
		ResolveHostToIP: net.ParseIP(resolveHostToIP),
	})
	if err != nil {
		s.Fatal("Failed to create a router env: ", err)
	}
	defer router.Cleanup(ctx)
	ifaceAddrs, err := router.WaitForVethInAddrs(ctx, true, false)

	resolveShillNameServer(ctx, s, router.VethOutName, ifaceAddrs.IPv4Addr.String(), m, svc)
	r := localTcpdump.NewLocalRunner()
	captureHTTPProbe(ctx, s, router.VethOutName, m, r)
	resolveHTTPProbe(ctx, s, r)
}

func resolveShillNameServer(ctx context.Context, s *testing.State, ifName, gateway string, m *shill.Manager, svc *shill.Service) {
	// Use the ipconfig api to resolve the nameserver used by shill.
	s.Log("Attempt to get the nameserver used by shill")

	if err := svc.WaitForProperty(ctx, shillconst.ServicePropertyState, shillconst.ServiceStateReady, 5*time.Second); err != nil {
		s.Fatal("Failed to wait for the service ready: ", err)
	}

	device, err := m.DeviceByName(ctx, ifName)
	if err != nil {
		s.Error("Failed to get device object from manager object: ", err)
	}

	deviceProps, err := device.GetProperties(ctx)
	if err != nil {
		s.Error("Failed to get device properties: ", err)
	}

	ipConfigPaths, err := deviceProps.GetObjectPaths(shillconst.DevicePropertyIPConfigs)
	if err != nil {
		s.Error("Failed to get ipConfig object paths: ", err)
	}

	for _, path := range ipConfigPaths {
		ip, err := shill.NewIPConfig(ctx, path)
		if err != nil {
			s.Error("Failed to create ipConfig object: ", err)
		}

		p, err := ip.GetProperties(ctx)
		if err != nil {
			s.Error("Failed to get ipConfig properties: ", err)
		}

		ipAddr, err := p.GetString(shillconst.IPConfigPropertyAddress)
		if err != nil {
			s.Error("Failed to get ip address from ipConfig object: ", err)
		}

		if len(strings.Split(ipAddr, ".")) == 4 {
			servers, err := p.GetStrings(shillconst.IPConfigPropertyNameServers)
			if err != nil {
				s.Error("Failed to get nameserver from ipConfig object: ", err)
			}
			if servers[0] == gateway {
				return
			}
		}
	}

	s.Errorf("Unexpected nameserver result: there is no nameserver %s ", gateway)
}

func captureHTTPProbe(ctx context.Context, s *testing.State, ifName string, m *shill.Manager, r *tcpdump.Runner) {
	s.Log("Start executing tcpdump command")
	outDir, ok := testing.ContextOutDir(ctx)
	if !ok {
		s.Error("outdir not found")
	}
	stdoutFile, err := prepareDirFile(ctx, path.Join(outDir, tcpdumpStdOutFile))
	if err != nil {
		s.Error("Failed to open stdout log of tcpdump: ", err)
	}
	stderrFile, err := prepareDirFile(ctx, path.Join(outDir, tcpdumpStdErrFile))
	if err != nil {
		s.Error("Failed to open stderr log of tcpdump: ", err)
	}

	err = r.StartTcpdump(ctx, ifName, path.Join(outDir, tcpdumpFile), stdoutFile, stderrFile)
	if err != nil {
		s.Fatal("Failed to start tcpdump: ", err)
	}

	s.Log("Restart portal check")
	if err := m.RecheckPortal(ctx); err != nil {
		s.Fatal("Failed to invoke RecheckPortal on shill: ", err)
	}

	defer func(ctx context.Context) {
		r.Close(ctx)
	}(ctx)
	ctx, cancel := r.ReserveForClose(ctx)
	defer cancel()
}

func resolveHTTPProbe(ctx context.Context, s *testing.State, r *tcpdump.Runner) {
	out, err := r.OutputTCP(ctx)
	if err != nil {
		s.Error("Tcpdump read failed: ", err)
	}

	regexStr := fmt.Sprintf(">.*%s.*http", resolveHostToIP)
	match, _ := regexp.MatchString(regexStr, string(out))
	if !match {
		s.Errorf("Unexpected tcpdump result: got %s which doesn't match %s", out, regexStr)
	}

}

// prepareDirFile prepares the base directory of the path under dir and opens the file.
func prepareDirFile(ctx context.Context, filename string) (*os.File, error) {
	if err := os.MkdirAll(path.Dir(filename), 0755); err != nil {
		return nil, errors.Wrapf(err, "failed to create basedir for %q", filename)
	}
	f, err := os.OpenFile(filename, os.O_WRONLY|os.O_CREATE, 0644)
	if err != nil {
		return nil, errors.Wrapf(err, "cannot open file %q", filename)
	}
	return f, nil
}
