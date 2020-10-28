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

	"chromiumos/tast/common/perf"
	"chromiumos/tast/common/wifi/security"
	"chromiumos/tast/ctxutil"
	"chromiumos/tast/errors"
	remoteiw "chromiumos/tast/remote/network/iw"
	"chromiumos/tast/remote/network/netperf"
	"chromiumos/tast/remote/wificell"
	ap "chromiumos/tast/remote/wificell/hostapd"
	"chromiumos/tast/services/cros/network"
	"chromiumos/tast/ssh"
	"chromiumos/tast/testing"
	"chromiumos/tast/timing"
)

type attenuatedPerfTestCase struct {
	config []netperf.Config
	apOpts []ap.Option
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
		Attr:        []string{"group:wificell", "wificell_perf", "wificell_unstable"},
		ServiceDeps: []string{"tast.cros.network.Wifi", "tast.cros.network.WifiService"},
		Pre:         wificell.TestFixturePreWithFeatures(wificell.TFFeaturesAttenuator),
		Vars: []string{"router", "routers", "attenuator", "pcap",
			"maxAttenuation", "attenuationStep", "freq", "seriesNote"},
		Timeout: 120 * time.Minute,
		Params: []testing.Param{
			{
				// Basic 802.11a test
				Name:      "80211a",
				ExtraAttr: []string{"wificell_unstable"},
				Val: []attenuatedPerfTestCase{
					{config: defaultNetperfCfg,
						apOpts: []ap.Option{ap.Mode(ap.Mode80211a), ap.Channel(48)}},
				},
			}, {
				// Basic 802.11b test
				Name:      "80211b",
				ExtraAttr: []string{"wificell_unstable"},
				Val: []attenuatedPerfTestCase{
					{config: defaultNetperfCfg,
						apOpts: []ap.Option{ap.Mode(ap.Mode80211b), ap.Channel(1)}},
					{config: defaultNetperfCfg,
						apOpts: []ap.Option{ap.Mode(ap.Mode80211b), ap.Channel(6)}},
				},
			}, {
				// Basic 802.11g test
				Name:      "80211g",
				ExtraAttr: []string{"wificell_unstable"},
				Val: []attenuatedPerfTestCase{
					{config: defaultNetperfCfg,
						apOpts: []ap.Option{ap.Mode(ap.Mode80211g), ap.Channel(6)}},
				},
			}, {
				// 802.11n 2.4G ht20
				Name:      "80211n24ht20",
				ExtraAttr: []string{"wificell_unstable"},
				Val: []attenuatedPerfTestCase{
					{config: defaultNetperfCfg,
						apOpts: []ap.Option{ap.Mode(ap.Mode80211nPure), ap.Channel(6), ap.HTCaps(ap.HTCapHT20)}},
				},
			}, {
				// 802.11n24ht20 test
				Name:      "80211n24ht40",
				ExtraAttr: []string{"wificell_unstable"},
				Val: []attenuatedPerfTestCase{
					{config: defaultNetperfCfg,
						apOpts: []ap.Option{ap.Mode(ap.Mode80211nPure), ap.Channel(6), ap.HTCaps(ap.HTCapHT40)}},
				},
			}}})
}

type dataPoint struct {
	atten         int
	throughput    float64
	throughputDev float64
	signalLevel   float64
	tag           string
}

const tsvOutputDir = "tsvs"

var seriesNote string
var boardName string

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
// NOTICE: This function is only intended for provifding name to test output files.
func hostBoard(ctx context.Context, host *ssh.Conn) (string, error) {
	ctx, st := timing.Start(ctx, "hostBoard")
	defer st.End()

	const lsbReleasePath = "/etc/lsb-release"
	const crosReleaseBoardKey = "CHROMEOS_RELEASE_BOARD"

	cmd := host.Command("cat", lsbReleasePath)
	out, err := cmd.Output(ctx)
	if err != nil {
		return "", errors.Wrapf(err, "failed to read %s", lsbReleasePath)
	}
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

// getSignalLevel checks on DUT what signal level is reported by iw.
func getSignalLevel(ctx context.Context, tf *wificell.TestFixture, host *ssh.Conn) int {
	clientIface, err := tf.ClientInterface(ctx)
	if err != nil {
		testing.ContextLog(ctx, "Failed to get the client interface: ", err)
		return -100
	}
	testing.ContextLog(ctx, "Checking client interface: ", clientIface)
	iwr := remoteiw.NewRemoteRunner(host)
	lvl, err := iwr.CurrentSignalLevel(ctx, clientIface)
	if err != nil {
		testing.ContextLog(ctx, "Failed to get the signal level: ", err)
		return -100
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
	testing.ContextLog(ctx, "Writing .tsv files")
	if err := os.MkdirAll(filepath.Join(s.OutDir(), tsvOutputDir), 0755); err != nil {
		return errors.Wrap(err, "cannot create tsv directory")
	}
	var seriesLabelParts []string
	seriesLabelParts = append(seriesLabelParts, boardName)
	// There's no equivalent of self.context.client.board yet.
	if seriesNote != "" {
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
		testing.ContextLog(ctx, "Filename: ", resultFileName)
		f, err := os.Create(resultFileName)
		if err != nil {
			return errors.Wrap(err, "cannot create tsv file")
		}
		defer f.Close()
		testing.ContextLog(ctx, strings.Join(seriesLabelParts, " "))
		if _, err = f.WriteString(strings.Join(seriesLabelParts, " ") + "\n"); err != nil {
			return errors.Wrap(err, "failed to write to tsv file (disk full?)")
		}
		testing.ContextLog(ctx, strings.Join(headerParts, "\t"))
		if _, err = f.WriteString(strings.Join(headerParts, "\t") + "\n"); err != nil {
			return errors.Wrap(err, "failed to write to tsv file (disk full?)")
		}
		for _, record := range throughputData {
			if record.tag == testType {
				testing.ContextLogf(ctx, "%d\t%f\t%f\t%f",
					record.atten, record.throughput,
					record.throughputDev, record.signalLevel)
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
	maxAttenuation := minAttenuation + tf.Attenuator().MaximumAttenuation()
	attenuationStep := float64(3)
	freq := 2400
	// These were command line args in original test, thus I'm leaving
	// opportunity to override them from cmd line.
	maxAttenuationStr, ok := s.Var("maxAttenuation")
	if ok && maxAttenuationStr != "" {
		val, err := strconv.ParseFloat(maxAttenuationStr, 64)
		if err != nil {
			s.Fatal("Failed to convert maxAttenuation value, err: ", err)
		}
		maxAttenuation = val
	}
	attenuationStepStr, ok := s.Var("attenuationStep")
	if ok && attenuationStepStr != "" {
		val, err := strconv.ParseFloat(attenuationStepStr, 64)
		if err != nil {
			s.Fatal("Failed to convert attenuationStep value, err: ", err)
		}
		attenuationStep = val
	}
	freqStr, ok := s.Var("freq")
	if ok && freqStr != "" {
		val, err := strconv.Atoi(freqStr)
		if err != nil {
			s.Fatal("Failed to convert freq value, err: ", err)
		}
		freq = val
	}
	seriesNote, ok = s.Var("seriesNote")

	s.Log("maxAttenuation: ", maxAttenuation)
	s.Log("attenuationStep: ", attenuationStep)
	s.Log("freq: ", freq)
	s.Log("seriesNote: ", seriesNote)

	defer func(ctx context.Context) {
		if err := tf.CollectLogs(ctx); err != nil {
			s.Log("Error collecting logs, err: ", err)
		}
	}(ctx)
	ctx, cancel := tf.ReserveForCollectLogs(ctx)
	defer cancel()

	boardName, _ = hostBoard(ctx, s.DUT().Conn())

	pv := perf.NewValues()
	defer func() {
		if err := pv.Save(s.OutDir()); err != nil {
			s.Log("Failed to save perf data, err: ", err)
		}
	}()

	testOnce := func(ctx context.Context, s *testing.State, tc attenuatedPerfTestCase) {
		tf.Attenuator().Reset(ctx)
		ap, err := tf.ConfigureAP(ctx, tc.apOpts, tc.secConfFac)
		if err != nil {
			s.Fatal("Failed to configure ap, err: ", err)
		}
		defer func(ctx context.Context) {
			if err := tf.DeconfigAP(ctx, ap); err != nil {
				s.Error("Failed to deconfig ap, err: ", err)
			}
		}(ctx)
		ctx, cancel := tf.ReserveForDeconfigAP(ctx, ap)
		defer cancel()
		s.Log("AP setup done")

		// Some tests may fail as expected at following ConnectWifiAP().
		// In that case entries should still be deleted properly.
		defer func(ctx context.Context) {
			req := &network.DeleteEntriesForSSIDRequest{Ssid: []byte(ap.Config().SSID)}
			if _, err := tf.WifiClient().DeleteEntriesForSSID(ctx, req); err != nil {
				s.Errorf("Failed to remove entries for ssid=%s, err: %v", ap.Config().SSID, err)
			}
		}(ctx)

		// Reserve time for deleting entries
		ctx, cancel = ctxutil.Shorten(ctx, time.Second)
		defer cancel()

		_, err = tf.ConnectWifiAP(ctx, ap)
		if err != nil {
			s.Fatal("Failed to connect to WiFi, err: ", err)
		}

		defer func(ctx context.Context) {
			if err := tf.CleanDisconnectWifi(ctx); err != nil {
				s.Error("Failed to disconnect WiFi, err: ", err)
			}
		}(ctx)

		ctx, cancel = tf.ReserveForDisconnect(ctx)
		defer cancel()
		s.Log("Connected")

		// LOL, tf cannot disconnect wifi if unable to connect, must reset
		// attenuation first.
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
			ap.ServerIP().String()) // Router IP

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
		for attenuation := minAttenuation; attenuation < maxAttenuation; attenuation += attenuationStep {
			testing.ContextLogf(ctx, "Step: %f", attenuation)
			tf.Attenuator().SetTotalAttenuationAllCh(ctx, attenuation, freq)
			attenTag := fmt.Sprintf("atten%03d", int(attenuation))
			singalLevel := float64(getSignalLevel(ctx, tf, s.DUT().Conn()))
			for _, cfg := range tc.config {
				runHistory, err := netperfSession.Run(ctx, cfg)
				if err != nil {
					if attenuation == minAttenuation {
						// For first measurement, this is definitely an error.
						s.Fatal("Failed to run netperf, err: ", err)
					}

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
					// We're leveraging fact that either throughput or transaction rate is empty
					throughput: aggregate.Measurements[netperf.CategoryThroughput] +
						aggregate.Measurements[netperf.CategoryTransactionRate],
					throughputDev: aggregate.Measurements[netperf.CategoryThroughputDev] +
						aggregate.Measurements[netperf.CategoryTransactionRateDev],
					signalLevel: singalLevel,
					tag:         cfg.HumanReadableTag()})
				var throughputVals []float64
				for _, sample := range runHistory {
					throughputVals = append(throughputVals,
						sample.Measurements[netperf.CategoryThroughput]+
							sample.Measurements[netperf.CategoryTransactionRate])
				}
				graphName := fmt.Sprintf("%s.%s", ap.Config().PerfDesc(), cfg.ShortTag())
				pv.Append(perf.Metric{
					Name:      graphName,
					Variant:   attenTag,
					Unit:      "dBm",
					Direction: perf.BiggerIsBetter,
					Multiple:  true}, throughputVals...)

			}
		}
		writeThroughputTSVFiles(ctx, s, throughputData, ap)
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
