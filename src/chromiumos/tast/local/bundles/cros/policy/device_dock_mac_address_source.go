// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package policy

import (
	"context"
	"io/ioutil"
	"net"
	"path/filepath"
	"strings"
	"time"

	"chromiumos/tast/common/fixture"
	"chromiumos/tast/common/policy"
	"chromiumos/tast/common/policy/fakedms"
	"chromiumos/tast/common/shillconst"
	"chromiumos/tast/ctxutil"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/network"
	"chromiumos/tast/local/policyutil"
	"chromiumos/tast/local/shill"
	"chromiumos/tast/testing"
	"chromiumos/tast/testing/hwdep"
)

// There is only 1 model with ethernet port: sarien. We explicitly list models without ethernet port.
// Otherwise we won't verify "DeviceNicMacAddress" policy value for new models with ethernet ports.
var noEthernetModels = []string{"arcada", "drallion", "drallion360"}

func init() {
	testing.AddTest(&testing.Test{
		Func: DeviceDockMacAddressSource,
		Desc: "Test setting the DeviceDockMacAddressSource policy by checking if the DUT changing MAC address",
		Contacts: []string{
			"lamzin@google.com", // Test author
			"chromeos-wilco@google.com",
		},
		SoftwareDeps: []string{"chrome", "wilco"},
		Fixture:      fixture.ChromeEnrolledLoggedIn,
		Params: []testing.Param{
			{
				Name:              "has_ethernet",
				Val:               true,
				ExtraAttr:         []string{"group:mainline", "informational"},
				ExtraHardwareDeps: hwdep.D(hwdep.SkipOnModel(noEthernetModels...)),
			},
			{
				Name:              "no_ethernet",
				Val:               false,
				ExtraAttr:         []string{"group:mainline", "informational"},
				ExtraHardwareDeps: hwdep.D(hwdep.Model(noEthernetModels...)),
			},
			{
				Name:              "has_ethernet_lab",
				Val:               true,
				ExtraAttr:         []string{"wilco_bve_dock"},
				ExtraHardwareDeps: hwdep.D(hwdep.SkipOnModel(noEthernetModels...)),
			},
			{
				Name:              "no_ethernet_lab",
				Val:               false,
				ExtraAttr:         []string{"wilco_bve_dock"},
				ExtraHardwareDeps: hwdep.D(hwdep.Model(noEthernetModels...)),
			},
		},
	})
}

func DeviceDockMacAddressSource(ctx context.Context, s *testing.State) {
	cr := s.FixtValue().(chrome.HasChrome).Chrome()
	fdms := s.FixtValue().(fakedms.HasFakeDMS).FakeDMS()

	cleanupCtx := ctx
	ctx, cancel := ctxutil.Shorten(ctx, 20*time.Second)
	defer cancel()

	defer func(ctx context.Context) {
		if err := policyutil.ResetChrome(ctx, fdms, cr); err != nil {
			s.Fatal("Failed to clean up: ", err)
		}
	}(cleanupCtx)

	designatedDockMAC, err := readMACFromVPD("dock_mac")
	if err != nil {
		s.Fatal("Failed to read dock_mac VPD field: ", err)
	}
	s.Log("designated dock MAC: ", designatedDockMAC)

	dockMAC, err := dockStationMAC(ctx)
	if err != nil {
		s.Fatal("Failed to get dock MAC: ", err)
	}
	s.Log("dock MAC: ", dockMAC)

	if dockMAC == designatedDockMAC {
		s.Fatal("Current dock MAC is equal to dock_mac VPD field")
	}

	type testCase struct {
		name    string                            // name is the subtest name.
		value   policy.DeviceDockMacAddressSource // value is the policy value.
		wantMAC string
	}

	testCases := []testCase{
		{
			name:    "DeviceDockMacAddress",
			value:   policy.DeviceDockMacAddressSource{Val: 1},
			wantMAC: designatedDockMAC,
		},
		{
			name:    "DockNicMacAddress",
			value:   policy.DeviceDockMacAddressSource{Val: 3},
			wantMAC: dockMAC,
		},
	}

	if hasEthernet := s.Param().(bool); hasEthernet {
		ethernetMAC, err := readMACFromVPD("ethernet_mac0")
		if err != nil {
			s.Fatal("Failed to read ethernet_mac0 VPD field: ", err)
		}
		s.Log("ethernet MAC: ", ethernetMAC)

		if dockMAC == ethernetMAC {
			s.Fatal("Current dock MAC is equal to ethernet_mac0 VPD field")
		}

		testCases = append(testCases,
			testCase{
				name:    "DeviceNicMacAddress",
				value:   policy.DeviceDockMacAddressSource{Val: 2},
				wantMAC: ethernetMAC,
			})
	} else {
		testCases = append(testCases,
			testCase{
				name:    "DeviceNicMacAddress",
				value:   policy.DeviceDockMacAddressSource{Val: 2},
				wantMAC: dockMAC,
			},
			// Need to repeat this test case to be sure that when policy is unset,
			// MAC address source will change to the default value.
			testCase{
				name:    "DeviceDockMacAddress_2",
				value:   policy.DeviceDockMacAddressSource{Val: 1},
				wantMAC: designatedDockMAC,
			})
	}

	testCases = append(testCases,
		testCase{
			name:    "unset",
			value:   policy.DeviceDockMacAddressSource{Stat: policy.StatusUnset},
			wantMAC: dockMAC,
		})

	// We lose connectivity along the way here, and if that races with the
	// recover_duts network-recovery hooks, it may interrupt us.
	unlock, err := network.LockCheckNetworkHook(ctx)
	if err != nil {
		s.Fatal("Failed to lock the check network hook: ", err)
	}
	defer unlock()

	for _, tc := range testCases {
		s.Run(ctx, tc.name, func(ctx context.Context, s *testing.State) {
			// Perform cleanup.
			if err := policyutil.ResetChrome(ctx, fdms, cr); err != nil {
				s.Fatal("Failed to clean up: ", err)
			}

			pb := fakedms.NewPolicyBlob()
			pb.AddPolicies([]policy.Policy{&tc.value})

			// After this point, the policy handler should be triggered.
			if err := policyutil.ServeBlobAndRefresh(ctx, fdms, cr, pb); err != nil {
				s.Fatal("Failed to serve and refresh: ", err)
			}

			if err := testing.Poll(ctx, func(ctx context.Context) error {
				newDockMAC, err := dockStationMAC(ctx)
				if err != nil {
					return testing.PollBreak(errors.Wrap(err, "failed to get dock MAC"))
				}
				if newDockMAC != tc.wantMAC {
					return errors.Errorf("unexpected dock MAC address = got %q, want %q", newDockMAC, tc.wantMAC)
				}
				return nil
			}, &testing.PollOptions{Timeout: 20 * time.Second}); err != nil {
				s.Fatal("Failed to verify dock station MAC changed as expected: ", err)
			}
		})
	}
}

func readMACFromVPD(vpd string) (string, error) {
	bytes, err := ioutil.ReadFile(filepath.Join("/sys/firmware/vpd/ro", vpd))
	if err != nil {
		return "", err
	}
	return strings.ToLower(string(bytes)), nil
}

// dockStationMAC returns MAC address of external USB Ethernet adapter.
func dockStationMAC(ctx context.Context) (string, error) {
	manager, err := shill.NewManager(ctx)
	if err != nil {
		return "", errors.Wrap(err, "failed to create shill manager proxy")
	}

	_, props, err := manager.DevicesByTechnology(ctx, shill.TechnologyEthernet)
	if err != nil {
		return "", errors.Wrap(err, "failed to get ethernet devices")
	}

	var dockIfaceName string

	for _, deviceProps := range props {
		if !deviceProps.Has(shillconst.DevicePropertyEthernetBusType) {
			continue
		}
		busType, err := deviceProps.GetString(shillconst.DevicePropertyEthernetBusType)
		if err != nil {
			return "", errors.Wrap(err, "failed to get device bus type")
		}

		iface, err := deviceProps.GetString(shillconst.DevicePropertyInterface)
		if err != nil {
			return "", errors.Wrap(err, "failed to get interface name")
		}

		if busType == "usb" {
			if dockIfaceName != "" {
				return "", errors.Errorf("more than 1 USB Ethernet adapters connected to the DUT (interfaces %q, %q)", dockIfaceName, iface)
			}
			dockIfaceName = iface
		}
	}

	if dockIfaceName == "" {
		return "", errors.New("not found USB Ethernet adapterd connected to the DUT")
	}

	ifi, err := net.InterfaceByName(dockIfaceName)
	if err != nil {
		return "", errors.Wrapf(err, "failed to get interface by %q name", dockIfaceName)
	}
	if ifi.HardwareAddr == nil {
		return "", errors.New("interface MAC address is nill")
	}
	return strings.ToLower(ifi.HardwareAddr.String()), nil
}
