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

		timeoutCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
		defer cancel()
		// When shill finds this ethernet interface, it will reset its IP address and start a DHCP client.
		// We must configure the static IP address through shill.
		if err := configureStaticIP(timeoutCtx, clientInterfaceName, clientAddress, manager); err != nil {
			s.Fatal("Failed configuring the static IP: ", err)
		}

		expectSuccess := true
		if strings.Contains(vpnType, "incorrect") {
			expectSuccess = false
		}

		if err := connectVPN(ctx, vpnType, serverAddress, manager, expectSuccess); err != nil {
			s.Fatal("Failed connecting to VPN server: ", err)
		}

		testing.Sleep(ctx, 20*time.Second)

		if err := ping(ctx, vpn.Xl2tpdServerIPAddress, 10, 3, 60, "chronos"); err != nil {
			s.Fatal("Failed pinging the server IP: ", err)
		}
		/*
			ToDo:
				// IPv6 should be blackholed, so ping returns
				// "other error"
				if err := ping(ctx, "2001:db8::1", 10, 1, 60, ""); err != nil {
					s.Fatal("Failed IPv6 ping should have aborted: ", err)
				}
		*/
		if err := server.Exit(ctx); err != nil {
			s.Fatal("Failed to Stop a L2tpip server: ", err)
		}

	}
}

// configureStaticIP configures the Static IP parameters for the Ethernet interface |interface_name| and applies
// those parameters to the interface by forcing a re-connect.
func configureStaticIP(ctx context.Context, interfaceName string, address string, manager *shill.Manager) error {
	var SetPropertyErr error
	for {
		select {
		case <-ctx.Done():
			return errors.Wrapf(SetPropertyErr, "failed to set the property %v", shill.ServicePropertyStaticIPConfig)
		default:
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

			if err := service.SetProperty(ctx, shill.ServicePropertyStaticIPConfig, map[string]interface{}{shill.IPConfigPropertyAddress: address, "Prefixlen": networkPrefix}); err == nil {
				SetPropertyErr = err
				if err = service.Disconnect(ctx); err != nil {
					return errors.Wrapf(err, "failed to dis-connect the service %v", service)
				}

				if err = service.Connect(ctx); err != nil {
					return errors.Wrap(err, "failed to re-connect after configuring the static IP")
				}

				return nil
			}
		}
	}
}

// getVpnClientProperties returns VPN configuration properties.
func getVpnClientProperties(ctx context.Context, vpnType string, serverAddress string) (map[string]interface{}, error) {
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
func connectVPN(ctx context.Context, vpnType string, serverAddress string, manager *shill.Manager, expectSuccess bool) error {
	vpnProps, err := getVpnClientProperties(ctx, vpnType, serverAddress)
	if err != nil {
		return err
	}

	servicePath, err := manager.ConfigureServicePath(ctx, vpnProps)
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

	serviceProps, err := service.GetProperties(ctx)
	if err != nil {
		return errors.Wrapf(err, "failed getting peroperties for the service %v", service)
	}

	waitForService := func(ctx context.Context, serviceProps *shill.Properties) (bool, error) {
		for {
			select {
			case <-ctx.Done():
				return false, nil
			default:
				serviceProps, err := service.GetProperties(ctx)
				if err != nil {
					return false, errors.Wrap(err, "failed getting service properties")
				}

				state, err := serviceProps.GetString(shill.ServicePropertyState)
				if err != nil {
					return false, errors.Wrap(err, "failed getting profile entries")
				}

				if state == "ready" || state == "online" {
					return true, nil
				}
				testing.ContextLogf(ctx, "The state of the service = %s", state)
				testing.Sleep(ctx, 1*time.Second)
			}
		}
	}

	timeoutCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	successful, err := waitForService(timeoutCtx, serviceProps)
	if err != nil {
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
func ping(ctx context.Context, host string, deadline int, tries int, timeout int, user string) error {
	args := []string{host}

	command := "ping"
	if strings.Contains(host, ":") {
		command = "ping6"
	}

	//args = append(args, fmt.Sprintf("-w%d", deadline))
	args = append(args, fmt.Sprintf("-c%d", tries))

	if user != "" {
		temp := []string{user, "-c", command}
		command = "su"
		args = append(temp, args...)
	}

	cmd := testexec.CommandContext(ctx, command, args...)

	var lines bytes.Buffer
	var stderr bytes.Buffer
	var exitCode int
	cmd.Stdout = &lines
	cmd.Stderr = &stderr
	err := cmd.Run()
	if exitError, ok := err.(*exec.ExitError); ok {
		ws := exitError.Sys().(syscall.WaitStatus)
		exitCode = ws.ExitStatus()
	}

	if err != nil {

		return errors.Wrapf(err, "command result, stdout: %v stderr: %v exitCode: %v command: %v ",
			lines.String(), stderr.String(), fmt.Sprintf("%d", exitCode), args)

	}

	/* ToDo:
	    rc = result.exit_status
	    lines = result.stdout.splitlines()

	    // rc=0: host reachable
	    // rc=1: host unreachable
		// other: an error (do not abbreviate)
	    if rc in (0, 1){
	        // Report the two stats lines, as a single line.
	        // [-2]: packets transmitted, 1 received, 0% packet loss, time 0ms
	    	// [-1]: rtt min/avg/max/mdev = 0.497/0.497/0.497/0.000 ms
	        stats := lines[-2:]
	        while '' in stats:
	            stats.remove('')

	        if stats or len(lines) < 2{
				logging.debug('[rc=%s] %s', rc, )
				testing.ContextLogf(ctx,"[rc=%s] %s", rc, '; '.join(stats))
	        }else{
				testing.ContextLogf(ctx,"[rc=%s] Ping output:\n%s", rc, result.stdout)
			}
		}else{
	        output = result.stdout.rstrip()
	        if output{
				testing.ContextLogf(ctx,"Unusual ping result (rc=%s):\n%s", rc, output)
			}else{
				testing.ContextLogf(ctx,"Unusual ping result (rc=%s).", rc)
			}
		}
		return rc
	*/

	return nil

}
