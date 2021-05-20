// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package network

import (
	"context"
	"fmt"
	"os"
	"time"

	"chromiumos/tast/common/crypto/certificate"
	"chromiumos/tast/common/network/ping"
	"chromiumos/tast/common/pkcs11/netcertstore"
	"chromiumos/tast/common/shillconst"
	"chromiumos/tast/ctxutil"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/bundles/cros/network/vpn"
	"chromiumos/tast/local/hwsec"
	"chromiumos/tast/local/network"
	localping "chromiumos/tast/local/network/ping"
	"chromiumos/tast/local/shill"
	"chromiumos/tast/local/upstart"
	"chromiumos/tast/testing"
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
		}, {
			Name: "l2tp_ipsec_cert",
			Val: vpnServer{
				vpnType:  "L2TP/IPsec",
				authType: "cert",
			},
		}},
	})
}

const (
	l2tpIPsec                       = "L2TP/IPsec"
	pskAuth                         = "psk"
	certAuth                        = "cert"
	testDefaultProfileName          = "vpnTestProfile"
	testUserProfileName             = "vpnTestProfile2"
	version                         = 1
	checkPortalPropertyName         = "CheckPortalList"
	checkPortalPropertyValue        = "wifi,cellular"
	checkPortalPropertyDefaultValue = "ethernet,wifi,cellular"
)

func VPNConnect(ctx context.Context, s *testing.State) {
	// If the main body of the test times out, we still want to reserve a few
	// seconds to allow for our cleanup code to run.
	cleanupCtx := ctx
	ctx, cancel := ctxutil.Shorten(cleanupCtx, 3*time.Second)
	defer cancel()

	// We lose connectivity along the way here, and if that races with the
	// recover_duts network-recovery hooks, it may interrupt us.
	unlock, err := network.LockCheckNetworkHook(ctx)
	if err != nil {
		s.Fatal("Failed to lock the check network hook: ", err)
	}
	defer unlock()

	if err := removeDefaultProfile(ctx); err != nil {
		s.Fatal("Failed to remove the default profile: ", err)
	}

	manager, err := shill.NewManager(ctx)
	if err != nil {
		s.Fatal("Failed creating shill manager proxy: ", err)
	}
	// Remove test profiles in case they already exist.
	manager.RemoveProfile(ctx, testDefaultProfileName)
	manager.RemoveProfile(ctx, testUserProfileName)

	// Clean up custom services and test profiles on exit.
	defer func(ctx context.Context) {
		manager.PopProfile(ctx, testUserProfileName)
		manager.RemoveProfile(ctx, testUserProfileName)
		manager.PopProfile(ctx, testDefaultProfileName)
		manager.RemoveProfile(ctx, testDefaultProfileName)

		if err := removeDefaultProfile(ctx); err != nil {
			s.Error("Failed to remove the default profile: ", err)
		}
	}(cleanupCtx)

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

	// Disable portal check ethernet to recognize virtual ethernet as online.
	if err = manager.SetProperty(ctx, checkPortalPropertyName, checkPortalPropertyValue); err != nil {
		s.Log(ctx, "Failed to disable portal check on ethernet: ", err)
	}
	defer func(ctx context.Context) {
		if err = manager.SetProperty(ctx, checkPortalPropertyName, checkPortalPropertyDefaultValue); err != nil {
			s.Log(ctx, "Failed to re-enable portal check on ethernet: ", err)
		}
	}(cleanupCtx)

	// Wait for the Ethernet service to be online before running the test.
	// It is because the previous profile cleanup step restarts shill and
	// the Ethernet service the test depends on might not be ready yet.
	// Also, a change in the default physical Ethernet during the test,
	// could cause the L2TP VPN connection to fail (b:157677857).
	props := map[string]interface{}{
		shillconst.ServicePropertyType:  shillconst.TypeEthernet,
		shillconst.ServicePropertyState: shillconst.ServiceStateOnline,
	}

	// Wait for Connected Ethernet service. We wait 60 seconds for DHCP
	// negotiation since some DUTs will end up retrying DHCP discover/request, and
	// this can often take 15-30 seconds depending on the number of retries.
	if _, err := manager.WaitForServiceProperties(ctx, props, 60*time.Second); err != nil {
		s.Fatal("Service not found: ", err)
	}

	vpnType := s.Param().(vpnServer).vpnType
	authType := s.Param().(vpnServer).authType

	var certStore *netcertstore.Store
	if authType == certAuth {
		runner := hwsec.NewCmdRunner()
		certStore, err = netcertstore.CreateStore(ctx, runner)
		if err != nil {
			s.Fatal("Failed to create cert store: ", err)
		}

		defer func(ctx context.Context) {
			if err := certStore.Cleanup(ctx); err != nil {
				s.Log("Failed to clean up cert store: ", err)
			}
		}(cleanupCtx)
	}

	var serverAddress string
	if vpnType == l2tpIPsec {
		// Create new L2TP/IPsec.
		server := vpn.NewL2tpipSecVpnServer(ctx, authType)
		if serverAddress, err = server.StartServer(ctx); err != nil {
			s.Fatal("Failed to create a L2TP/IPsec server: ", err)
		}
		defer func(ctx context.Context) {
			if err := server.Exit(ctx); err != nil {
				s.Fatal("Failed to Stop a L2TP/IPsec server: ", err)
			}
		}(cleanupCtx)
	} else {
		s.Fatalf("Unexpected VPN type %s", vpnType)
	}

	if err := connectVPN(ctx, vpnType, authType, serverAddress, manager, certStore); err != nil {
		s.Fatal("Failed connecting to VPN server: ", err)
	}

	// Currently, the connected state of a VPN service doesn't mean the VPN service is ready for tunneling traffic: patchpanel needs to setup
	// some iptables rules (for routing and connection pinning) before that. If ping is started before iptables rules are ready, the traffic
	// generated by ping will be "pinned" to the previous default interface. So adds a small timeout here to mitigate this racing case.
	testing.Sleep(ctx, 500*time.Millisecond)

	pr := localping.NewLocalRunner()
	res, err := pr.Ping(ctx, vpn.Xl2tpdServerIPAddress, ping.Count(3), ping.User("chronos"))
	if err != nil {
		s.Fatal("Failed pinging the server IPv4: ", err)
	}
	if res.Received == 0 {
		s.Fatalf("Failed to ping %s: no response received", vpn.Xl2tpdServerIPAddress)
	}

	// IPv6 should be blackholed.
	if res, err := pr.Ping(ctx, "2001:db8::1", ping.Count(1), ping.User("chronos")); err == nil && res.Received != 0 {
		s.Fatal("IPv6 ping should fail: ", err)
	}

}

// removeDefaultProfile stops shill temporarily and remove the default profile, then restarts shill.
func removeDefaultProfile(ctx context.Context) (retErr error) {
	if err := upstart.StopJob(ctx, "shill"); err != nil {
		return errors.Wrap(err, "failed stopping shill")
	}

	defer func() {
		if err := upstart.RestartJob(ctx, "shill"); err != nil {
			if retErr != nil {
				retErr = errors.Wrap(err, "failed starting shill")
			} else {
				testing.ContextLog(ctx, "Failed starting shill: ", err)
			}

		}
	}()

	if err := os.Remove(shillconst.DefaultProfilePath); err != nil && !os.IsNotExist(err) {
		return errors.Wrap(err, "failed removing default profile")
	}

	return nil
}

// getVpnClientProperties returns VPN configuration properties.
func getVpnClientProperties(ctx context.Context, vpnType, authType, serverAddress string, certStore *netcertstore.Store) (map[string]interface{}, error) {
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
	if (vpnType == l2tpIPsec) && (authType == certAuth) {
		slotID := certStore.Slot()
		pin := certStore.Pin()
		clientCred := certificate.TestCert1().ClientCred
		certID, err := certStore.InstallCertKeyPair(ctx, clientCred.PrivateKey, clientCred.Cert)
		if err != nil {
			return nil, errors.Wrap(err, "failed to insert cert key pair into cert store")
		}
		params := map[string]interface{}{
			"L2TPIPsec.CACertPEM":      []string{certificate.TestCert1().CACred.Cert},
			"L2TPIPsec.ClientCertID":   certID,
			"L2TPIPsec.ClientCertSlot": fmt.Sprintf("%d", slotID),
			"L2TPIPsec.User":           vpn.ChapUser,
			"L2TPIPsec.Password":       vpn.ChapSecret,
			"L2TPIPsec.PIN":            pin,
			"Name":                     "test-vpn-l2tp-cert",
			"Provider.Host":            serverAddress,
			"Provider.Type":            "l2tpipsec",
			"Type":                     "vpn",
		}
		return params, nil
	}

	return nil, errors.Errorf("unexpected server type: got %s-%s, want L2TP/IPsec-psk", vpnType, authType)
}

// connectVPN connects the client to the VPN server.
func connectVPN(ctx context.Context, vpnType, authType, serverAddress string, manager *shill.Manager, certStore *netcertstore.Store) error {
	vpnProps, err := getVpnClientProperties(ctx, vpnType, authType, serverAddress, certStore)
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
