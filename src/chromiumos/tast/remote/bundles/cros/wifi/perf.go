// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package wifi

import (
	"context"
	"fmt"
	"strings"
	"time"

	"chromiumos/tast/common/perf"
	"chromiumos/tast/common/wifi/security"
	"chromiumos/tast/common/wifi/security/wpa"
	"chromiumos/tast/ctxutil"
	"chromiumos/tast/errors"
	remoteiw "chromiumos/tast/remote/network/iw"
	"chromiumos/tast/remote/network/netperf"
	"chromiumos/tast/remote/wificell"
	ap "chromiumos/tast/remote/wificell/hostapd"
	"chromiumos/tast/ssh"
	"chromiumos/tast/testing"
)

type perfTestcase struct {
	perfConfigs []netperf.Config
	apOpts      []ap.Option
	secConfFac  security.ConfigFactory
}

var defaultNetperfConfigs = []netperf.Config{
	netperf.Config{
		TestTime: 10 * time.Second,
		TestType: netperf.TestTypeTCPStream},
	netperf.Config{
		TestTime: 10 * time.Second,
		TestType: netperf.TestTypeTCPMaerts},
	netperf.Config{
		TestTime: 10 * time.Second,
		TestType: netperf.TestTypeUDPStream},
	netperf.Config{
		TestTime: 10 * time.Second,
		TestType: netperf.TestTypeUDPMaerts},
}

func init() {
	testing.AddTest(&testing.Test{
		Func:        Perf,
		Desc:        "Measure WiFi throughput in various modes",
		Contacts:    []string{"wgd@google.com", "chromeos-platform-connectivity@google.com"},
		Attr:        []string{"group:wificell", "wificell_unstable"},
		ServiceDeps: []string{wificell.TFServiceName},
		// CR: Cannot use TestFixturePreWithCapture because the pcap files rapidly exhaust all space in /tmp,
		// and when there is no second router to dedicate to pcap this causes the actual test setup to fail
		// after getting through one or two test cases.
		Pre:     wificell.TestFixturePre(),
		Vars:    []string{"router", "pcap", "governor"},
		Timeout: 120 * time.Minute,
		Params: []testing.Param{
			{
				Name: "11g",
				Val: []perfTestcase{{
					perfConfigs: defaultNetperfConfigs,
					apOpts:      []ap.Option{ap.Mode(ap.Mode80211g), ap.Channel(6)},
				}},
			}, {
				Name: "11g_aes",
				Val: []perfTestcase{{
					perfConfigs: defaultNetperfConfigs,
					apOpts:      []ap.Option{ap.Mode(ap.Mode80211g), ap.Channel(6)},
					secConfFac:  wpa.NewConfigFactory("chromeos", wpa.Mode(wpa.ModePureWPA), wpa.Ciphers(wpa.CipherCCMP)),
				}},
			}, {
				Name: "11g_tkip",
				Val: []perfTestcase{{
					perfConfigs: defaultNetperfConfigs,
					apOpts:      []ap.Option{ap.Mode(ap.Mode80211g), ap.Channel(6)},
					secConfFac:  wpa.NewConfigFactory("chromeos", wpa.Mode(wpa.ModePureWPA), wpa.Ciphers(wpa.CipherTKIP)),
				}},
			}, {
				Name: "ht20",
				Val: []perfTestcase{
					{
						perfConfigs: defaultNetperfConfigs,
						apOpts:      []ap.Option{ap.Channel(1), ap.Mode(ap.Mode80211nPure), ap.HTCaps(ap.HTCapHT20)},
					}, {
						perfConfigs: defaultNetperfConfigs,
						apOpts:      []ap.Option{ap.Channel(157), ap.Mode(ap.Mode80211nPure), ap.HTCaps(ap.HTCapHT20)},
					},
				},
			}, {
				Name: "ht20_aes",
				Val: []perfTestcase{
					{
						perfConfigs: defaultNetperfConfigs,
						apOpts:      []ap.Option{ap.Channel(1), ap.Mode(ap.Mode80211nPure), ap.HTCaps(ap.HTCapHT20)},
						secConfFac:  wpa.NewConfigFactory("chromeos", wpa.Mode(wpa.ModePureWPA), wpa.Ciphers(wpa.CipherCCMP)),
					}, {
						perfConfigs: defaultNetperfConfigs,
						apOpts:      []ap.Option{ap.Channel(157), ap.Mode(ap.Mode80211nPure), ap.HTCaps(ap.HTCapHT20)},
						secConfFac:  wpa.NewConfigFactory("chromeos", wpa.Mode(wpa.ModePureWPA), wpa.Ciphers(wpa.CipherCCMP)),
					},
				},
			}, {
				Name: "ht40",
				Val: []perfTestcase{
					{
						perfConfigs: defaultNetperfConfigs,
						apOpts:      []ap.Option{ap.Channel(1), ap.Mode(ap.Mode80211nPure), ap.HTCaps(ap.HTCapHT40)},
					}, {
						perfConfigs: defaultNetperfConfigs,
						apOpts:      []ap.Option{ap.Channel(157), ap.Mode(ap.Mode80211nPure), ap.HTCaps(ap.HTCapHT40)},
					},
				},
			}, {
				Name: "ht40_aes",
				Val: []perfTestcase{
					{
						perfConfigs: defaultNetperfConfigs,
						apOpts:      []ap.Option{ap.Channel(1), ap.Mode(ap.Mode80211nPure), ap.HTCaps(ap.HTCapHT40)},
						secConfFac:  wpa.NewConfigFactory("chromeos", wpa.Mode(wpa.ModePureWPA), wpa.Ciphers(wpa.CipherCCMP)),
					}, {
						perfConfigs: defaultNetperfConfigs,
						apOpts:      []ap.Option{ap.Channel(157), ap.Mode(ap.Mode80211nPure), ap.HTCaps(ap.HTCapHT40)},
						secConfFac:  wpa.NewConfigFactory("chromeos", wpa.Mode(wpa.ModePureWPA), wpa.Ciphers(wpa.CipherCCMP)),
					},
				},
			}, {
				Name: "vht80",
				Val: []perfTestcase{
					{
						perfConfigs: defaultNetperfConfigs,
						apOpts: []ap.Option{
							ap.Mode(ap.Mode80211acMixed), ap.Channel(44),
							ap.HTCaps(ap.HTCapHT40Plus), ap.VHTCaps(ap.VHTCapSGI80),
							ap.VHTCenterChannel(42), ap.VHTChWidth(ap.VHTChWidth80),
						},
					}, {
						perfConfigs: defaultNetperfConfigs,
						apOpts: []ap.Option{
							ap.Mode(ap.Mode80211acMixed), ap.Channel(157),
							ap.HTCaps(ap.HTCapHT40Plus), ap.VHTCaps(ap.VHTCapSGI80),
							ap.VHTCenterChannel(45), ap.VHTChWidth(ap.VHTChWidth80),
						},
					},
				},
			},
		},
	})
}

func Perf(ctx context.Context, s *testing.State) {
	tf := s.PreValue().(*wificell.TestFixture)
	defer func(ctx context.Context) {
		if err := tf.CollectLogs(ctx); err != nil {
			s.Log("Error collecting logs, err: ", err)
		}
	}(ctx)
	ctx, cancel := tf.ReserveForCollectLogs(ctx)
	defer cancel()

	pv := perf.NewValues()
	defer func() {
		if err := pv.Save(s.OutDir()); err != nil {
			s.Log("Failed to save perf data, err: ", err)
		}
	}()

	iwr := remoteiw.NewRemoteRunner(s.DUT().Conn())
	clientIface, err := tf.ClientInterface(ctx)
	if err != nil {
		s.Fatal("Failed to get the client interface: ", err)
	}

	// CR: Can this logic be changed to something more comprehensible? Is the
	// governor stuff actually used by anybody?
	//
	// The empty string means "don't set the CPU governor, but try to infer
	// the name from the active governors on DUT and Router, and if they don't
	// match then call it 'default'".
	//
	// In practice the DUT is usually using the 'powersave' governor and the AP
	// the 'performance' governor, so we end up calling it 'default' any time
	// the governors haven't been manually set up beforehand, and an actual
	// named governor policy is only used when explicitly requested at the
	// command line when the test was invoked.
	//
	// This seems unnecessarily complicated in the common case, but it's what
	// the previous Autotest incarnation of the test does (using 'None' instead
	// of the empty string).
	governors := []string{""}
	if governorFlag, ok := s.Var("governor"); ok && governorFlag != "" {
		governors = append(governors, governorFlag)
	}
	s.Logf("Testing with CPU governors: %q", governors)

	testOnce := func(ctx context.Context, s *testing.State, tc perfTestcase) {
		s.Log("Configuring AP")
		ap, err := tf.ConfigureAP(ctx, tc.apOpts, tc.secConfFac)
		if err != nil {
			s.Fatal("Failed to configure AP: ", err)
		}
		defer func(ctx context.Context) {
			if err := tf.DeconfigAP(ctx, ap); err != nil {
				s.Error("Failed to deconfig AP: ", err)
			}
		}(ctx)
		ctx, cancel = tf.ReserveForDeconfigAP(ctx, ap)
		defer cancel()
		apDesc := ap.Config().PerfDesc()

		s.Log("Connecting to AP")
		cleanupCtx := ctx
		ctx, cancel = tf.ReserveForDisconnect(ctx)
		defer cancel()
		if _, err := tf.ConnectWifiAP(ctx, ap); err != nil {
			s.Fatal("Failed to connect to AP: ", err)
		}
		defer func(ctx context.Context) {
			if err := tf.CleanDisconnectWifi(ctx); err != nil {
				s.Error("Failed to disconnect WiFi: ", err)
			}
		}(cleanupCtx)

		// CR: Is this necessary?
		s.Log("Verifying connection to AP")
		if err := tf.VerifyConnection(ctx, ap); err != nil {
			s.Fatal("Failed to verify connection: ", err)
		}

		s.Log("Starting Netperf Session")
		addrs, err := tf.ClientIPv4Addrs(ctx)
		if err != nil || len(addrs) == 0 {
			s.Fatal("Failed to get client IP address: ", err)
		}
		clientIP := addrs[0]
		netperfSession := netperf.NewSession(s.DUT().Conn(), clientIP.String(), tf.Router().Conn(), ap.ServerIP().String())
		defer func(ctx context.Context) {
			netperfSession.Close(ctx)
		}(ctx)

		s.Log("Warming up stations")
		if err := netperfSession.WarmupStations(ctx); err != nil {
			s.Fatal("Failed to warm up stations: ", err)
		}

		for _, powerSave := range []bool{true, false} {
			for _, governor := range governors {
				s.Logf("Setting powersave mode to %t", powerSave)
				cleanupCtx = ctx
				ctx, cancel = ctxutil.Shorten(ctx, time.Second)
				defer cancel()
				oldPowerSave, err := iwr.PowersaveMode(ctx, clientIface)
				if err != nil {
					s.Fatal("Failed to get the powersave mode: ", err)
				}
				defer func(ctx context.Context) {
					s.Logf("Restoring power save mode to %t", oldPowerSave)
					if err := iwr.SetPowersaveMode(ctx, clientIface, oldPowerSave); err != nil {
						s.Errorf("Failed to restore powersave mode to %t: %v", oldPowerSave, err)
					}
				}(cleanupCtx)
				if err := iwr.SetPowersaveMode(ctx, clientIface, powerSave); err != nil {
					s.Fatalf("Failed to set powersave mode to %t: %v", powerSave, err)
				}

				// CR: The Autotest code explicitly checks and modifies the CPU
				// governor on both the DUT *and* the Router when a non-default
				// policy is requested. Is there a scenario where that even
				// makes sense? My gut feeling is that there are legitimate
				// reasons we might want to test the impact of different CPU
				// governors on the client, but wanting any AP governor besides
				// 'performance' seems like an extremely niche situation?
				//
				// I've implemented a simpler bit of logic as a stub here, which
				// merely sets/restores the DUT scaling governor if one is
				// requested.
				//
				// TODO: Figure out why only CPUs 0/2/4/6 can be modified and
				// CPU 1/3/5/7 instead error out. This happens even if I just
				// run 'echo performance > /sys/devices/system/cpu/cpu1/cpufreq/scaling_governor'
				// on the DUT directly so I don't think it's specific to the
				// test code here. Did this logic break at some point in the past
				// and we never noticed because nobody ever uses non-default
				// governors with this test anyway?
				governorName := "default"
				if governor != "" {
					s.Logf("Setting CPU scaling governor to %s", governor)
					governorName = governor
					cleanupCtx := ctx
					ctx, cancel := ctxutil.Shorten(ctx, time.Second)
					defer cancel()
					dutGovernors, err := getScalingGovernors(ctx, s.DUT().Conn())
					if err != nil {
						s.Fatal("Failed to get scaling governor values: ", err)
					}
					if err := setScalingGovernors(ctx, s.DUT().Conn(), governor); err != nil {
						s.Fatal("Failed to set scaling governor values: ", err)
					}
					defer func(ctx context.Context) {
						s.Log("Restoring scaling governors")
						if err := restoreScalingGovernors(ctx, s.DUT().Conn(), dutGovernors); err != nil {
							s.Error("Failed to restore scaling governors: ", err)
						}
					}(cleanupCtx)
				}

				for _, perfConfig := range tc.perfConfigs {
					runHistory, err := netperfSession.Run(ctx, perfConfig)
					if err != nil {
						s.Fatalf("Failed to take measurement for %s: %v", perfConfig.HumanReadableTag(), err)
					}
					aggregate, err := netperf.AggregateSamples(ctx, runHistory)
					if err != nil {
						s.Fatal("Failed to aggregate performance data: ", err)
					}
					psDesc := "PSoff"
					if powerSave {
						psDesc = "PSon"
					}
					govDesc := "governor-" + governorName
					modeDesc := fmt.Sprintf("%s_%s_%s.%s", apDesc, psDesc, govDesc, perfConfig.ShortTag())
					throughput, ok := aggregate.Measurements[netperf.CategoryThroughput]
					if !ok {
						s.Fatal("Missing throughput metric")
					}
					throughputDev, ok := aggregate.Measurements[netperf.CategoryThroughputDev]
					if !ok {
						s.Error("Missing throughput deviation metric")
					}
					// Ignoring 'ok' because a missing 'errors' metric means there were no errors
					errorCount, _ := aggregate.Measurements[netperf.CategoryErrors]
					s.Logf("Measurement %s = %.2f ± %.2f Mbps", modeDesc, throughput, throughputDev)
					pv.Set(perf.Metric{
						Name:      modeDesc,
						Variant:   "throughput",
						Unit:      "Mbps",
						Direction: perf.BiggerIsBetter,
					}, throughput)
					pv.Set(perf.Metric{
						Name:      modeDesc,
						Variant:   "throughput_deviation",
						Unit:      "Mbps",
						Direction: perf.SmallerIsBetter,
					}, throughputDev)
					pv.Set(perf.Metric{
						Name:      modeDesc,
						Variant:   "errors",
						Unit:      "errors",
						Direction: perf.SmallerIsBetter,
					}, errorCount)
				}
			}
		}
	}

	testcases := s.Param().([]perfTestcase)
	for i, tc := range testcases {
		subtest := func(ctx context.Context, s *testing.State) {
			testOnce(ctx, s, tc)
		}
		subtestName := fmt.Sprintf("Testcase #%d", i)
		if !s.Run(ctx, subtestName, subtest) {
			break
		}
	}
}

// TODO: Migrate all this CPU frequency governor logic into a library?

func cpufreqPaths(ctx context.Context, host *ssh.Conn, filename string) ([]string, error) {
	glob := "/sys/devices/system/cpu/cpu*/cpufreq/" + filename
	cmd := host.Command("sh", "-c", fmt.Sprintf("echo %s", glob))
	out, err := cmd.Output(ctx)
	if err != nil {
		return nil, errors.Wrap(err, "unable to list cpufreq paths")
	}
	paths := strings.Split(strings.TrimSpace(string(out)), " ")
	// If the glob result equals itself then we probably didn't match any real paths
	if len(paths) == 1 && paths[0] == glob {
		return nil, errors.Errorf("no paths matching %q", glob)
	}
	return paths, nil
}

func getScalingGovernors(ctx context.Context, host *ssh.Conn) (map[string]string, error) {
	paths, err := cpufreqPaths(ctx, host, "scaling_governor")
	if err != nil {
		return nil, errors.Wrap(err, "unable to get scaling governor paths")
	}
	states := make(map[string]string)
	for _, path := range paths {
		cmd := host.Command("head", "-n", "1", path)
		out, err := cmd.Output(ctx)
		if err != nil {
			return nil, errors.Wrap(err, "unable to read scaling governor")
		}
		states[path] = string(out)
	}
	return states, nil
}

func setScalingGovernors(ctx context.Context, host *ssh.Conn, governor string) error {
	paths, err := cpufreqPaths(ctx, host, "scaling_governor")
	if err != nil {
		return errors.Wrap(err, "unable to get scaling governor paths")
	}
	for _, path := range paths {
		cmd := host.Command("sh", "-c", fmt.Sprintf("echo %q > %s", governor, path))
		if err := cmd.Run(ctx); err != nil {
			// CR: The Autotest version of this code ignores failures to support
			// platforms where CPUs can be dynamically enabled/disabled. Is that
			// applicable to ChromeOS devices or is that a bit of unnecessary
			// platform support?
			testing.ContextLogf(ctx, "unable to write scaling governor %q: %v", path, err)
		}
	}
	return nil
}

func restoreScalingGovernors(ctx context.Context, host *ssh.Conn, states map[string]string) error {
	for path, value := range states {
		cmd := host.Command("sh", "-c", fmt.Sprintf("echo %q > %s", value, path))
		if err := cmd.Run(ctx); err != nil {
			// CR: See setScalingGovernors
			testing.ContextLogf(ctx, "unable to write scaling governor %q: %v", path, err)
		}
	}
	return nil
}
