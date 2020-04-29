// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package wifi

import (
	"context"
	"os"
	"os/exec"
	"time"

	"chromiumos/tast/common/allion"
	"chromiumos/tast/common/mtbferrors"
	"chromiumos/tast/local/bundles/mtbf/wifi/common"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/mtbf/wifi"
	"chromiumos/tast/testing"
	"chromiumos/tast/timing"
)

const (
	// 1GB
	//largeFileURL = "http://storage.googleapis.com/chromiumos-test-assets-public/Shaka-Dash/2160.webm"

	// 280MB 1m27.358s 18.234s
	//largeFileURL = "http://storage.googleapis.com/chromiumos-test-assets-public/Shaka-Dash/1440_vp8.webm"

	// 85MB 10 seconds
	// largeFileURL = "http://storage.googleapis.com/chromiumos-test-assets-public/Shaka-Dash/1080_vp8.webm"

	//largeFileURL  = "%v/2160.webm"
	wifiSsid      = "tc58.wifi.ssid"
	wifiPassword  = "tc58.wifi.pwd"
	largeFilePath = "/home/chronos/user/Downloads/video.webm"
)

var running = true

func init() {
	testing.AddTest(&testing.Test{
		Func:         MTBF058WifiDownload,
		Desc:         "MTBF058 To ensure the device can successfully finish a download while WiFi signal strength changes",
		SoftwareDeps: []string{"chrome"},
		Pre:          chrome.LoginReuse(),
		Attr:         []string{"group:mainline"},
		Contacts:     []string{"xliu@cienet.com"},
		Timeout:      10 * time.Minute,
		Vars: []string{
			"dut.id",
			"tc58.wifi.ssid",
			"tc58.wifi.pwd",
			"file.download.url",
			"allion.api.server",
			"allion.deviceId",
			"tc58.wifi.att.id",
			"detach.status.server",
		},
	})
}

// MTBF058WifiDownload testing the test case MTBF058
func MTBF058WifiDownload(ctx context.Context, s *testing.State) {
	s.Log("Start to run sub case --- MTBF058WifiDownload")
	ctx, st := timing.Start(ctx, "mtbf058_wifi_download")
	cr := s.PreValue().(*chrome.Chrome)
	defer st.End()

	caseName := "wifi.MTBF058WifiDownload"
	dutID := common.GetVar(ctx, s, "dut.id")
	deviceID := common.GetVar(ctx, s, "allion.deviceId")
	fileDownloadURL := common.GetVar(ctx, s, "file.download.url")
	allionServerURL := common.GetVar(ctx, s, "allion.api.server")
	detachStatusSvr := common.GetVar(ctx, s, "detach.status.server")
	wifiSsid := common.GetVar(ctx, s, wifiSsid)
	wifiPwd := common.GetVar(ctx, s, wifiPassword)
	allionAPI := allion.NewRestAPI(ctx, allionServerURL)
	attnID := common.GetVar(ctx, s, "tc58.wifi.att.id")
	s.Logf("MTBF058WifiDownload - allionServerURL=%v, deviceID=%v ssid=%v", allionServerURL, deviceID, wifiSsid)
	common.InformStatusServlet(ctx, s, detachStatusSvr, "start", dutID, caseName)
	defer common.InformStatusServlet(ctx, s, detachStatusSvr, "end", dutID, caseName)
	defer stopChangingWifiStr(s)
	setWifiStrBack(allionAPI, attnID, s)

	wifiConn, mtbferr := wifi.NewConn(ctx, cr, true, wifiSsid, wifiPwd, allionServerURL, deviceID)

	if mtbferr != nil {
		s.Fatal(mtbferr)
	}

	defer wifiConn.Close()

	if mtbferr := wifiConn.ForgetAllWiFiAP(); mtbferr != nil {
		s.Fatal(mtbferr)
	}

	if mtbferr := wifiConn.ConnectToAp(); mtbferr != nil {
		s.Fatal(mtbferr)
	}

	disableEthernet(allionAPI, deviceID, s)
	testing.Sleep(ctx, 5000*time.Millisecond)

	go changeWifiStrength(ctx, s, allionAPI, attnID)

	if mtbferr := downloadFile(s, largeFilePath, fileDownloadURL); mtbferr != nil {
		s.Fatal(mtbferr)
	}

	// Enable ethernett to make the API request less likely to fail
	stopChangingWifiStr(s)
	testing.Sleep(ctx, 1000*time.Millisecond)
	setWifiStrBack(allionAPI, attnID, s)
	testing.Sleep(ctx, 3000*time.Millisecond)
	enableEthernet(allionAPI, deviceID, s)
	defer deleteFile(s, largeFilePath)
	testing.Sleep(ctx, 5000*time.Millisecond) // Sleep 5 seconds to wait for ethernet enabled to inform detach status
}

func disableEthernet(allionAPI *allion.RestAPI, deviceID string, s *testing.State) {
	mtbferr := allionAPI.DisableEthernet(deviceID)

	if mtbferr != nil {
		s.Fatal(mtbferr)
	}
}

func enableEthernet(allionAPI *allion.RestAPI, deviceID string, s *testing.State) {
	mtbferr := allionAPI.EnableEthernetWithRetry(deviceID, 3)

	if mtbferr != nil {
		s.Fatal(mtbferr)
	}
}

func downloadFile(s *testing.State, filePath string, url string) error {
	s.Logf("Use wget to download a large file %v to %v", url, filePath)
	cmd := exec.Command("wget", "-O", filePath, url)
	stdoutStderr, err := cmd.CombinedOutput()
	// err := cmd.Run()

	if err != nil {
		s.Log("err: ", err)
		s.Log("sdtout and stderr:", string(stdoutStderr))
		return mtbferrors.New(mtbferrors.WIFIDownld, err, url)
	}

	// s.Log("sdtout and stderr: ", string(stdoutStderr))
	s.Log("Download finished")
	return nil
}

func deleteFile(s *testing.State, filePath string) error {
	s.Log("Delete the file ", filePath)

	if err := os.Remove(filePath); err != nil {
		s.Log("Failed to delete file " + filePath)
		return err
	}

	return nil
}

func changeWifiStrength(ctx context.Context, s *testing.State, allionAPI *allion.RestAPI, attnID string) {
	wifiStrength := [6]string{"5", "15", "20", "30", "40", "0"}
	i := 0
	s.Log("Start changing WiFi strength")

	for running {
		strength := wifiStrength[i%6]
		s.Log("Change wifi strength to be reduced: ", strength)
		mtbferr := allionAPI.WifiStrManual(attnID, strength)

		if mtbferr != nil {
			// Ignore this error and keep changing wifi strength
			// Not sure if calling s.Fatal in a go routine will stop the test case running.
			s.Error("Failed to change wifi strength: ", mtbferr)
		}

		i++
		testing.Sleep(ctx, 3000*time.Millisecond)
	}

	s.Log("Changing WiFi strength finished")
}

func stopChangingWifiStr(s *testing.State) {
	s.Log("Stop changing wifi strength")
	running = false
}

func setWifiStrBack(allionAPI *allion.RestAPI, attnID string, s *testing.State) {
	s.Log("Set WiFi strength back to the strongest")
	mtbferr := allionAPI.WifiStrManual(attnID, "0")

	if mtbferr != nil {
		s.Error(mtbferr)
	}
}
