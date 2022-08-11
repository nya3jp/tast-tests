// Copyright 2022 The ChromiumOS Authors.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package network

import (
	"context"
	"io/ioutil"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/golang/protobuf/ptypes/empty"

	"chromiumos/tast/common/testexec"
	"chromiumos/tast/ctxutil"
	"chromiumos/tast/remote/firmware/fixture"
	"chromiumos/tast/rpc"
	"chromiumos/tast/services/cros/network"
	"chromiumos/tast/services/cros/wifi"
	"chromiumos/tast/testing"
)

type ethernet struct {
	ethType string
}

func init() {
	testing.AddTest(&testing.Test{
		Func:         EthernetSpeed,
		LacrosStatus: testing.LacrosVariantUnneeded,
		Desc:         "Ethernet LAN Speed Test",
		SoftwareDeps: []string{"chrome"},
		Timeout:      3 * time.Minute,
		Data:         []string{"testing_rsa"},
		ServiceDeps:  []string{"tast.cros.network.EthernetService", "tast.cros.wifi.ShillService"},
		Contacts:     []string{"ambalavanan.m.m@intel.com", "intel-chrome-system-automation-team@intel.com"},
		Fixture:      fixture.NormalMode,
		Params: []testing.Param{{
			Name: "native",
			Val:  ethernet{ethType: "native"},
		}, {
			Name: "type_a",
			Val:  ethernet{ethType: "typeA"},
		}},
	})
}

func EthernetSpeed(ctx context.Context, s *testing.State) {
	h := s.FixtValue().(*fixture.Value).Helper

	testOpts := s.Param().(ethernet)

	if testOpts.ethType == "typeA" {
		usbDetectionRe := regexp.MustCompile(`Class=.*(480M|5000M|10G|20G)`)
		out, err := h.DUT.Conn().CommandContext(ctx, "lsusb", "-t").Output()
		if err != nil {
			s.Fatal("Failed to execute lsusb command: ", err)
		}

		if !usbDetectionRe.MatchString(string(out)) {
			s.Fatal("Failed: ethernet is not connected to DUT using Type-A adapter")
		}
	}

	cleanupCtx := ctx
	ctx, cancel := ctxutil.Shorten(ctx, 5*time.Second)
	defer cancel()

	testfileName := "test.img"
	tmpDir, err := ioutil.TempDir("", "tast-tmp")
	if err != nil {
		s.Fatal("Failed to create temp dir: ", err)
	}
	testfilePath := filepath.Join(tmpDir, testfileName)

	cl, err := rpc.Dial(ctx, h.DUT, s.RPCHint())
	if err != nil {
		s.Fatal("Failed to connect to the RPC service on the DUT: ", err)
	}
	defer cl.Close(cleanupCtx)

	// Disable WiFi.
	wifiClient := wifi.NewShillServiceClient(cl.Conn)
	if _, err := wifiClient.SetWifiEnabled(ctx, &wifi.SetWifiEnabledRequest{Enabled: false}); err != nil {
		s.Fatal("Could not disable Wifi: ", err)
	}

	// Assert WiFi is down.
	if response, err := wifiClient.GetWifiEnabled(ctx, &empty.Empty{}); err != nil {
		s.Fatal("Failed to WiFi status: ", err)
	} else if response.Enabled {
		s.Fatal("Failed: Wifi is on, expected to be off ")
	}

	client := network.NewEthernetServiceClient(cl.Conn)

	if _, err := client.New(ctx, &empty.Empty{}); err != nil {
		s.Fatal("Failed to start Chrome: ", err)
	}

	path, err := client.DownloadPath(ctx, &empty.Empty{})
	if err != nil {
		s.Fatal("Failed to get Download path: ", err)
	}

	// Chmod the keyfile so that ssh connections do not fail due to
	// open permissions.
	cmd := testexec.CommandContext(ctx, "cp", s.DataPath("testing_rsa"), tmpDir)
	if err := cmd.Run(); err != nil {
		s.Fatal("Failed to copy testing_rsa to tast temp dir: ", err)
	}
	sshKey := filepath.Join(tmpDir, "testing_rsa")
	if err := os.Chmod(sshKey, 0600); err != nil {
		s.Fatal("Unable to chmod sshkey to 0600: ", err)
	}

	// Generate a 1GB file for scp.
	if err := testexec.CommandContext(
		ctx, "dd", "if=/dev/zero", "of="+testfilePath, "bs=1", "count=0", "seek=1G").Run(testexec.DumpLogOnError); err != nil {
		s.Fatal("Failed to generate test file: ", err)
	}
	defer testexec.CommandContext(cleanupCtx, "rm", testfilePath).Run()

	hostName := s.DUT().HostName()
	dutIP := strings.Split(hostName, ":")[0]

	args := []string{"-i", sshKey, "-o", "StrictHostKeyChecking=no", "-o",
		"UserKnownHostsFile=/dev/null", testfilePath,
		"root@" + dutIP + ":" + path.DownloadPath}

	start := time.Now()
	if err = testexec.CommandContext(ctx, "scp", args...).Run(); err != nil {
		s.Fatal("Failed to do scp: ", err)
	}
	defer h.DUT.Conn().CommandContext(cleanupCtx, "rm", filepath.Join(path.DownloadPath, testfileName)).Run()

	elapsedTime := time.Since(start)

	expectedTime := 10 * time.Second
	if elapsedTime > expectedTime {
		s.Fatalf("Failed to transfer file within %v Seconds. Elapsed time: %v Seconds", expectedTime, elapsedTime)
	}

}
