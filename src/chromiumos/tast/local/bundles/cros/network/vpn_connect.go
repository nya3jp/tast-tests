// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package network

import (
	"context"
	"os"
	"os/exec"
	"syscall"
	"time"

	"chromiumos/tast/common/network/ping"
	"chromiumos/tast/common/shillconst"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/bundles/cros/network/veth"
	"chromiumos/tast/local/bundles/cros/network/vpn"
	"chromiumos/tast/local/network"
	localping "chromiumos/tast/local/network/ping"
	"chromiumos/tast/local/shill"
	"chromiumos/tast/local/upstart"
	"chromiumos/tast/testing"
	"chromiumos/tast/timing"
)

type vpnServer struct {
	vpnType  string
	authType string
}

func init() {
	testing.AddTest(&testing.Test{
		Func:     VPNConnect,
		Desc:     "Ensure that we can connect to a VPN",
		Contacts: []string{"arowa@google.com", "cros-networking@google.com"},
		Attr:     []string{"group:mainline", "informational"},
		Params: []testing.Param{{
			Name: "l2tp_ipsec_psk",
			Val: vpnServer{
				vpnType:  "L2TP/IPsec",
				authType: "psk",
			},
		}},
	})
}

const (
	l2tpIPsec              = "L2TP/IPsec"
	pskAuth                = "psk"
	testDefaultProfileName = "vpnTestProfile"
	testUserProfileName    = "vpnTestProfile2"
	clientInterfaceName    = "pseudoethernet0"
	serverInterfaceName    = "serverethernet0"
	version                = 1
	serverAddress          = "10.9.8.1"
	clientAddress          = "10.9.8.2"
	networkPrefix          = 24
)

func VPNConnect(ctx context.Context, s *testing.State) {
	// We lose connectivity along the way here, and if that races with the
	// recover_duts network-recovery hooks, it may interrupt us.
	unlock, err := network.LockCheckNetworkHook(ctx)
	if err != nil {
		s.Fatal("Failed to lock the check network hook: ", err)
	}
	defer unlock()

	func() {
		// Stop shill temporarily and remove the default profile.
		if err := upstart.StopJob(ctx, "shill"); err != nil {
			s.Fatal("Failed stopping shill: ", err)
		}
		defer func() {
			if err := upstart.RestartJob(ctx, "shill"); err != nil {
				s.Fatal("Failed starting shill: ", err)
			}
		}()

		if err := os.Remove(shillconst.DefaultProfilePath); err != nil && !os.IsNotExist(err) {
			s.Fatal("Failed removing default profile: ", err)
		}
	}()

	manager, err := shill.NewManager(ctx)
	if err != nil {
		s.Fatal("Failed creating shill manager proxy: ", err)
	}
	// Remove test profiles in case they already exist.
	manager.RemoveProfile(ctx, testDefaultProfileName)
	manager.RemoveProfile(ctx, testUserProfileName)

	// Clean up custom services and test profiles on exit.
	defer func() {
		manager.PopProfile(ctx, testUserProfileName)
		manager.RemoveProfile(ctx, testUserProfileName)
		manager.PopProfile(ctx, testDefaultProfileName)
		manager.RemoveProfile(ctx, testDefaultProfileName)

		upstart.StopJob(ctx, "shill")
		os.Remove(shillconst.DefaultProfilePath)
		upstart.RestartJob(ctx, "shill")
	}()

	// Pop user profiles and push a temporary default profile on top.
	s.Log("Popping all user profiles and pushing new default profile")
	if err = manager.PopAllUserProfiles(ctx); err != nil {
		s.Fatal("Failed to pop user profiles: ", err)
	}
	if _, err = manager.CreateProfile(ctx, testDefaultProfileName); err != nil {
		s.Fatal("Failed to create profile: ", err)
	}
	if _, err = manager.PushProfile(ctx, testDefaultProfileName); err != nil {
		s.Fatal("Failed to push profile: ", err)
	}

	// Prepare virtual ethernet link.
	if _, err := veth.NewPair(ctx, serverInterfaceName, clientInterfaceName); err != nil {
		s.Fatal("Failed to setup veth: ", err)
	}

	vpnType := s.Param().(vpnServer).vpnType
	authType := s.Param().(vpnServer).authType

	if vpnType == l2tpIPsec {
		// Create new L2TP/IPsec.
		server := vpn.NewL2tpipSecVpnServer(ctx, authType, serverInterfaceName, serverAddress, networkPrefix)
		if err := server.StartServer(ctx); err != nil {
			s.Fatal("Failed to create a L2TP/IPsec server: ", err)
		}
		defer func() {
			if err := server.Exit(ctx); err != nil {
				s.Fatal("Failed to Stop a L2TP/IPsec server: ", err)
			}
		}()
	} else {
		s.Fatalf("Unexpected VPN type %s", vpnType)
	}

	// When shill finds this ethernet interface, it will reset its IP address and start a DHCP client.
	// We must configure the static IP address through shill.
	if err := configureStaticIP(ctx, clientInterfaceName, clientAddress, manager); err != nil {
		s.Fatal("Failed configuring the static IP: ", err)
	}

	if err := connectVPN(ctx, vpnType, authType, serverAddress, manager); err != nil {
		s.Fatal("Failed connecting to VPN server: ", err)
	}

	pr := localping.NewLocalRunner()
	res, err := pr.Ping(ctx, vpn.Xl2tpdServerIPAddress, ping.Count(3), ping.User("chronos"))
	if err != nil {
		s.Fatal("Failed pinging the server IPv4: ", err)
	}
	if res.Received == 0 {
		s.Fatalf("Failed to ping %s: no response received", vpn.Xl2tpdServerIPAddress)
	}

	// IPv6 should be blackholed, so ping returns
	// "other error"
	isExitCode := func(err error, code int) bool {
		var exitErr *exec.ExitError
		if ok := errors.As(err, &exitErr); ok {
			return exitErr.Sys().(syscall.WaitStatus).ExitStatus() == code
		}
		return false
	}
	if _, err := pr.Ping(ctx, "2001:db8::1", ping.Count(1), ping.User("chronos")); !isExitCode(err, 2) {
		s.Fatal("Failed IPv6 ping should have aborted: ", err)
	}

}

// configureStaticIP configures the Static IP parameters for the Ethernet interface |interface_name| and applies
// those parameters to the interface by forcing a re-connect.
func configureStaticIP(ctx context.Context, interfaceName, address string, manager *shill.Manager) error {
	// Wait for static IP to be configured.
	testing.ContextLog(ctx, "Wait for static IP to be configured")
	ctx, st := timing.Start(ctx, "waitConfigureStaticIP")
	defer st.End()
	if err := testing.Poll(ctx, func(ctx context.Context) error {
		device, err := manager.WaitForDeviceByName(ctx, interfaceName, 5*time.Second)
		if err != nil {
			return errors.Wrapf(err, "failed to find the device with interface name %s", interfaceName)
		}

		deviceProp, err := device.GetProperties(ctx)
		if err != nil {
			return errors.Wrapf(err, "failed to get properties of device %v", device)
		}

		servicePath, err := deviceProp.GetObjectPath(shillconst.DevicePropertySelectedService)
		if err != nil {
			return errors.Wrapf(err, "failed to get the DBus object path for the property %s", shillconst.DevicePropertySelectedService)
		}

		service, err := shill.NewService(ctx, servicePath)
		if err != nil {
			return errors.Wrap(err, "failed creating shill service proxy")
		}

		if err := service.SetProperty(ctx, shillconst.ServicePropertyStaticIPConfig, map[string]interface{}{shillconst.IPConfigPropertyAddress: address, "Prefixlen": networkPrefix}); err != nil {
			return errors.Wrap(err, "failed to configure the static IP address")
		}

		// Device::OnIPConfigUpdated doesn't cause an Online Service to change state,
		// as this would lead to fluctuations of what the default Service is every time
		// a DHCP lease is renewed. So in this case we need to wait for routing to be
		// re-established, but don't have a good D-Bus property to poll. Because of that,
		// we need to disconnect/connect the service to make sure the routing rules are re-stablished.
		if err = service.Disconnect(ctx); err != nil {
			return errors.Wrapf(err, "failed to dis-connect the service %v", service)
		}

		if err = service.Connect(ctx); err != nil {
			return errors.Wrap(err, "failed to re-connect after configuring the static IP")
		}

		return nil
	}, &testing.PollOptions{Timeout: 100 * time.Second, Interval: 1 * time.Second}); err != nil {
		return errors.Wrap(err, "failed to wait for static IP to be configured")
	}

	return nil
}

// getVpnClientProperties returns VPN configuration properties.
func getVpnClientProperties(ctx context.Context, vpnType, authType, serverAddress string) (map[string]interface{}, error) {
	if (vpnType == l2tpIPsec) && (authType == pskAuth) {
		params := map[string]interface{}{
			"L2TPIPsec.Password": vpn.ChapSecret,
			"L2TPIPsec.PSK":      vpn.IPsecPresharedKey,
			"L2TPIPsec.User":     vpn.ChapUser,
			"Name":               "test-vpn-l2tp-psk",
			"Provider.Host":      serverAddress,
			"Provider.Type":      "l2tpipsec",
			"Type":               "vpn",
		}

		return params, nil
	}

	return nil, errors.Errorf("unexpected server type: got %s-%s, want L2TP/IPsec-psk", vpnType, authType)
}

// connectVPN connects the client to the VPN server.
func connectVPN(ctx context.Context, vpnType, authType, serverAddress string, manager *shill.Manager) error {
	vpnProps, err := getVpnClientProperties(ctx, vpnType, authType, serverAddress)
	if err != nil {
		return err
	}

	servicePath, err := manager.ConfigureService(ctx, vpnProps)
	if err != nil {
		return errors.Wrapf(err, "unable to configure the service for the VPN properties %v", vpnProps)
	}

	service, err := shill.NewService(ctx, servicePath)
	if err != nil {
		return errors.Wrap(err, "failed creating shill service proxy")
	}

	// Wait for service to be connected.
	testing.ContextLog(ctx, "Connecting to service: ", service)

	// Spawn watcher before connect.
	pw, err := service.CreateWatcher(ctx)
	if err != nil {
		return errors.Wrap(err, "failed to create watcher")
	}
	defer pw.Close(ctx)

	if err = service.Connect(ctx); err != nil {
		return errors.Wrapf(err, "failed to connect the service %v", service)
	}

	// Wait until connection established.
	// According to previous Autotest tests, a reasonable timeout is
	// 15 seconds for association and 15 seconds for configuration.
	timeoutCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	if err := pw.Expect(timeoutCtx, shillconst.ServicePropertyIsConnected, true); err != nil {
		return err
	}

	return nil
}
