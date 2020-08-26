// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package meta

import (
	"context"
	"strconv"
	"strings"
	"sync"
	"time"

	"chromiumos/tast/common/allion"
	"chromiumos/tast/common/attenuator"
	"chromiumos/tast/common/mtbferrors"
	"chromiumos/tast/remote/bundles/mtbf/meta/common"
	"chromiumos/tast/remote/bundles/mtbf/meta/tastrun"
	"chromiumos/tast/testing"
)

var running = true

func init() {
	testing.AddTest(&testing.Test{
		Func:         MTBF058WifiDownloadMain,
		Desc:         "MTBF058 To ensure the device can successfully finish a download while WiFi signal strength changes",
		SoftwareDeps: []string{"chrome"},
		Attr:         []string{"group:mainline", "informational"},
		Contacts:     []string{"xliu@cienet.com"},
		Timeout:      15 * time.Minute,
		Vars: []string{
			"wifi.dutId",
			"wifi.allionApiServer",
			"wifi.allionDevId",
			"wifi.detachStatusServer",
			"wifi.attenuationId",
			"wifi.attLeadTime",
			"wifi.strInterval",
			"wifi.attenuateStrength",
			"wifi.attenuatorURL",
		},
		ServiceDeps: []string{
			"tast.mtbf.svc.CommService",
		},
	})
}

func MTBF058WifiDownloadMain(ctx context.Context, s *testing.State) {
	s.Log("Start to run MTBF058WifiDownloadMain sub case --- MTBF058WifiDownload")
	localCaseName := "wifi.MTBF058WifiDownload"
	dutID := s.RequiredVar("wifi.dutId")
	deviceID := s.RequiredVar("wifi.allionDevId")
	detachStatusSvr := s.RequiredVar("wifi.detachStatusServer")
	allionServerURL := s.RequiredVar("wifi.allionApiServer")
	attnID := s.RequiredVar("wifi.attenuationId")
	s.Logf("dutID=%v, deviceID=%v, detachStatusSvr=%v, allionServerURL=%v", dutID, deviceID, detachStatusSvr, allionServerURL)
	allionAPI := allion.NewRestAPI(ctx, allionServerURL)
	attenuatorAPI := getAttenuatorAPI(ctx, s)
	defer common.EnableEthernet(ctx, s, allionServerURL, deviceID)
	defer setWifiStrBack(attenuatorAPI, allionAPI, attnID, s)

	// IMPORTANT: set a right detachDuration
	// flags := common.GetDetachedWithSvrFlags(s, 10, detachSvr)
	flags := common.GetDetachedFlags(s, 600)

	// flags for running with detach mode
	// concurrent := s.Param().(bool)

	// here for demo purpose, we just sleep 5 seconds
	s.Log("Step 1: Change WiFi signal to strongest")
	setWifiStrBack(attenuatorAPI, allionAPI, attnID, s)

	//need to forget all WiFi AP

	// 2. Calling tast to run local test in DUT
	var wg sync.WaitGroup
	wg.Add(1)

	go func(waitGroup *sync.WaitGroup) {
		defer common.EnableEthernet(ctx, s, allionServerURL, deviceID)
		defer stopChangingWifiStr(s)
		defer setWifiStrBack(attenuatorAPI, allionAPI, attnID, s)
		defer waitGroup.Done()
		s.Log("Step 2: Start calling tast to run a local test")

		if err := tastrun.RunTestWithFlags(ctx, s, flags, localCaseName); err != nil {
			if strings.Contains(err.Error(), "Test did not finish") {
				s.Fatal(mtbferrors.New(mtbferrors.WIFIDownldTimeout, err))
			} else if strings.Contains(err.Error(), "[ERR-") {
				s.Log("it's an MTBFError: ", err)
				mtbferr := err
				s.Fatal(mtbferr)
			} else {
				s.Fatal(mtbferrors.New(mtbferrors.WIFIDownldR, err))
			}
		}

		s.Log("Step 2: End calling tast to run a local test")
	}(&wg)

	s.Log("Step 3: Begin to change WiFi strength")
	go changeWifiStrength(ctx, s, attenuatorAPI, allionAPI, attnID)
	s.Log("Step 4: Polling the status server to check if sub case finished")

	if err := common.PollDetachedCaseDone(ctx, s, detachStatusSvr, dutID, localCaseName); err != nil {
		s.Fatal(mtbferrors.New(mtbferrors.WIFIDtchPollSubCase, err, "MTBF058"))
	}

	common.EnableEthernet(ctx, s, allionServerURL, deviceID)
	testing.Sleep(ctx, 1000*time.Millisecond)
	wg.Wait()
	stopChangingWifiStr(s)
	setWifiStrBack(attenuatorAPI, allionAPI, attnID, s)
	s.Log("End running mtbf sub-tests")
}

func getAttLeadTime(ctx context.Context, s *testing.State) int {
	var leadTime int
	leadTimeStr, ok := s.Var("wifi.attLeadTime")
	var err error

	if ok {
		leadTime, err = strconv.Atoi(leadTimeStr)
		if err != nil {
			s.Fatal(mtbferrors.New(mtbferrors.OSVarRead, err, "wifi.attLeadTime"))
		}
	} else {
		leadTime = 60
		s.Log("wifi.attLeadTime is not set. Use default value 60 seconds")
	}

	s.Log("getAttLeadTime - leadTime: ", leadTime)
	return leadTime
}

func getWifiStrInterval(ctx context.Context, s *testing.State) int {
	var interval int
	strInterval, ok := s.Var("wifi.strInterval")
	var err error

	if ok {
		interval, err = strconv.Atoi(strInterval)
		if err != nil {
			s.Fatal(mtbferrors.New(mtbferrors.OSVarRead, err, "wifi.strInterval"))
		}
	} else {
		interval = 10
		s.Log("wifi.strInterval is not set. Use default value 60 seconds")
	}

	s.Log("getWifiStrInterval - interval: ", interval)
	return interval
}

func getAttenuatorAPI(ctx context.Context, s *testing.State) *attenuator.RestAPI {
	attenuatorURL, ok := s.Var("wifi.attenuatorURL")

	if !ok {
		s.Log("wifi.attenuatorURL is not set! Will use Allion API")
		return nil
	}

	s.Log("attenuatorURL: " + attenuatorURL)
	return attenuator.NewRestAPI(ctx, attenuatorURL)
}

func getAttStrength(ctx context.Context, s *testing.State) []string {
	attStr, ok := s.Var("wifi.attenuateStrength")
	var wifiStrength []string

	if ok {
		s.Log("attStr: ", attStr)
		wifiStrength = strings.Split(attStr, ",")
	} else {
		wifiStrength = []string{"0", "10", "20", "30", "20", "10"}
		s.Log("attStr is not set. Use default value ")
	}

	s.Log("wifiStrength: ", wifiStrength)
	return wifiStrength
}

func changeWifiStrength(ctx context.Context, s *testing.State, attenuatorAPI *attenuator.RestAPI, allionAPI *allion.RestAPI, attnID string) {
	wifiStrength := getAttStrength(ctx, s)
	size := len(wifiStrength)
	i := 0
	leadTime := getAttLeadTime(ctx, s)
	interval := getWifiStrInterval(ctx, s)
	s.Logf("Will start changing WiFi strength in %v seconds, interval=%v seconds", leadTime, interval)
	common.Sleep(ctx, s, time.Duration(leadTime)*time.Second)
	s.Logf("Start changing WiFi strength size: %v, runnging: %v", size, running)
	var mtbferr error

	for running {
		strength := wifiStrength[i%size]
		s.Logf("Change WiFi strength. i=%v, strength=%v", i, strength)

		if attenuatorAPI != nil {
			s.Log("Use attenuator API to set WiFi strength")
			mtbferr = attenuatorAPI.SetStrength(strength)
		} else {
			s.Log("Use allion API to set WiFi strength")
			mtbferr = allionAPI.WifiStrManual(attnID, strength)
		}

		if mtbferr != nil {
			// Ignore this error and keep changing wifi strength
			// Not sure if calling s.Fatal in a go routine will stop the test case running.
			s.Log("Failed to change wifi strength: ", mtbferr)
		}

		i++
		common.Sleep(ctx, s, time.Duration(interval)*time.Second)
	}

	s.Log("Changing WiFi strength finished. i=", i)
}

func setWifiStrBack(attenuatorAPI *attenuator.RestAPI, allionAPI *allion.RestAPI, attnID string, s *testing.State) {
	s.Log("Set WiFi strength back")
	var mtbferr error

	if attenuatorAPI != nil {
		s.Log("Use attenuator API to set WiFi strength")
		mtbferr = attenuatorAPI.SetStrength("0") //TODO retry
	} else {
		s.Log("Use allion API to set WiFi strength")
		mtbferr = allionAPI.WifiStrManualWithRetry(attnID, "0", 3)
	}

	if mtbferr != nil {
		s.Error("Allion API failed: ", mtbferr)
	}
}

func stopChangingWifiStr(s *testing.State) {
	s.Log("Stop changing wifi strength")
	running = false
}
