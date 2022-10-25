// Copyright 2021 The ChromiumOS Authors
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
	"chromiumos/tast/common/pci"
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
// TODO(b/172208984): update this when Ethernet hardware dependency will be available.
var noEthernetModels = []string{"arcada", "drallion", "drallion360"}

func init() {
	testing.AddTest(&testing.Test{
		Func:         DeviceDockMacAddressSource,
		LacrosStatus: testing.LacrosVariantUnneeded,
		Desc:         "Test setting the DeviceDockMacAddressSource policy by checking if the DUT changing MAC address",
		Contacts: []string{
			"chromeos-oem-services@google.com", // Use team email for tickets.
			"bkersting@google.com",
			"lamzin@google.com",
		},
		SoftwareDeps: []string{"chrome", "wilco"},
		Fixture:      fixture.ChromeEnrolledLoggedIn,
		SearchFlags: []*testing.StringPair{
			pci.SearchFlag(&policy.DeviceDockMacAddressSource{}, pci.VerifiedFunctionalityOS),
		},
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
				ExtraAttr:         []string{"group:wilco_bve_dock"},
				ExtraHardwareDeps: hwdep.D(hwdep.SkipOnModel(noEthernetModels...)),
			},
			{
				Name:              "no_ethernet_lab",
				Val:               false,
				ExtraAttr:         []string{"group:wilco_bve_dock"},
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
	s.Log("Designated dock MAC: ", designatedDockMAC)

	dockMACs, err := dockStationMACs(ctx)
	if err != nil {
		s.Fatal("Failed to get dock MAC: ", err)
	}
	s.Log("Dock MACs: ", dockMACs)

	for _, dockMAC := range dockMACs {
		if dockMAC == designatedDockMAC {
			s.Fatalf("Current dock MAC (%s) is equal to dock_mac (%s) VPD field", dockMAC, designatedDockMAC)
		}
	}

	type testCase struct {
		name     string                            // name is the subtest name
		value    policy.DeviceDockMacAddressSource // value is the policy value
		wantMACs []string                          // expected MAC addresses
	}

	testCases := []testCase{
		{
			name:     "DeviceDockMacAddress",
			value:    policy.DeviceDockMacAddressSource{Val: 1},
			wantMACs: []string{designatedDockMAC},
		},
		{
			name:     "DockNicMacAddress",
			value:    policy.DeviceDockMacAddressSource{Val: 3},
			wantMACs: dockMACs,
		},
	}

	if hasEthernet := s.Param().(bool); hasEthernet {
		ethernetMAC, err := readMACFromVPD("ethernet_mac0")
		if err != nil {
			s.Fatal("Failed to read ethernet_mac0 VPD field: ", err)
		}
		s.Log("Ethernet MAC: ", ethernetMAC)

		for _, dockMAC := range dockMACs {
			if dockMAC == ethernetMAC {
				s.Fatalf("Current dock MAC (%s) is equal to ethernet_mac0 (%s) VPD field", dockMAC, ethernetMAC)
			}
		}

		testCases = append(testCases,
			testCase{
				name:     "DeviceNicMacAddress",
				value:    policy.DeviceDockMacAddressSource{Val: 2},
				wantMACs: []string{ethernetMAC},
			})
	} else {
		testCases = append(testCases,
			testCase{
				name:     "DeviceNicMacAddress",
				value:    policy.DeviceDockMacAddressSource{Val: 2},
				wantMACs: dockMACs,
			},
			// Need to repeat this test case to be sure that when policy is unset,
			// MAC address source will change to the default value.
			testCase{
				name:     "DeviceDockMacAddress_2",
				value:    policy.DeviceDockMacAddressSource{Val: 1},
				wantMACs: []string{designatedDockMAC},
			})
	}

	testCases = append(testCases,
		testCase{
			name:     "unset",
			value:    policy.DeviceDockMacAddressSource{Stat: policy.StatusUnset},
			wantMACs: dockMACs,
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

			pb := policy.NewBlob()
			pb.AddPolicies([]policy.Policy{&tc.value})

			if err := policyutil.ServeBlobAndRefresh(ctx, fdms, cr, pb); err != nil {
				s.Fatal("Failed to serve and refresh: ", err)
			}

			if err := testing.Poll(ctx, func(ctx context.Context) error {
				newDockMACs, err := dockStationMACs(ctx)
				if err != nil {
					return testing.PollBreak(errors.Wrap(err, "failed to get dock MACs"))
				}

				for _, wantMAC := range tc.wantMACs {
					match := false
					for _, newDockMAC := range newDockMACs {
						if newDockMAC == wantMAC {
							match = true
							break
						}
					}

					if !match {
						return errors.Errorf("unexpected dock MAC addresses = got %v, want contains %q", newDockMACs, wantMAC)
					}
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

// dockStationMACs returns MAC addresses of all external USB Ethernet adapter.
func dockStationMACs(ctx context.Context) ([]string, error) {
	manager, err := shill.NewManager(ctx)
	if err != nil {
		return nil, errors.Wrap(err, "failed to create shill manager proxy")
	}

	_, props, err := manager.DevicesByTechnology(ctx, shill.TechnologyEthernet)
	if err != nil {
		return nil, errors.Wrap(err, "failed to get ethernet devices")
	}

	var macs []string

	for _, deviceProps := range props {
		if !deviceProps.Has(shillconst.DevicePropertyEthernetBusType) {
			continue
		}
		busType, err := deviceProps.GetString(shillconst.DevicePropertyEthernetBusType)
		if err != nil {
			return nil, errors.Wrap(err, "failed to get device bus type")
		}

		iface, err := deviceProps.GetString(shillconst.DevicePropertyInterface)
		if err != nil {
			return nil, errors.Wrap(err, "failed to get interface name")
		}

		if busType != "usb" {
			continue
		}

		ifi, err := net.InterfaceByName(iface)
		if err != nil {
			return nil, errors.Wrapf(err, "failed to get interface by %q name", iface)
		}
		if ifi.HardwareAddr == nil {
			return nil, errors.New("interface MAC address is nil")
		}
		macs = append(macs, strings.ToLower(ifi.HardwareAddr.String()))
	}

	if len(macs) == 0 {
		return nil, errors.New("not found USB Ethernet adapter connected to the DUT")
	}

	return macs, nil
}
