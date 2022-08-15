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
	"chromiumos/tast/ctxutil"
	"chromiumos/tast/errors"
	localTcpdump "chromiumos/tast/local/network/tcpdump"
	"chromiumos/tast/local/network/virtualnet"
	"chromiumos/tast/local/network/virtualnet/subnet"
	"chromiumos/tast/local/shill"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:     ShillCaptivePortalDNS,
		Desc:     "Ensure that shill sends portal detection probes to the IP address given by dnsmasq",
		Contacts: []string{"tinghaolin@google.com", "cros-network-health-team@google.com"},
		Attr:     []string{"group:mainline", "informational"},
		Fixture:  "shillReset",
	})
}

const (
	resolveHostToIP   = "129.0.0.1"
	tcpdumpStderrFile = "pcap-dnsmasq.stderr"
	tcpdumpStdoutFile = "pcap-dnsmasq.stdout"
	tcpdumpFile       = "pcap-dnsmasq.pcap"
)

func ShillCaptivePortalDNS(ctx context.Context, s *testing.State) {
	manager, err := shill.NewManager(ctx)
	if err != nil {
		s.Fatal("Failed to create manager proxy: ", err)
	}

	testing.ContextLog(ctx, "Enabling portal detection on ethernet")
	// Relying on shillReset test fixture to undo the enabling of portal detection.
	if err := manager.EnablePortalDetection(ctx); err != nil {
		s.Fatal("Enable Portal Detection failed: ", err)
	}

	testing.ContextLog(ctx, "Setting up a netns for router")
	pool := subnet.NewPool()
	svc, router, err := virtualnet.CreateRouterEnv(ctx, manager, pool, virtualnet.EnvOptions{
		Priority:        5,
		NameSuffix:      "",
		EnableDHCP:      true,
		EnableDNS:       true,
		RAServer:        false,
		ResolveHostToIP: net.ParseIP(resolveHostToIP),
	})
	if err != nil {
		s.Fatal("Failed to create a router env: ", err)
	}
	cleanupCtx := ctx
	ctx, cancel := ctxutil.Shorten(ctx, 5*time.Second)
	defer cancel()
	defer router.Cleanup(cleanupCtx)

	ifaceAddrs, err := router.WaitForVethInAddrs(ctx, true, false)
	if err != nil {
		s.Fatal("Failed to wait for IP addresses on the inside interface: ", err)
	}

	if nameServer, err := waitForShillNameServer(ctx, svc); err != nil {
		s.Fatal("Failed to wait for shill name server: ", err)
	} else if nameServer != ifaceAddrs.IPv4Addr.String() {
		s.Errorf("Unexpected nameserver result: got %s want %s",
			nameServer,
			ifaceAddrs.IPv4Addr.String())
	}

	runner := localTcpdump.NewLocalRunner()

	testing.ContextLog(ctx, "Start executing tcpdump command")
	if err := captureHTTPProbe(ctx, svc, router.VethOutName, runner); err != nil {
		s.Fatal("Failed to capture packets during HTTP probe: ", err)
	}

	args := []string{"dst", resolveHostToIP, "and", "tcp", "dst", "port", "http"}
	if out, err := runner.Output(ctx, args...); err != nil {
		s.Error("Tcpdump read failed: ", err)
	} else if out == nil {
		s.Errorf("Unexpected tcpdump result: no HTTP traffic to %s found", resolveHostToIP)
	}
}

func waitForShillNameServer(ctx context.Context, svc *shill.Service) (string, error) {
	pw, err := svc.CreateWatcher(ctx)
	if err != nil {
		return "", errors.Wrap(err, "failed to create watcher")
	}
	defer pw.Close(ctx)

	timeoutCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	_, err = pw.ExpectIn(timeoutCtx, shillconst.ServicePropertyState, shillconst.ServiceConnectedStates)
	if err != nil {
		return "", errors.Wrap(err, "service state is not a connected state")
	}

	configs, err := svc.GetIPConfigs(ctx)
	if err != nil {
		return "", errors.Wrap(err, "failed to get IPConfigs from service")
	}

	for _, config := range configs {
		props, err := config.GetIPProperties(ctx)
		if err != nil {
			return "", errors.Wrap(err, "failed to get IPConfig properties")
		}

		ip := net.ParseIP(props.Address)
		if ip == nil || ip.To4() == nil {
			continue
		}

		if len(props.NameServers) == 0 {
			return "", errors.New("nameservers is an empty array")
		}
		// By setting the dnsmasq configuration, the dnsmasq service tells the DHCP client that
		// the DNS server is set to the address of the machine running dnsmasq. In this test,
		// IPConfig for IPv4 should contain only one name server so we return that name server here.
		return props.NameServers[0], nil
	}

	return "", errors.Wrap(err, "no nameserver")
}

func captureHTTPProbe(ctx context.Context, svc *shill.Service, ifName string, runner *tcpdump.Runner) error {
	outDir, ok := testing.ContextOutDir(ctx)
	if !ok {
		return errors.New("outdir not found")
	}
	stdoutFile, err := prepareDirFile(ctx, path.Join(outDir, tcpdumpStdoutFile))
	if err != nil {
		return errors.Wrap(err, "failed to open stdout log of tcpdump")
	}
	stderrFile, err := prepareDirFile(ctx, path.Join(outDir, tcpdumpStderrFile))
	if err != nil {
		return errors.Wrap(err, "failed to open stderr log of tcpdump")
	}

	if err := runner.StartTcpdump(ctx, ifName, path.Join(outDir, tcpdumpFile), stdoutFile, stderrFile); err != nil {
		return errors.Wrap(err, "failed to start tcpdump")
	}

	cleanupCtx := ctx
	ctx, cancel := runner.ReserveForClose(ctx)
	defer cancel()
	defer func(cleanupCtx context.Context) {
		runner.Close(cleanupCtx)
	}(cleanupCtx)

	testing.ContextLog(ctx, "Start to trigger portal detection")
	if err := svc.SetProperty(ctx, shillconst.ServicePropertyCheckPortal, "false"); err != nil {
		return errors.Wrap(err, "failed to set CheckPortal to false")
	}
	if err := svc.WaitForProperty(ctx, shillconst.ServicePropertyState, shillconst.ServiceStateOnline, shillconst.DefaultTimeout); err != nil {
		return errors.Wrap(err, "failed to wait for the service online")
	}

	if err := svc.SetProperty(ctx, shillconst.ServicePropertyCheckPortal, "true"); err != nil {
		return errors.Wrap(err, "failed to set CheckPortal to true")
	}
	if err := svc.WaitForProperty(ctx, shillconst.ServicePropertyState, shillconst.ServiceStateNoConnectivity, shillconst.DefaultTimeout); err != nil {
		return errors.Wrap(err, "failed to wait for the service no-connectivity")
	}

	return nil
}

// prepareDirFile prepares the base directory for filename and opens the file.
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
