// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package wifi

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/golang/protobuf/ptypes/empty"

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
	"chromiumos/tast/testing/hwdep"
	"chromiumos/tast/timing"
)

type attenuatedPerfTestCase struct {
	config                          []netperf.Config
	maxAttenuation, attenuationStep int
	apOpts                          []ap.Option
	// If unassigned, use default security config: open network.
	secConfFac security.ConfigFactory
}

var defaultNetperfCfg = []netperf.Config{
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
		Func:        AttenuatedPerf,
		Desc:        "Test maximal achievable bandwidth while varying attenuation",
		Contacts:    []string{"jck@semihalf.com", "chromeos-kernel-wifi@google.com"},
		Attr:        []string{"group:wificell", "wificell_unstable"},
		ServiceDeps: []string{"tast.cros.network.WifiService"},
		Pre:         wificell.TestFixturePreWithFeatures(wificell.TFFeaturesAttenuator),
		Vars: []string{"router", "attenuator", "pcap",
			"maxAttenuation", "attenuationStep", "seriesNote"},
		Timeout: 120 * time.Minute,
		Params: []testing.Param{
			{
				// Basic 802.11a test
				Name: "80211a",
				Val: []attenuatedPerfTestCase{{
					config:          defaultNetperfCfg,
					apOpts:          []ap.Option{ap.Mode(ap.Mode80211a), ap.Channel(48)},
					maxAttenuation:  100,
					attenuationStep: 3,
				}},
			}, {
				// Basic 802.11b test
				Name: "80211b",
				Val: []attenuatedPerfTestCase{{
					config:          defaultNetperfCfg,
					apOpts:          []ap.Option{ap.Mode(ap.Mode80211b), ap.Channel(1)},
					maxAttenuation:  100,
					attenuationStep: 3,
				}, {
					config:          defaultNetperfCfg,
					apOpts:          []ap.Option{ap.Mode(ap.Mode80211b), ap.Channel(6)},
					maxAttenuation:  100,
					attenuationStep: 3,
				}},
			}, {
				// Basic 802.11g test
				Name: "80211g",
				Val: []attenuatedPerfTestCase{{
					config:          defaultNetperfCfg,
					apOpts:          []ap.Option{ap.Mode(ap.Mode80211g), ap.Channel(6)},
					maxAttenuation:  100,
					attenuationStep: 3,
				}},
			}, {
				// 802.11n 2.4G ht20
				Name: "ht20_ch006",
				Val: []attenuatedPerfTestCase{{
					config:          defaultNetperfCfg,
					apOpts:          []ap.Option{ap.Mode(ap.Mode80211nPure), ap.Channel(6), ap.HTCaps(ap.HTCapHT20)},
					maxAttenuation:  100,
					attenuationStep: 3,
				}},
			}, {
				// 802.11n 2.4G ht20
				Name: "ht20_ch006_wpa",
				Val: []attenuatedPerfTestCase{{
					config: defaultNetperfCfg,
					apOpts: []ap.Option{ap.Mode(ap.Mode80211nPure), ap.Channel(6), ap.HTCaps(ap.HTCapHT20)},
					secConfFac: wpa.NewConfigFactory(
						"chromeos", wpa.Mode(wpa.ModePureWPA),
						wpa.Ciphers(wpa.CipherTKIP),
					),
					maxAttenuation:  100,
					attenuationStep: 3,
				}},
			}, {
				// Original autotest testcases:
				// ht40_ch001
				Name: "ht40_ch001",
				Val: []attenuatedPerfTestCase{{
					config: defaultNetperfCfg,
					apOpts: []ap.Option{ap.Mode(ap.Mode80211nPure),
						ap.Channel(1), ap.HTCaps(ap.HTCapHT40)},
					maxAttenuation:  100,
					attenuationStep: 4,
				}},
			}, {
				// ht40_ch006
				Name: "ht40_ch006",
				Val: []attenuatedPerfTestCase{{
					config: defaultNetperfCfg,
					apOpts: []ap.Option{ap.Mode(ap.Mode80211nPure),
						ap.Channel(6), ap.HTCaps(ap.HTCapHT40Plus)},
					maxAttenuation:  100,
					attenuationStep: 4,
				}},
			}, {
				// ht40_ch011
				Name: "ht40_ch011",
				Val: []attenuatedPerfTestCase{{
					config: defaultNetperfCfg,
					apOpts: []ap.Option{ap.Mode(ap.Mode80211nPure),
						ap.Channel(11), ap.HTCaps(ap.HTCapHT40Minus)},
					maxAttenuation:  100,
					attenuationStep: 4,
				}},
			}, {
				// ht40_ch044
				Name: "ht40_ch044",
				Val: []attenuatedPerfTestCase{{
					config: defaultNetperfCfg,
					apOpts: []ap.Option{ap.Mode(ap.Mode80211nPure),
						ap.Channel(44), ap.HTCaps(ap.HTCapHT40)},
					maxAttenuation:  100,
					attenuationStep: 4,
				}},
			}, {
				// ht40_ch153
				Name: "ht40_ch153",
				Val: []attenuatedPerfTestCase{{
					config: defaultNetperfCfg,
					apOpts: []ap.Option{ap.Mode(ap.Mode80211nPure),
						ap.Channel(153), ap.HTCaps(ap.HTCapHT40)},
					maxAttenuation:  100,
					attenuationStep: 4,
				}},
			}, {
				// vht40_ch036
				Name: "vht40_ch036",
				Val: []attenuatedPerfTestCase{{
					config: defaultNetperfCfg,
					apOpts: []ap.Option{
						ap.Mode(ap.Mode80211acPure), ap.Channel(36),
						ap.HTCaps(ap.HTCapHT40), ap.VHTChWidth(ap.VHTChWidth20Or40)},
					maxAttenuation:  100,
					attenuationStep: 6,
				}},
				ExtraHardwareDeps: hwdep.D(hwdep.Wifi80211ac()),
			}, {
				// vht40_ch060
				Name: "vht40_ch060",
				Val: []attenuatedPerfTestCase{{
					config: defaultNetperfCfg,
					apOpts: []ap.Option{
						ap.Mode(ap.Mode80211acPure), ap.Channel(60),
						ap.HTCaps(ap.HTCapHT40), ap.VHTChWidth(ap.VHTChWidth20Or40)},
					maxAttenuation:  100,
					attenuationStep: 6,
				}},
			}, {
				// vht40_ch149
				Name: "vht40_ch0149",
				Val: []attenuatedPerfTestCase{{
					config: defaultNetperfCfg,
					apOpts: []ap.Option{
						ap.Mode(ap.Mode80211acPure), ap.Channel(149),
						ap.HTCaps(ap.HTCapHT40), ap.VHTChWidth(ap.VHTChWidth20Or40)},
					maxAttenuation:  100,
					attenuationStep: 6,
				}},
			}, {
				// vht40_ch157
				Name: "vht40_ch157",
				Val: []attenuatedPerfTestCase{{
					config: defaultNetperfCfg,
					apOpts: []ap.Option{
						ap.Mode(ap.Mode80211acPure), ap.Channel(157),
						ap.HTCaps(ap.HTCapHT40), ap.VHTChWidth(ap.VHTChWidth20Or40)},
					maxAttenuation:  100,
					attenuationStep: 6,
				}},
				ExtraHardwareDeps: hwdep.D(hwdep.Wifi80211ac()),
			}, {
				// vht80_ch042
				Name: "vht80_ch042",
				Val: []attenuatedPerfTestCase{{
					config: defaultNetperfCfg,
					apOpts: []ap.Option{
						ap.Mode(ap.Mode80211acPure), ap.Channel(44),
						ap.HTCaps(ap.HTCapHT40Plus), ap.VHTCaps(ap.VHTCapSGI80),
						ap.VHTCenterChannel(42), ap.VHTChWidth(ap.VHTChWidth80),
					},
					maxAttenuation:  100,
					attenuationStep: 6,
				}},
				ExtraHardwareDeps: hwdep.D(hwdep.Wifi80211ac()),
			}, {
				// vht80_ch155
				Name: "vht80_ch155",
				Val: []attenuatedPerfTestCase{{
					config: defaultNetperfCfg,
					apOpts: []ap.Option{
						ap.Mode(ap.Mode80211acPure), ap.Channel(157),
						ap.HTCaps(ap.HTCapHT40Plus), ap.VHTCaps(ap.VHTCapSGI80),
						ap.VHTCenterChannel(155), ap.VHTChWidth(ap.VHTChWidth80),
					},
					maxAttenuation:  100,
					attenuationStep: 6,
				}},
				ExtraHardwareDeps: hwdep.D(hwdep.Wifi80211ac()),
			}}})
}

// dataPoint holds data chunk meant to be written to .tsv file.
type dataPoint struct {
	// atten is total line attenuation set when this data point was collected.
	atten int
	// throughput in Mbps.
	throughput float64
	// throughputDev measures standard deviation for throughput in this data point.
	throughputDev float64
	// signalLevel signal level as seen by DUT under the given attenuation.
	signalLevel float64
	// tag human-readable configuration description for this data point.
	tag string
}

// testTypes extracts slice of testtypes present in throughput data.
func testTypes(throughputData []dataPoint) []string {
	dataMap := map[string]int{}
	// Putting tags as key maps will remove duplicates.
	for _, record := range throughputData {
		dataMap[record.tag] = 1
	}
	// Now, simply extract keys.
	var keys []string
	for key := range dataMap {
		keys = append(keys, key)
	}
	return keys
}

// hostBoard returns the board information on a chromeos host.
// NOTICE: This function is only intended for providing name to test output files.
func hostBoard(ctx context.Context, host *ssh.Conn) (string, error) {
	ctx, st := timing.Start(ctx, "hostBoard")
	defer st.End()

	const lsbReleasePath = "/etc/lsb-release"
	const crosReleaseBoardKey = "CHROMEOS_RELEASE_BOARD"

	cmd := host.Command("grep", crosReleaseBoardKey, lsbReleasePath)
	out, err := cmd.Output(ctx)
	if err != nil {
		return "", errors.Wrapf(err, "failed to read %s", lsbReleasePath)
	}
	// There might be other strings that meet the pattern, double-check.
	for _, line := range strings.Split(string(out), "\n") {
		tokens := strings.SplitN(strings.TrimSpace(line), "=", 2)
		if len(tokens) != 2 {
			continue
		}
		if tokens[0] == crosReleaseBoardKey {
			return tokens[1], nil
		}
	}
	return "", errors.Errorf("no %s key found in %s", crosReleaseBoardKey, lsbReleasePath)
}

// varDefaultInt is state.Var() variant which returns default value when no command
// line arguments were provided.
func varDefaultInt(s *testing.State, name string, defaultVal int) int {
	str, ok := s.Var(name)
	if ok && str != "" {
		val, err := strconv.Atoi(str)
		if err != nil {
			s.Fatal("Failed to convert value, err: ", err)
		}
		return val
	}
	return defaultVal
}

// getSignalLevel checks on DUT what signal level is reported by iw.
// In case of error, simply return low enough value. This function is necessary
// only for reults presentation, it should not cause the test to fail.
func getSignalLevel(ctx context.Context, tf *wificell.TestFixture, host *ssh.Conn) int {
	clientIface, err := tf.ClientInterface(ctx)
	if err != nil {
		testing.ContextLog(ctx, "Failed to get the client interface: ", err)
		return -120
	}
	testing.ContextLog(ctx, "Checking client interface: ", clientIface)
	iwr := remoteiw.NewRemoteRunner(host)
	lvl, err := iwr.CurrentSignalLevel(ctx, clientIface)
	if err != nil {
		testing.ContextLog(ctx, "Failed to get the signal level: ", err)
		return -120
	}
	return lvl
}

// writeThroughputTSVFiles writes .tsv files with plotable data from throughputData.
// Each .tsv file starts with a label for the series that can be customized with
// a short note passed in from the command line. It then has column headers
// and fields separated by tabs. This format is easy to parse and also works well
// with spreadsheet programs for custom report generation.
func writeThroughputTSVFiles(ctx context.Context, s *testing.State,
	throughputData []dataPoint, ap *wificell.APIface) error {
	const tsvOutputDir = "tsvs"

	testing.ContextLog(ctx, "Writing .tsv files")
	if err := os.MkdirAll(filepath.Join(s.OutDir(), tsvOutputDir), 0755); err != nil {
		return errors.Wrap(err, "cannot create tsv directory")
	}

	boardName, _ := hostBoard(ctx, s.DUT().Conn())
	var seriesLabelParts []string
	seriesLabelParts = append(seriesLabelParts, boardName)
	// If no series note given, it will be empty, we're fine with that.

	if seriesNote, _ := s.Var("seriesNote"); seriesNote != "" {
		seriesLabelParts = append(seriesLabelParts, seriesNote)
	}
	seriesLabelParts = append(seriesLabelParts,
		fmt.Sprintf("ch%03d", ap.Config().Channel))

	// Sort whole results set by attenuation, sort doesn't need to be stable.
	sort.Slice(throughputData, func(i, j int) bool {
		return throughputData[i].atten < throughputData[j].atten
	})

	var headerParts = []string{"Attenuation", "Throughput(Mbps)", "StdDev(Mbps)",
		"Client Reported Signal"}
	for _, testType := range testTypes(throughputData) {
		resultFileName := filepath.Join(s.OutDir(), tsvOutputDir,
			fmt.Sprintf("%s_%s.tsv", ap.Config().PerfDesc(), testType))
		f, err := os.Create(resultFileName)
		if err != nil {
			return errors.Wrap(err, "cannot create tsv file")
		}
		defer f.Close()
		if _, err = f.WriteString(strings.Join(seriesLabelParts, " ") + "\n"); err != nil {
			return errors.Wrap(err, "failed to write to tsv file (disk full?)")
		}
		if _, err = f.WriteString(strings.Join(headerParts, "\t") + "\n"); err != nil {
			return errors.Wrap(err, "failed to write to tsv file (disk full?)")
		}
		for _, record := range throughputData {
			if record.tag == testType {
				if _, err = f.WriteString(fmt.Sprintf("%d\t%f\t%f\t%f\n",
					record.atten, record.throughput,
					record.throughputDev, record.signalLevel)); err != nil {
					return errors.Wrap(err, "failed to write to tsv file (disk full?)")
				}
			}
		}
	}
	return nil
}

// AttenuatedPerf reports transmission performance metrics while varying attenuation.
func AttenuatedPerf(ctx context.Context, s *testing.State) {
	tf := s.PreValue().(*wificell.TestFixture)
	minAttenuation, _ := tf.Attenuator().MinTotalAttenuation(0)
	maxTotalAttenuation := minAttenuation + tf.Attenuator().MaximumAttenuation()

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

	testOnce := func(ctx context.Context, s *testing.State, tc attenuatedPerfTestCase) {
		maxAttenuation := varDefaultInt(s, "maxAttenuation", tc.maxAttenuation)
		if maxAttenuation > int(maxTotalAttenuation) {
			s.Fatal("Required max attenuation is greater than available")
		}
		attenuationStep := varDefaultInt(s, "attenuationStep", tc.attenuationStep)
		s.Log("maxAttenuation: ", maxAttenuation)
		s.Log("attenuationStep: ", attenuationStep)
		tf.Attenuator().Reset(ctx)
		ap1, err := tf.ConfigureAP(ctx, tc.apOpts, tc.secConfFac)
		if err != nil {
			s.Fatal("Failed to configure ap, err: ", err)
		}
		defer func(ctx context.Context) {
			if err := tf.DeconfigAP(ctx, ap1); err != nil {
				s.Error("Failed to deconfig ap, err: ", err)
			}
		}(ctx)
		ctx, cancel := tf.ReserveForDeconfigAP(ctx, ap1)
		defer cancel()
		s.Log("AP setup done")

		if _, err = tf.ConnectWifiAP(ctx, ap1); err != nil {
			s.Fatal("Failed to connect to WiFi, err: ", err)
		}

		freq, err := ap.ChannelToFrequency(ap1.Config().Channel)
		if err != nil {
			s.Fatal("Cannot convert channel to frequnecy, err: ", err)
		}
		s.Log("freq: ", freq)

		defer func(ctx context.Context) {
			if err := tf.ForceDisconnectWifi(ctx, ap1.Config().SSID); err != nil {
				s.Error("Failed to disconnect WiFi, err: ", err)
			}
		}(ctx)

		ctx, cancel = tf.ReserveForDisconnect(ctx)
		defer cancel()
		s.Log("Connected")

		// Reset attenuator to limit errors in disconnect logs
		// when overreaching attenuation.
		defer func(ctx context.Context) {
			if err := tf.Attenuator().Reset(ctx); err != nil {
				s.Log("Failed to reset attenuator, err: ", err)
			}
		}(ctx)
		// Reserve time for attenuator reset
		ctx, cancel = ctxutil.Shorten(ctx, 3*time.Second)
		defer cancel()

		addrs, err := tf.ClientIPv4Addrs(ctx)
		if err != nil || len(addrs) == 0 {
			s.Fatal("Failed to get the IP address, err: ", err)
		}

		netperfSession := netperf.NewSession(
			s.DUT().Conn(),
			addrs[0].String(), // DUT IP
			tf.Router().Conn(),
			ap1.ServerIP().String()) // Router IP

		defer func(ctx context.Context) {
			netperfSession.Close(ctx)
		}(ctx)

		testing.ContextLog(ctx, "Warming up stations")
		err = netperfSession.WarmupStations(ctx)
		if err != nil {
			s.Fatal("Warm up stations failed, err: ", err)
		}

		var throughputData []dataPoint

	outTest:
		for attenuation := int(minAttenuation); attenuation < maxAttenuation; attenuation += attenuationStep {
			testing.ContextLogf(ctx, "Step: %d", attenuation)
			tf.Attenuator().SetTotalAttenuationAllCh(ctx, float64(attenuation), freq)
			attenTag := fmt.Sprintf("atten%03d", attenuation)
			signalLevel := float64(getSignalLevel(ctx, tf, s.DUT().Conn()))
			testing.ContextLogf(ctx, "Signal level: %f", signalLevel)
			_, err := tf.WifiClient().SelectedService(ctx, &empty.Empty{})
			if err != nil || tf.PingFromDUT(ctx, ap1.ServerIP().String()) != nil {
				testing.ContextLogf(ctx,
					"Attenuation %d too high; aborting", attenuation)
				break
			}
			for _, cfg := range tc.config {
				runHistory, err := netperfSession.Run(ctx, cfg)
				if err != nil {
					if attenuation == int(minAttenuation) {
						// For first measurement, this is definitely an error.
						s.Fatal("Failed to run netperf, err: ", err)
					}

					// Depending on the test arguments, we may end up in attenuation
					// high enough, that no measurement may be possible.
					// In such case netperf will return error.
					// However, knowing that some previous results were already recorded,
					// we don't want the test to fail, but to abort sample recording,
					// so we can proceed to resutls generation.
					testing.ContextLogf(ctx, "Unable to take measurement for %s; aborting",
						cfg.HumanReadableTag())
					break outTest
				}
				aggregate, err := netperf.AggregateSamples(ctx, runHistory)
				if err != nil {
					s.Fatal("Failed to obtain results, err: ", err)
				}

				throughputData = append(throughputData, dataPoint{
					atten: int(attenuation),
					// We're leveraging the fact, that either throughput
					// or transaction rate is empty.
					throughput: aggregate.Measurements[netperf.CategoryThroughput] +
						aggregate.Measurements[netperf.CategoryTransactionRate],
					throughputDev: aggregate.Measurements[netperf.CategoryThroughputDev] +
						aggregate.Measurements[netperf.CategoryTransactionRateDev],
					signalLevel: signalLevel,
					tag:         cfg.HumanReadableTag()})
				var throughputVals []float64
				for _, sample := range runHistory {
					throughputVals = append(throughputVals,
						sample.Measurements[netperf.CategoryThroughput]+
							sample.Measurements[netperf.CategoryTransactionRate])
				}
				graphName := fmt.Sprintf("%s.%s", ap1.Config().PerfDesc(), cfg.ShortTag())
				pv.Append(perf.Metric{
					Name:      graphName,
					Variant:   attenTag,
					Unit:      "dBm",
					Direction: perf.BiggerIsBetter,
					Multiple:  true}, throughputVals...)

			}
		}
		writeThroughputTSVFiles(ctx, s, throughputData, ap1)
	}

	testcases := s.Param().([]attenuatedPerfTestCase)
	for i, tc := range testcases {
		subtest := func(ctx context.Context, s *testing.State) {
			testOnce(ctx, s, tc)
		}
		if !s.Run(ctx, fmt.Sprintf("Testcase #%d", i), subtest) {
			// Stop if any sub-test failed.
			return
		}
	}
	s.Log("Tearing down")
}
