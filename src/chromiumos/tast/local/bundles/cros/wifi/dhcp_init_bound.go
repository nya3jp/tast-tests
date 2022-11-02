// Copyright 2022 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package wifi

import (
	"context"
	"os"
	"path"
	"time"

	"chromiumos/tast/ctxutil"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/bundles/cros/wifi/hwsim"
	"chromiumos/tast/local/network/tcpdump"
	"chromiumos/tast/local/network/virtualnet"
	"chromiumos/tast/local/network/virtualnet/dnsmasq"
	"chromiumos/tast/local/network/virtualnet/subnet"
	"chromiumos/tast/local/shill"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func: DHCPInitBound,
		Desc: "Verifies DUT supports a minimum set of required protocols",
		Contacts: []string{
			"jiejiang@google.com",
			"cros-networking@google.com",
		},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"wifi", "shill-wifi"},
		LacrosStatus: testing.LacrosVariantUnneeded,
		Fixture:      "shillSimulatedWiFi",
	})
}

type capturer struct {
	runner *tcpdump.Runner
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

func StartCapturer(ctx context.Context, ifname string) (*capturer, error) {
	outDir, ok := testing.ContextOutDir(ctx)
	if !ok {
		return nil, errors.New("failed to prepare out dir")
	}
	stdoutFile, err := prepareDirFile(ctx, path.Join(outDir, "tcpdump.stdout"))
	if err != nil {
		return nil, errors.Wrap(err, "failed to open stdout log of tcpdump")
	}
	stderrFile, err := prepareDirFile(ctx, path.Join(outDir, "tcpdump.stderr"))
	if err != nil {
		return nil, errors.Wrap(err, "failed to open stderr log of tcpdump")
	}

	runner := tcpdump.NewLocalRunner()
	if err := runner.StartTcpdump(ctx, ifname, path.Join(outDir, "tcpdump.pcap"), stdoutFile, stderrFile); err != nil {
		return nil, errors.Wrap(err, "failed to start tcpdump")
	}

	return &capturer{runner}, nil
}

func (c *capturer) Stop(ctx context.Context) {
	c.runner.Close(ctx)
}

func DHCPInitBound(ctx context.Context, s *testing.State) {
	//Use a shortened context for test operations to reserve time for cleanup.
	cleanupCtx := ctx
	ctx, cancel := ctxutil.Shorten(ctx, 10*time.Second)
	defer cancel()

	m, err := shill.NewManager(ctx)
	if err != nil {
		s.Fatal("Failed to create shill manager proxy: ", err)
	}

	simWiFi := s.FixtValue().(*hwsim.ShillSimulatedWiFi)
	pool := subnet.NewPool()
	wifi, err := virtualnet.CreateWifiRouterEnv(ctx, simWiFi.AP[0], m, pool, virtualnet.EnvOptions{})
	if err != nil {
		s.Fatal("Failed to create virtual WiFi router: ", err)
	}
	defer func() {
		if err := wifi.Cleanup(cleanupCtx); err != nil {
			s.Error("Failed to clean up virtual WiFi router")
		}
	}()

	subnet, err := pool.AllocNextIPv4Subnet()
	if err != nil {
		s.Fatal("Failed to allocate subnet for DHCP: ", err)
	}
	dnsmasqServer := dnsmasq.New(dnsmasq.WithDHCPServer(subnet))
	if err := wifi.Router.StartServer(ctx, "dnsmasq", dnsmasqServer); err != nil {
		s.Fatal("Failed to start dnsmasq: ", err)
	}

	printLeases := func() {
		leases, err := dnsmasqServer.GetLeases(ctx)
		if err != nil {
			s.Fatal("Failed to get DHCP leases: ", err)
		}
		testing.ContextLogf(ctx, "Print leases")
		for _, l := range leases {
			testing.ContextLogf(ctx, "%s %s", l.IP, l.Hostname)
		}
	}

	c, _ := StartCapturer(ctx, simWiFi.Client[0])
	defer c.Stop(cleanupCtx)

	// connect service, there should be DISCOVER and REQUEST
	if err := wifi.Service.Connect(ctx); err != nil {
		s.Fatal("Failed to connect to the WiFi service: ", err)
	}
	if err := wifi.Service.WaitForConnectedOrError(ctx); err != nil {
		s.Fatal("Failed to wait for WiFi service be connected: ", err)
	}

	printLeases()

	// reconnect service, there should only be a REQUEST
	if err := wifi.Service.Disconnect(ctx); err != nil {
		s.Fatal("Failed to disconnect from the WiFi service: ", err)
	}
	if err := wifi.Service.Connect(ctx); err != nil {
		s.Fatal("Failed to reconnect to the WiFi service: ", err)
	}
	if err := wifi.Service.WaitForConnectedOrError(ctx); err != nil {
		s.Fatal("Failed to wait for WiFi service be connected: ", err)
	}

	printLeases()

	// testing.Sleep(ctx, 1*time.Minute)
}
