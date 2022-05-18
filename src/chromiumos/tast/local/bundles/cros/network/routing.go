// Copyright 2022 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package network

import (
	"context"
	"time"

	"chromiumos/tast/common/network/ping"
	"chromiumos/tast/common/shillconst"
	"chromiumos/tast/common/testexec"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/bundles/cros/network/netns"
	networkshill "chromiumos/tast/local/bundles/cros/network/shill"
	localping "chromiumos/tast/local/network/ping"
	"chromiumos/tast/local/shill"
	shillDBus "chromiumos/tast/local/shill"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         Routing,
		Desc:         "TODO",
		Contacts:     []string{"jiejiang@google.com", "cros-networking@google.com"},
		Attr:         []string{},
		LacrosStatus: testing.LacrosVariantUnneeded,
	})
}

const (
	ethName = "eth0"
)

func Routing(ctx context.Context, s *testing.State) {
	testing.ContextLog(ctx, "Restarting shill")

	if err := networkshill.ResetShill(ctx); err != nil {
		s.Fatal("Failed to reset shill")
	}

	manager, err := shillDBus.NewManager(ctx)
	if err != nil {
		s.Fatal("Failed to create manager proxy: ", err)
	}

	testing.ContextLog(ctx, "Disabling portal detection on ethernet")
	if err := manager.SetProperty(ctx, shillconst.ProfilePropertyCheckPortalList, "wifi,cellular"); err != nil {
		s.Fatal("Failed to disable portal detection on ethernet: ", err)
	}

	// testing.ContextLog(ctx, "Claiming eth0 interface from shill")
	// if err := claimEthernetInterface(ctx, manager); err != nil {
	// 	s.Fatal("Failed to claim ethernet service: ", err)
	// }

	testing.ContextLog(ctx, "Setting up a netns for router")
	router := netns.NewNetNSEnv("router")
	if err := router.Startup(ctx); err != nil {
		s.Fatal("Failed to start up router netns: ", err)
	}
	defer router.Shutdown(ctx)

	testing.ContextLog(ctx, "Starting radvd")
	radvdCfgFile := "radvd.conf"
	ipv6Prefix := "2001:db7::/64"
	radvdCfg := "interface " + router.VethInName + ` {
	MinRtrAdvInterval 3;
	MaxRtrAdvInterval 4;
	AdvSendAdvert on;
	AdvManagedFlag on;
	prefix ` + ipv6Prefix + ` { AdvValidLifetime 14300; AdvPreferredLifetime 14200; };
	RDNSS 2001:4860:4860::8888 {
		AdvRDNSSLifetime 7200;
	};
};`
	if err := router.WriteTempFile(ctx, radvdCfgFile, radvdCfg); err != nil {
		s.Fatal(ctx, "Failed to write radvd config file: ", err)
	}
	if err := router.StartCommand(ctx, "/usr/local/sbin/radvd",
		"-n", "-d", "4", "-C", "/tmp/"+radvdCfgFile, "-p", "/tmp/radvd.pid"); err != nil {
		s.Fatal(ctx, "Failed to start radvd: ", err)
	}

	testing.ContextLog(ctx, "Configuring shill service")
	if err := setServicePriority(ctx, manager, router.VethOutName, 100); err != nil {
		s.Fatal(ctx, "Failed to set service priority: ", err)
	}

	testing.ContextLog(ctx, "Setting up a netns for server")
	server := netns.NewNetNSEnv("server")
	if err := server.Startup(ctx); err != nil {
		s.Fatal("Failed to start up server netns: ", err)
	}
	defer server.Shutdown(ctx)

	// connect router to the server
	if err := testexec.CommandContext(ctx, "ip", "link", "set", server.VethOutName, "netns", router.NetNSName).Run(); err != nil {
		s.Fatal("Failed to move out if of server into netns of router: ", err)
	}

	routerAddr := "fd10::1"
	serverAddr := "fd10::2"
	if err := router.RunChroot(ctx, []string{"/bin/ip", "addr", "add", "dev", server.VethOutName, routerAddr + "/64"}); err != nil {
		s.Fatal("Failed to add addr in router ns: ", err)
	}
	if err := server.RunChroot(ctx, []string{"/bin/ip", "addr", "add", "dev", server.VethInName, serverAddr + "/64"}); err != nil {
		s.Fatal("Failed to add addr in server ns: ", err)
	}
	if err := router.RunChroot(ctx, []string{"/bin/ip", "link", "set", "dev", server.VethOutName, "up"}); err != nil {
		s.Fatal("Failed to bring out interface up of server ns: ", err)
	}
	if err := server.RunChroot(ctx, []string{"/bin/ip", "route", "add", ipv6Prefix, "via", routerAddr}); err != nil {
		s.Fatal("Failed to install route in server ns: ", err)
	}

	testing.ContextLog(ctx, "Sleep for 5 seconds to wait for shill ready")
	testing.Sleep(ctx, 5*time.Second)

	// check that we can ping serverAddr
	if err := expectPingSuccess(ctx, serverAddr); err != nil {
		s.Fatalf("Failed to ping %s: %v", router.VethOutName, err)
	}

	// reset link local address in the router ns
	testing.ContextLog(ctx, "Changing mac address and link local address")
	if err := router.RunChroot(ctx, []string{"/bin/ip", "link", "set", "dev", router.VethInName, "addr", "22:73:4a:00:00:00"}); err != nil {
		s.Fatal("Failed to change mac address: ", err)
	}
	if err := router.RunChroot(ctx, []string{"/bin/ip", "addr", "flush", "dev", router.VethInName, "scope", "link"}); err != nil {
		s.Fatal("Failed to flush link local address on interface: ", err)
	}
	if err := testexec.CommandContext(ctx, "ip", "netns", "exec", router.NetNSName, "sysctl", "-w", "net.ipv6.conf."+router.VethInName+".addr_gen_mode=3").Run(); err != nil {
		s.Fatal("Failed to refresh link local address: ", err)
	}
	// if err := testexec.CommandContext(ctx, "ip", "netns", "exec", router.NetNSName, "sysctl", "-w", "net.ipv6.conf."+router.VethInName+".addr_gen_mode=0").Run(); err != nil {
	// 	s.Fatal("Failed to refresh link local address", err)
	// }

	testing.ContextLog(ctx, "Sleep for 5 seconds to wait for ra packets propogated")
	testing.Sleep(ctx, 5*time.Second)

	// check the ping again
	if err := expectPingSuccess(ctx, serverAddr); err != nil {
		s.Fatalf("Failed to ping %s: %v", router.VethOutName, err)
	}

	testing.ContextLog(ctx, "Test done. Sleep for 100 seconds for manual testings")
	testing.Sleep(ctx, 100*time.Second)
}

func claimEthernetInterface(ctx context.Context, manager *shillDBus.Manager) error {
	// Let shill stop managing the interface
	if err := manager.ClaimInterface(ctx, "tast-test", ethName); err != nil {
		return errors.Wrapf(err, "failed to claim %s", ethName)
	}

	// Enable IPv6 on the interface
	// enableIPv6 := "net.ipv6.conf." + ethName + ".disable_ipv6=0"
	// if err := testexec.CommandContext(ctx, "sysctl", "-w", enableIPv6).Run(); err != nil {
	// 	return errors.Wrapf(err, "failed to enable IPv6 on interface %s", ethName)
	// }

	// acceptRA := "net.ipv6.conf." + ethName + ".accept_ra=2"
	// if err := testexec.CommandContext(ctx, "sysctl", "-w", acceptRA).Run(); err != nil {
	// 	return errors.Wrapf(err, "failed to set accept_ra on interface %s", ethName)
	// }

	// if err := testexec.CommandContext(ctx, "ip", "link", "set", "dev", ethName, "up").Run(); err != nil {
	// 	return errors.Wrapf(err, "failed to bring up %s", ethName)
	// }

	// // remove defualt route
	// if err := testexec.CommandContext(ctx, "ip", "-6", "addr", "flush", "dev", ethName, "to", "default").Run(); err != nil {
	// 	return errors.Wrapf(err, "Failed to flush default route on %s", ethName)
	// }

	// lose connection here...

	return nil
}

func expectPingSuccess(ctx context.Context, addr string) error {
	testing.ContextLog(ctx, "Start to ping ", addr)
	pr := localping.NewLocalRunner()
	res, err := pr.Ping(ctx, addr, ping.Count(3), ping.User("chronos"))
	if err != nil {
		return err
	}
	if res.Received == 0 {
		return errors.New("no response received")
	}
	return nil
}

func setServicePriority(ctx context.Context, m *shill.Manager, ifName string, priority int) error {
	device, err := m.WaitForDeviceByName(ctx, ifName, 5*time.Second)
	if err != nil {
		return errors.Wrapf(err, "failed to find the device with interface name %s", ifName)
	}

	testing.ContextLog(ctx, "Waiting for veth service selected")
	servicePath, err := device.WaitForSelectedService(ctx, 5*time.Second)
	if err != nil {
		return errors.Wrap(err, "failed to get the selected service path")
	}
	testing.ContextLog(ctx, "Selected service: ", servicePath)

	// Configure static IP parameters on the service for this veth. The properties
	// should be applied automatically and bring this service Online.
	service, err := shill.NewService(ctx, servicePath)
	if err != nil {
		return errors.Wrap(err, "failed to create shill service proxy")
	}
	if err := service.SetProperty(ctx, shillconst.ServicePropertyEphemeralPriority, priority); err != nil {
		return errors.Wrap(err, "failed to configure the static IP address")
	}
	return nil
}
