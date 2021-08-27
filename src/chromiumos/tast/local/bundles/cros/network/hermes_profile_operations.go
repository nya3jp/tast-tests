// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package network

import (
	"context"
	"encoding/json"
	"time"

	//"chromiumos/tast/common/hermesconst"
	"chromiumos/tast/common/mmconst"
	"chromiumos/tast/common/testexec"
	"chromiumos/tast/ctxutil"
	"chromiumos/tast/local/dbusutil"
	"chromiumos/tast/local/hermes"
	"chromiumos/tast/local/modemmanager"
	"chromiumos/tast/testing"

	"github.com/godbus/dbus"
)

func init() {
	testing.AddTest(&testing.Test{
		Func: HermesProfileOperations,
		Desc: "Verifies that basic Hermes operations succeed",
		Contacts: []string{
			"pholla@google.com",
			"chromeos-cellular-team@google.com",
		},
		Attr:    []string{"group:cellular", "cellular_unstable"},
		Timeout: 5 * time.Minute,
	})
}

func HermesProfileOperations(ctx context.Context, s *testing.State) {
	modem, err := modemmanager.NewModem(ctx)
	if err != nil {
		s.Fatal("Failed to create Modem: ", err)
	}
	props, err := modem.GetProperties(ctx)
	if err != nil {
		s.Fatal("Failed to call GetProperties on Modem: ", err)
	}
	device, err := props.GetString(mmconst.ModemPropertyDevice)
	if err != nil {
		s.Fatal("Missing Device property: ", err)
	}

	obj, err := dbusutil.NewDBusObject(ctx, modemmanager.DBusModemmanagerService, modemmanager.DBusModemmanagerInterface, modemmanager.DBusModemmanagerPath)
	if err != nil {
		s.Fatal("Unable to connect to ModemManager1: ", err)
	}
	if err = obj.Call(ctx, "InhibitDevice", device, true).Err; err != nil {
		s.Fatal("InhibitDevice(true) failed: ", err)
	}

	// TODO: get slot num smartly
	euicc, err := hermes.GetEuicc(ctx, 1)
	if err != nil {
		s.Fatal("Unable to get Hermes euicc: ", err)
	}

	if err := euicc.DBusObject.Call(ctx, "UseTestCerts", true).Err; err != nil {
		s.Fatal("Failed to use test certs: ", err)
	}

	// curl  --cacert '/usr/share/hermes-ca-certificates/test/gsma-ci.pem' -H 'Content-Type:application/json'  -X POST --data "{'gtsTestProfileList':[{'eid':'','confirmationCode':'12345','maxConfirmationCodeAttempts':0,'maxDownloadAttempts':5,'profileStatus':'RELEASED','profileClass':'OPERATIONAL','serviceProviderName':'CarrierConfirmationCode','generateSmdsEvent':true,'profilePolicyRules':[]}],'eid':''}" https://prod.smdp-plus.rsp.goog/gts/startGtsSession
	// data := " --data 'hello'"
	// cmd := testexec.CommandContext(ctx, "curl",  url)
	// certs := "--cacert '" + "/usr/share/hermes-ca-certificates/test/gsma-ci.pem" + "'"
	data := "{'gtsTestProfileList':[{'eid':'','confirmationCode':'','maxConfirmationCodeAttempts':1,'maxDownloadAttempts':5,'profileStatus':'RELEASED','profileClass':'OPERATIONAL','serviceProviderName':'CarrierConfirmationCode','generateSmdsEvent':true,'profilePolicyRules':[]}],'eid':''}"
	url := "https://prod.smdp-plus.rsp.goog/gts/startGtsSession"
	cmd := testexec.CommandContext(ctx, "curl",
		"--cacert", "/usr/share/hermes-ca-certificates/test/gsma-ci.pem",
		"-H", "Content-Type:application/json",
		"-X", "POST",
		"--data", data,
		url)
	s.Log(cmd.Cmd.String())
	out, err := cmd.Output()
	if err != nil {
		s.Error("Curl command to create profile failed: ", err)
	}
	s.Log("curl output:", string(out))

	var objmap map[string]json.RawMessage
	if err := json.Unmarshal(out, &objmap); err != nil {
		s.Error("Curl command to create profile failed: ", err)
	}

	var sessionId string 
	if err = json.Unmarshal(objmap["sessionId"], &sessionId); err != nil {
		s.Error("Could not unmarshal sessionId: ", err)
	}
	// curl  --cacert './gsma-ci-test.pem'  "https://prod.smdp-plus.rsp.goog/gts/endGtsSession?sessionId=dbofniegfcbrbbkqkssdfadfadf"
	
	cleanupTime := 5 * time.Second
	defer func(ctx context.Context) {
	url = "https://prod.smdp-plus.rsp.goog/gts/endGtsSession?sessionId=" + sessionId
	cmd = testexec.CommandContext(ctx, "curl",
		"--cacert", "/usr/share/hermes-ca-certificates/test/gsma-ci.pem",
		url)
		out, _ := cmd.CombinedOutput()
		s.Log("cleanup: ", string(out))
	}(ctx)
	ctx, cancel := ctxutil.Shorten(ctx, cleanupTime)
	defer cancel()


	var gtsTestProfileList []json.RawMessage
	if err = json.Unmarshal(objmap["gtsTestProfileList"], &gtsTestProfileList); err != nil {
		s.Fatal("Could not unmarshal gtsTestProfileList: ", err)
	}
	var profileInfo map[string]interface{}
	if err = json.Unmarshal(gtsTestProfileList[0], &profileInfo); err != nil {
		s.Fatal("Could not unmarshal into profileInfo: ", err)
	}

	s.Log("activation code: ", "1$prod.smdp-plus.rsp.goog$", profileInfo["matchingId"].(string))
	// TODO: Convert strings to constants
	profilePath := dbus.ObjectPath("")
	res := euicc.DBusObject.Call(ctx, "InstallProfileFromActivationCode", "1$prod.smdp-plus.rsp.goog$" + profileInfo["matchingId"].(string),"")
	if res.Err != nil {
		s.Fatal("Failed to install profile: ", err)
	}
	if err := res.Store(&profilePath); err != nil {
		s.Fatal("Failed to install profile: ", err)
	}
	s.Logf("Installed %s", profilePath)

	if err := euicc.DBusObject.Call(ctx, "ResetMemory", 1).Err; err != nil {
		s.Fatal("Failed to reset memory: ", err)
	}
}
