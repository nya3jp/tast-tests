// Copyright 2022 The ChromiumOS Authors.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package network

import (
	"context"
	"net"
	"os"
	"path"
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
	// Relying on shillReset test fixture to undo the enabling of portal detection.
	if err := m.EnablePortalDetection(ctx); err != nil {
		s.Fatal("Enable Portal Detection failed: ", err)
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

	nameServer, err := waitForShillNameServer(ctx, svc)
	if err != nil {
		s.Fatal("Failed to wait for shill name server: ", err)
	}
	if nameServer != ifaceAddrs.IPv4Addr.String() {
		s.Errorf("Unexpected nameserver result: got %s but want %s",
			nameServer,
			ifaceAddrs.IPv4Addr.String())
	}

	r := localTcpdump.NewLocalRunner()

	s.Log("Start executing tcpdump command")
	err = captureHTTPProbe(ctx, svc, router.VethOutName, r)
	if err != nil {
		s.Fatal("Failed to capture packets during HTTP probe: ", err)
	}

	args := []string{"dst", resolveHostToIP, "and", "tcp", "dst", "port", "http"}
	out, err := r.Output(ctx, args...)
	if err != nil {
		s.Error("Tcpdump read failed: ", err)
	}
	if out == nil {
		s.Errorf("Unexpected tcpdump result: no HTTP traffic to %s found", resolveHostToIP)
	}
}

func waitForShillNameServer(ctx context.Context, svc *shill.Service) (string, error) {
	if err := svc.WaitForProperty(
		ctx,
		shillconst.ServicePropertyState,
		shillconst.ServiceStateReady,
		5*time.Second); err != nil {
		return "", errors.Wrap(err, "failed to wait for the service ready")
	}

	configs, err := svc.GetIPConfigs(ctx)
	if err != nil {
		return "", errors.Wrap(err, "failed to get IPConfigs from service")
	}

	for _, config := range configs {
		p, err := config.GetProperties(ctx)
		if err != nil {
			return "", errors.Wrap(err, "failed to get IPConfig properties")
		}

		addr, err := p.GetString(shillconst.IPConfigPropertyAddress)
		if err != nil {
			return "", errors.Wrap(err, "failed to get IP address from IPConfig object")
		}

		ip := net.ParseIP(addr)
		if ip != nil && ip.To4() != nil {
			servers, err := p.GetStrings(shillconst.IPConfigPropertyNameServers)
			if err != nil {
				return "", errors.Wrap(err, "failed to get nameserver from IPConfig object")
			}
			if len(servers) == 0 {
				return "", errors.New("nameservers is an empty array")
			}
			return servers[0], nil
		}
	}

	return "", errors.Wrap(err, "there is no name server")
}

func captureHTTPProbe(ctx context.Context, svc *shill.Service, ifName string, r *tcpdump.Runner) error {
	outDir, ok := testing.ContextOutDir(ctx)
	if !ok {
		return errors.New("outdir not found")
	}
	stdoutFile, err := prepareDirFile(ctx, path.Join(outDir, tcpdumpStdOutFile))
	if err != nil {
		return errors.Wrap(err, "failed to open stdout log of tcpdump")
	}
	stderrFile, err := prepareDirFile(ctx, path.Join(outDir, tcpdumpStdErrFile))
	if err != nil {
		return errors.Wrap(err, "failed to open stderr log of tcpdump")
	}

	err = r.StartTcpdump(ctx, ifName, path.Join(outDir, tcpdumpFile), stdoutFile, stderrFile)
	if err != nil {
		return errors.Wrap(err, "failed to start tcpdump")
	}
	defer func(ctx context.Context) {
		r.Close(ctx)
	}(ctx)
	ctx, cancel := r.ReserveForClose(ctx)
	defer cancel()

	testing.ContextLog(ctx, "Start to trigger portal detection")
	if err := svc.SetProperty(ctx, shillconst.ServicePropertyCheckPortal, "false"); err != nil {
		return errors.Wrap(err, "failed to set CheckPortal to false")
	}
	if err := svc.WaitForProperty(
		ctx,
		shillconst.ServicePropertyState,
		shillconst.ServiceStateOnline,
		5*time.Second); err != nil {
		return errors.Wrap(err, "failed to wait for the service online")
	}

	if err := svc.SetProperty(ctx, shillconst.ServicePropertyCheckPortal, "true"); err != nil {
		return errors.Wrap(err, "failed to set CheckPortal to true")
	}
	if err := svc.WaitForProperty(
		ctx,
		shillconst.ServicePropertyState,
		shillconst.ServiceStateNoConnectivity,
		15*time.Second); err != nil {
		return errors.Wrap(err, "failed to wait for the service no-connectivity")
	}

	return nil
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
