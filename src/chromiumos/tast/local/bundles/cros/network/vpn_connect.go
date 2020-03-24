// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package network

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"syscall"
	"time"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/bundles/cros/network/veth"
	"chromiumos/tast/local/bundles/cros/network/vpn"
	"chromiumos/tast/local/network"
	"chromiumos/tast/local/shill"
	"chromiumos/tast/local/testexec"
	"chromiumos/tast/local/upstart"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:     VPNConnect,
		Desc:     "Ensure that we can complete OpenVPN authentication with a server",
		Contacts: []string{"arowa@google.com", "cros-networking@google.com"},
		Attr:     []string{"group:mainline", "informational"},
	})
}

const (
	defaultProfilePath     = "/var/cache/shill/default.profile"
	testDefaultProfileName = "vpnTestProfile"
	testUserProfileName    = "vpnTestProfile2"
	clientInterfaceName    = "pseudoethernet0"
	serverInterfaceName    = "serverethernet0"
	version                = 1
	serverAddress          = "10.9.8.1"
	clientAddress          = "10.9.8.2"
	networkPrefix          = 24
)

var (
	vpnTypes = []string{"l2tpipsec-psk"}
)

func VPNConnect(ctx context.Context, s *testing.State) {
	for _, vpnType := range vpnTypes {
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

			if err := os.Remove(defaultProfilePath); err != nil && !os.IsNotExist(err) {
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
			os.Remove(defaultProfilePath)
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

		// Create new L2tpipsec.
		server := vpn.NewL2tpipSecVpnServer(ctx, "psk", serverInterfaceName, serverAddress, networkPrefix, strings.Contains(vpnType, "xauth"), strings.Contains(vpnType, "evil"))
		if err := server.StartServer(ctx); err != nil {
			s.Fatal("Failed to create a L2tpip server: ", err)
		}

		defer func() {
			if err := server.Exit(ctx); err != nil {
				s.Fatal("Failed to Stop a L2tpip server: ", err)
			}
		}()

		// When shill finds this ethernet interface, it will reset its IP address and start a DHCP client.
		// We must configure the static IP address through shill.
		if err := configureStaticIP(ctx, clientInterfaceName, clientAddress, manager); err != nil {
			s.Fatal("Failed configuring the static IP: ", err)
		}

		expectSuccess := true
		if strings.Contains(vpnType, "incorrect") {
			expectSuccess = false
		}

		if err := connectVPN(ctx, vpnType, serverAddress, manager, expectSuccess); err != nil {
			s.Fatal("Failed connecting to VPN server: ", err)
		}

		if rslt := ping(ctx, vpn.Xl2tpdServerIPAddress, 3, "chronos"); rslt != 0 {
			s.Fatal("Failed pinging the server IP")
		}

		// IPv6 should be blackholed, so ping returns
		// "other error"
		if rslt := ping(ctx, "2001:db8::1", 1, "chronos"); rslt != 2 {
			s.Fatal("Failed IPv6 ping should have aborted: ", rslt)
		}
	}
}

// configureStaticIP configures the Static IP parameters for the Ethernet interface |interface_name| and applies
// those parameters to the interface by forcing a re-connect.
func configureStaticIP(ctx context.Context, interfaceName, address string, manager *shill.Manager) error {
	// Wait for static IP to be configured.
	testing.ContextLog(ctx, "Wait for static IP to be configured")
	if err := testing.Poll(ctx, func(ctx context.Context) error {
		device, err := manager.WaitForDeviceByName(ctx, interfaceName, 5*time.Second)
		if err != nil {
			return errors.Wrapf(err, "failed to find the device with interface name %s", interfaceName)
		}

		deviceProp, err := device.GetProperties(ctx)
		if err != nil {
			return errors.Wrapf(err, "failed to get properties of device %v", device)
		}

		servicePath, err := deviceProp.GetObjectPath(shill.DevicePropertySelectedService)
		if err != nil {
			return errors.Wrapf(err, "failed to get the DBus object path for the property %s", shill.DevicePropertySelectedService)
		}

		service, err := shill.NewService(ctx, servicePath)
		if err != nil {
			return errors.Wrap(err, "failed creating shill service proxy")
		}

		if err := service.SetProperty(ctx, shill.ServicePropertyStaticIPConfig, map[string]interface{}{shill.IPConfigPropertyAddress: address, "Prefixlen": networkPrefix}); err != nil {
			return errors.Wrap(err, "failed to configure the static IP address")
		}

		if err = service.Disconnect(ctx); err != nil {
			return errors.Wrapf(err, "failed to dis-connect the service %v", service)
		}

		if err = service.Connect(ctx); err != nil {
			return errors.Wrap(err, "failed to re-connect after configuring the static IP")
		}

		return nil
	}, &testing.PollOptions{Timeout: 100 * time.Second}); err != nil {
		return errors.Wrap(err, "failed to wait for static IP to be configured")
	}

	return nil
}

// getVpnClientProperties returns VPN configuration properties.
func getVpnClientProperties(ctx context.Context, vpnType, serverAddress string) (map[string]interface{}, error) {
	if strings.HasPrefix(vpnType, "l2tpipsec-psk") {
		params := map[string]interface{}{
			"L2TPIPsec.Password": vpn.ChapSecret,
			"L2TPIPsec.PSK":      vpn.IPsecPresharedKey,
			"L2TPIPsec.User":     vpn.ChapUser,
			"Name":               "test-vpn-l2tp-psk",
			"Provider.Host":      serverAddress,
			"Provider.Type":      "l2tpipsec",
			"Type":               "vpn",
		}

		if strings.Contains(vpnType, "xauth") {
			if strings.Contains(vpnType, "incorrect_user") {
				params["L2TPIPsec.XauthUser"] = "wrong_user"
				params["L2TPIPsec.XauthPassword"] = "wrong_password"
			} else if !strings.Contains(vpnType, "incorrect_missing_user") {
				params["L2TPIPsec.XauthUser"] = vpn.XauthUser
				params["L2TPIPsec.XauthPassword"] = vpn.XauthPassword
			}
		}

		return params, nil
	}

	return nil, errors.Errorf("failed unexpected server type: got %s want %v", vpnType, []string{"l2tpipsec-psk", "l2tpipsec-cert", "openvpn"})
}

// connectVPN connects the client to the VPN server.
func connectVPN(ctx context.Context, vpnType, serverAddress string, manager *shill.Manager, expectSuccess bool) error {
	vpnProps, err := getVpnClientProperties(ctx, vpnType, serverAddress)
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

	if err = service.Connect(ctx); err != nil {
		return errors.Wrapf(err, "failed to connect the service %v", service)
	}

	successful := false
	// Wait for server to be online/ready.
	testing.ContextLog(ctx, "Wait for server to be online/ready")
	if err := testing.Poll(ctx, func(ctx context.Context) error {
		serviceProps, err := service.GetProperties(ctx)
		if err != nil {
			return errors.Wrap(err, "failed getting service properties")
		}

		state, err := serviceProps.GetString(shill.ServicePropertyState)
		if err != nil {
			return errors.Wrap(err, "failed getting profile entries")
		}

		if state == "configuration" {
			return errors.New("failed the server state is still in configuration")
		}

		if state == "failure" {
			return nil
		}

		if state != "ready" && state != "online" {
			return errors.Errorf("failed unexpected server state: got %s wan ready/online/failure", state)
		}

		successful = true
		return nil
	}, &testing.PollOptions{Timeout: 10 * time.Second}); err != nil {
		return err
	}

	if !successful && expectSuccess {
		return errors.New("VPN connection failed")
	}

	if successful && !expectSuccess {
		return errors.New("VPN connection succeeded when it should have failed")
	}

	return nil
}

// ping sends a ping to the host.
func ping(ctx context.Context, host string, tries int, user string) int {
	args := []string{host}

	command := "ping"
	if strings.Contains(host, ":") {
		command = "ping6"
	}

	args = append(args, fmt.Sprintf("-c%d", tries))

	if user != "" {
		userCmd := command + " " + host + " " + fmt.Sprintf("-c%d", tries)
		args = []string{user, "-c", userCmd}
		command = "su"
	}

	cmd := testexec.CommandContext(ctx, command, args...)

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	var exitCode int
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err := cmd.Run()
	if exitError, ok := err.(*exec.ExitError); ok {
		ws := exitError.Sys().(syscall.WaitStatus)
		exitCode = ws.ExitStatus()
	}

	// testing.ContextLogf(ctx, "command result, stdout: %v stderr: %v exitCode: %v command: %v ", stdout.String(), stderr.String(), fmt.Sprintf("%d", exitCode), cmd.String())

	lines := strings.Split(stdout.String(), "\n")

	// exitCode=0: host reachable
	// exitCode=1: host unreachable
	// other: an error (do not abbreviate)
	if exitCode == 0 || exitCode == 1 {
		// Report the two stats lines, as a single line.
		// [-2]: packets transmitted, 1 received, 0% packet loss, time 0ms
		// [-1]: rtt min/avg/max/mdev = 0.497/0.497/0.497/0.000 ms
		var stats []string
		if len(lines) > 2 {
			stats = lines[len(lines)-2:]
		} else {
			stats = lines
		}

		if (len(stats) > 0) || (len(lines) < 2) {
			testing.ContextLogf(ctx, "[exitCode=%d] %s", exitCode, strings.Join(stats, "; "))
		} else {
			testing.ContextLogf(ctx, "[exitCode=%d] Ping output: %s", exitCode, stdout.String())
		}
	} else {
		output := strings.TrimSpace(stdout.String())
		if len(output) > 0 {
			testing.ContextLogf(ctx, "Unusual ping result (exitCode=%d): %s", exitCode, output)
		} else {
			testing.ContextLogf(ctx, "Unusual ping result (exitCode=%d)", exitCode)
		}
	}

	return exitCode
}
