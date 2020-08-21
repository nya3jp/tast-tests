// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package wifi

import (
	"bufio"
	"bytes"
	"context"
	"os"
	"os/exec"
	"strconv"
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

	largeFilePath = "/home/chronos/user/Downloads/video.webm"
)

var running = true

func init() {
	testing.AddTest(&testing.Test{
		Func:         MTBF058WifiDownload,
		Desc:         "MTBF058 To ensure the device can successfully finish a download while WiFi signal strength changes",
		SoftwareDeps: []string{"chrome", "arc"},
		Pre:          chrome.LoginReuse(),
		Attr:         []string{"group:mainline", "informational"},
		Contacts:     []string{"xliu@cienet.com"},
		Timeout:      10 * time.Minute,
		Vars: []string{
			"wifi.dutId",
			"wifi.atnApSsid",
			"wifi.atnApPwd",
			"wifi.downloadURL",
			"wifi.allionApiServer",
			"wifi.allionDevId",
			"wifi.attenuationId",
			"wifi.dlStatusGap",
			"wifi.detachStatusServer",
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
	dutID := s.RequiredVar("wifi.dutId")
	deviceID := s.RequiredVar("wifi.allionDevId")
	fileDownloadURL := s.RequiredVar("wifi.downloadURL")
	allionServerURL := s.RequiredVar("wifi.allionApiServer")
	detachStatusSvr := s.RequiredVar("wifi.detachStatusServer")
	wifiSsid := s.RequiredVar("wifi.atnApSsid")
	wifiPwd := s.RequiredVar("wifi.atnApPwd")
	allionAPI := allion.NewRestAPI(ctx, allionServerURL)
	attnID := s.RequiredVar("wifi.attenuationId")
	s.Logf("MTBF058WifiDownload - allionServerURL=%v, deviceID=%v ssid=%v", allionServerURL, deviceID, wifiSsid)
	common.InformStatusServlet(ctx, s, detachStatusSvr, "start", dutID, caseName)
	defer common.InformStatusServlet(ctx, s, detachStatusSvr, "end", dutID, caseName)
	wifiConn, mtbferr := wifi.NewConn(ctx, cr, true, wifiSsid, wifiPwd, allionServerURL, deviceID)

	if mtbferr != nil {
		s.Fatal(mtbferr)
	}

	defer wifiConn.Close(true)

	if mtbferr := wifiConn.ForgetAllWiFiAP(); mtbferr != nil {
		s.Fatal(mtbferr)
	}

	if mtbferr := wifiConn.ConnectToAp(); mtbferr != nil {
		s.Fatal(mtbferr)
	}

	go logWifiStrInfo(ctx, s, wifiConn)
	defer stopLoggingWifiInfo(s)
	disableEthernet(allionAPI, deviceID, s)
	testing.Sleep(ctx, 5000*time.Millisecond)

	if mtbferr := downloadFile(ctx, s, largeFilePath, fileDownloadURL); mtbferr != nil {
		s.Fatal(mtbferr)
	}

	setWifiStrBack(allionAPI, attnID, s)
	testing.Sleep(ctx, 3000*time.Millisecond)
	enableEthernet(allionAPI, deviceID, s)
	defer deleteFile(s, largeFilePath)
	testing.Sleep(ctx, 5000*time.Millisecond) // Sleep 5 seconds to wait for ethernet enabled to inform detach status
}

func disableEthernet(allionAPI *allion.RestAPI, deviceID string, s *testing.State) {
	mtbferr := allionAPI.DisableEthernet(deviceID)

	if mtbferr != nil {
		s.Log("Error occured while disabling ethernet: ", mtbferr)
		// It might be caused ethernet disconnected, so ignore it.
	}
}

func enableEthernet(allionAPI *allion.RestAPI, deviceID string, s *testing.State) {
	mtbferr := allionAPI.EnableEthernetWithRetry(deviceID, 3)

	if mtbferr != nil {
		s.Fatal(mtbferr)
	}
}

func downloadFile(ctx context.Context, s *testing.State, filePath, url string) error {
	s.Logf("Use wget to download a large file %v to %v", url, filePath)
	cmd := exec.Command("wget", "-O", filePath, url)
	stdoutStderr, err := cmd.CombinedOutput()
	// err := cmd.Run()

	if err != nil {
		s.Log("err: ", err)
		s.Log("sdtout and stderr:", string(stdoutStderr))
		return mtbferrors.New(mtbferrors.WIFIDownld, err, url)
	}

	outputLen := len(stdoutStderr)
	scanner := bufio.NewScanner(bytes.NewReader(stdoutStderr))
	dlStatusGap := getDownloadStatusGap(ctx, s, outputLen)
	i := 0
	s.Log("Downlaod status below. stdoutStderr size: ", outputLen)

	for scanner.Scan() {
		i++
		if i%dlStatusGap == 1 {
			// Print download status every $dlStatusGap lines
			s.Logf("%d: %s", i, scanner.Text())
		}
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

func setWifiStrBack(allionAPI *allion.RestAPI, attnID string, s *testing.State) {
	s.Log("Set WiFi strength back to the strongest")
	mtbferr := allionAPI.WifiStrManualWithRetry(attnID, "0", 3)

	if mtbferr != nil {
		s.Error(mtbferr)
	}
}

func logWifiStrInfo(ctx context.Context, s *testing.State, wifiConn *wifi.Conn) {
	for running {
		wifiInfo, err := wifiConn.GetWifiStrInfo()

		if err != nil {
			s.Log("Failed to get WiFi strength info: ", err)
			// ignore this eror
		}

		s.Log("WiFi strength info: ", wifiInfo)
		time.Sleep(time.Second * 10) // NOLINT
	}
}

func stopLoggingWifiInfo(s *testing.State) {
	s.Log("Stop changing wifi strength")
	running = false
}

func getDownloadStatusGap(ctx context.Context, s *testing.State, outputLen int) int {
	statusGap := -1
	statusGapStr, ok := s.Var("wifi.dlStatusGap")
	var err error

	if ok {
		statusGap, err = strconv.Atoi(statusGapStr)
		if err != nil {
			//ignore this error, because it's only for logging
			s.Log(mtbferrors.New(mtbferrors.OSVarRead, err, "wifi.dlStatusGap"))
		}
	} else {
		s.Log("wifi.dlStatusGap is not set. Use default value")
	}

	if statusGap == -1 {
		statusGap = outputLen / 2735
	}

	s.Logf("getDownloadStatusGap - outputLen=%d statusGap=%d", outputLen, statusGap)
	return statusGap
}
