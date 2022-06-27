// Copyright 2022 The ChromiumOS Authors.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package network

import (
	"bytes"
	"context"
	"fmt"
	"net"
	"os"
	"path"
	"strings"
	"time"

	"chromiumos/tast/common/network/tcpdump"
	"chromiumos/tast/common/shillconst"
	"chromiumos/tast/common/testexec"
	"chromiumos/tast/errors"
	resetshill "chromiumos/tast/local/bundles/cros/network/shill"
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
	})
}

const (
	dnsResolvedIPAddr  = "129.0.0.1"
	resovledDomainName = "google.com"
	tcpdumpStdErrFile  = "/var/log/pcap-dnsmasq.stderr"
	tcpdumpStdOutFile  = "/var/log/pcap-dnsmasq.stdout"
	tcpdumpFile        = "/var/log/pcap-dnsmasq.pcap"
)

func ShillCaptivePortalDNS(ctx context.Context, s *testing.State) {
	testing.ContextLog(ctx, "Restarting shill")
	if err := resetshill.ResetShill(ctx); err != nil {
		s.Fatal("Failed to reset shill")
	}

	m, err := shill.NewManager(ctx)
	if err != nil {
		s.Fatal("Failed to create manager proxy: ", err)
	}

	testing.ContextLog(ctx, "Enabling portal detection on ethernet")
	if err := m.SetProperty(ctx, shillconst.ProfilePropertyCheckPortalList, "wifi,cellular,ethernet"); err != nil {
		s.Fatal("Failed to enable portal detection on ethernet: ", err)
	}

	testing.ContextLog(ctx, "Setting up a netns for router")
	pool := subnet.NewPool()
	_, router, err := virtualnet.CreateRouterEnv(ctx, m, pool, virtualnet.EnvOptions{
		Priority:         5,
		NameSuffix:       "",
		EnableDHCP:       true,
		EnableDNS:        true,
		RAServer:         false,
		AddressToForceIP: dnsResolvedIPAddr,
	})
	if err != nil {
		s.Fatal("Failed to create a router env: ", err)
	}
	defer router.Cleanup(ctx)

	// ifName is device's interface name and ip is subnet ip.
	ifName, ip := virtualnet.GetDNSMasqParams()
	gateway := net.IPv4(ip[0], ip[1], ip[2], 1).String()

	resolveHost(ctx, s, gateway)
	resolveShillNameServer(ctx, s, ifName, gateway, m)
	r := localTcpdump.NewLocalRunner()
	captureHTTPProbe(ctx, s, ifName, m, r)
	resolveHTTPProbe(ctx, s, r)
}

func resolveHost(ctx context.Context, s *testing.State, gateway string) {
	// Confirm that the DNS functionality of the DNSMasq service is working.
	s.Log("Attempt to resolve host from DNS server")
	out, err := testexec.CommandContext(ctx, "dig", resovledDomainName, fmt.Sprintf("@%s", gateway)).Output()
	if err != nil {
		s.Error("Dig command failed: ", err)
	} else if !bytes.Contains(out, []byte(dnsResolvedIPAddr)) {
		s.Errorf("Unexpected dig result: got %s which doesn't include %s", out, dnsResolvedIPAddr)
	}
}

func resolveShillNameServer(ctx context.Context, s *testing.State, ifName, gateway string, m *shill.Manager) {
	// Use the ipconfig api to resolve the nameserver used by shill.
	s.Log("Attempt to get the nameserver used by shill")
	device, err := m.DeviceByName(ctx, ifName)
	if err != nil {
		s.Error("Failed to get device object from manager object: ", err)
	}

	// Waiting for a connection to the selected service.
	servicePath, err := device.WaitForSelectedService(ctx, 5*time.Second)
	if err != nil {
		s.Fatal("Failed to get service path: ", err)
	}

	service, err := shill.NewService(ctx, servicePath)
	if err != nil {
		s.Fatal("Failed to get service object: ", err)
	}

	waitServiceReady(ctx, service)

	deviceProps, err := device.GetProperties(ctx)
	if err != nil {
		s.Error("Failed to get device properties: ", err)
	}

	ipConfigPaths, err := deviceProps.GetObjectPaths(shillconst.DevicePropertyIPConfigs)
	if err != nil {
		s.Error("Failed to get ipConfig object paths: ", err)
	}

	var nameservers []string
	for _, path := range ipConfigPaths {
		ip, err := shill.NewIPConfig(ctx, path)
		if err != nil {
			s.Error("Failed to create ipConfig object: ", err)
		}

		p, err := ip.GetProperties(ctx)
		if err != nil {
			s.Error("Failed to get ipConfig properties: ", err)
		}
		servers, err := p.GetStrings(shillconst.IPConfigPropertyNameServers)
		if err != nil {
			s.Error("Failed to get nameserver from ipConfig object: ", err)
		} else {
			nameservers = append(nameservers, servers...)
		}
	}

	if !stringInSlice(gateway, nameservers) {
		s.Errorf("Unexpected nameserver result: got %s; want %s", strings.Join(nameservers, ","), gateway)
	}
}

func captureHTTPProbe(ctx context.Context, s *testing.State, ifName string, m *shill.Manager, r *tcpdump.Runner) {
	s.Log("Start executing tcpdump command")
	stdoutFile, err := prepareDirFile(ctx, tcpdumpStdOutFile)
	if err != nil {
		s.Error("Failed to open stdout log of tcpdump: ", err)
	}
	stderrFile, err := prepareDirFile(ctx, tcpdumpStdErrFile)
	if err != nil {
		s.Error("Failed to open stderr log of tcpdump: ", err)
	}

	err = r.StartTcpdump(ctx, ifName, tcpdumpFile, stdoutFile, stderrFile)
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
	} else if !bytes.Contains(out, []byte(dnsResolvedIPAddr)) {
		s.Errorf("Unexpected tcpdump result: got %s which doesn't include %s", out, dnsResolvedIPAddr)
	}
}

// waitServiceReady will wait for the status of the service to become ready.
func waitServiceReady(ctx context.Context, service *shill.Service) error {
	pw, err := service.CreateWatcher(ctx)
	if err != nil {
		return errors.Wrap(err, "failed to create watcher")
	}
	defer pw.Close(ctx)

	serviceProps, err := service.GetProperties(ctx)
	if err != nil {
		return errors.Wrap(err, "failed to get service properties")
	}

	state, err := serviceProps.GetString(shillconst.ServicePropertyState)
	if err != nil {
		return errors.Wrap(err, "failed to get service's state from properties")
	}

	if state != shillconst.ServiceStateReady {
		timeoutCtx, cancel := context.WithTimeout(ctx, time.Duration(10*time.Second))
		defer cancel()

		if err := pw.Expect(timeoutCtx, shillconst.ServicePropertyState, shillconst.ServiceStateReady); err != nil {
			return errors.Wrap(err, "failed to wait for service to be ready")
		}
	}

	return nil
}

// stringInSlice will check if a specific string is in an array of strings.
func stringInSlice(target string, list []string) bool {
	for _, element := range list {
		if element == target {
			return true
		}
	}
	return false
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
