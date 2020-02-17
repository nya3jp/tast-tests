// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package network

import (
	"context"
	"net"
	"os"
	"time"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/network"
	"chromiumos/tast/local/shill"
	"chromiumos/tast/local/upstart"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func: EthernetStaticIP,
		Desc: "Test whether static IP configurations behave as they should between profile changes",
		Contacts: []string{
			"matthewmwang@chromium.org",
			"cros-networking@google.com",
		},
		Attr: []string{"group:mainline", "informational"},
	})
}

func getIPForInterface(iface string) (string, error) {
	ifaceObj, err := net.InterfaceByName(iface)
	if err != nil {
		return "", err
	}
	addrs, err := ifaceObj.Addrs()
	if err != nil {
		return "", err
	}
	for _, a := range addrs {
		if ipnet, ok := a.(*net.IPNet); ok && ipnet.IP.To4() != nil {
			return ipnet.IP.String(), nil
		}
	}
	return "", errors.New("no IPv4 address found for " + iface)
}

func waitForIPOnInterface(ctx context.Context, iface string, expected string, timeout time.Duration) error {
	if err := testing.Poll(ctx, func(ctx context.Context) error {
		if ip, err := getIPForInterface(iface); err != nil {
			return err
		} else if ip != expected {
			return errors.Errorf("wrong IP address for Ethernet: got %v, want %v", ip, expected)
		}
		return nil
	}, &testing.PollOptions{Timeout: timeout}); err != nil {
		return err
	}
	return nil
}

func EthernetStaticIP(ctx context.Context, s *testing.State) {
	const (
		defaultProfilePath     = "/var/cache/shill/default.profile"
		testDefaultProfileName = "ethTestProfile"
		testUserProfileName    = "ethTestProfile2"
		testIP1                = "10.9.8.2"
		testIP2                = "10.9.8.3"
		// TODO(crbug/1027742): Shorten the timeout after the root cause of long IP waiting time is found.
		timeoutWaitForIP = 30 * time.Second
	)

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
		// TODO(oka): It's possible that the default profile has been
		// removed by the previous test, and this test has started
		// before the default profile is created by the previous test's
		// (re)starting of Shill. It's a confusing race condition, so
		// fix it by making sure that the default profile exsits here.
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

	// Find the Ethernet service and set the static IP.
	s.Log("Setting static IP")
	servicePath, err := manager.WaitForServiceProperties(ctx, map[string]interface{}{shill.ServicePropertyType: "ethernet"}, 5*time.Second)
	if err != nil {
		s.Fatal("Unable to find service: ", err)
	}
	service, err := shill.NewService(ctx, servicePath)
	if err != nil {
		s.Fatal("Failed creating shill service proxy: ", err)
	}
	if err = service.SetProperty(ctx, shill.ServicePropertyStaticIPConfig, map[string]interface{}{shill.IPConfigPropertyAddress: testIP1}); err != nil {
		s.Fatal("Failed to set property: ", err)
	}

	// Find the corresponding interface name for the Ethernet service
	s.Log("Finding the interface name for the Ethernet service")
	device, err := service.GetDevice(ctx)
	if err != nil {
		s.Fatal("Failed to get device: ", err)
	}
	deviceProp, err := device.GetProperties(ctx)
	if err != nil {
		s.Fatal("Failed to get properties of device: ", err)
	}
	iface, err := deviceProp.GetString(shill.DevicePropertyInterface)
	if err != nil {
		s.Fatal("Failed to get interface for device: ", err)
	}

	// Test that static IP has been set.
	s.Log("Finding service with set static IP")
	if err = waitForIPOnInterface(ctx, iface, testIP1, timeoutWaitForIP); err != nil {
		s.Fatal("Unable to find expected IP for Ethernet: ", err)
	}

	// Test that after profile push, property is still there.
	s.Log("Pushing profile and checking that static IP has not changed")
	userProfileObjectPath, err := manager.CreateProfile(ctx, testUserProfileName)
	if err != nil {
		s.Fatal("Failed to create profile: ", err)
	}
	if _, err = manager.PushProfile(ctx, testUserProfileName); err != nil {
		s.Fatal("Failed to push profile: ", err)
	}
	defaultProfileProps := map[string]interface{}{
		shill.ServicePropertyType:           "ethernet",
		shill.ServicePropertyStaticIPConfig: map[string]interface{}{shill.IPConfigPropertyAddress: testIP1},
	}
	if _, err := manager.WaitForServiceProperties(ctx, defaultProfileProps, 5*time.Second); err != nil {
		s.Fatal("Unable to find service: ", err)
	}
	if err = waitForIPOnInterface(ctx, iface, testIP1, timeoutWaitForIP); err != nil {
		s.Fatal("Unable to find expected IP for Ethernet: ", err)
	}

	// Configure service for user profile with different static IP.
	s.Log("Configure different static IP for the new profile")
	userProfileProps := map[string]interface{}{
		shill.ServicePropertyType:           "ethernet",
		shill.ServicePropertyStaticIPConfig: map[string]interface{}{shill.IPConfigPropertyAddress: testIP2},
	}
	if _, err = manager.ConfigureServiceForProfile(ctx, userProfileObjectPath, userProfileProps); err != nil {
		s.Fatal("Unable to configure service: ", err)
	}

	// Test that new static IP is there.
	s.Log("Finding service with new static IP")
	if err = waitForIPOnInterface(ctx, iface, testIP2, timeoutWaitForIP); err != nil {
		s.Fatal("Unable to find expected IP for Ethernet: ", err)
	}

	// Test that after profile pop, first static IP is there.
	s.Log("Popping user profile and checking that static IP has reverted")
	if err = manager.PopProfile(ctx, testUserProfileName); err != nil {
		s.Fatal("Unable to pop profile: ", err)
	}
	if err = waitForIPOnInterface(ctx, iface, testIP1, timeoutWaitForIP); err != nil {
		s.Fatal("Unable to find expected IP for Ethernet: ", err)
	}

	// Test that after push, second static IP is there again.
	s.Log("Re-pushing the same user profile and checking that static IP changes")
	if _, err = manager.PushProfile(ctx, testUserProfileName); err != nil {
		s.Fatal("Failed to push profile: ", err)
	}
	if err = waitForIPOnInterface(ctx, iface, testIP2, timeoutWaitForIP); err != nil {
		s.Fatal("Unable to find expected IP for Ethernet: ", err)
	}
}
