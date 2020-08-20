// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package wifi

import (
	"context"
	"fmt"
	"time"

	"chromiumos/tast/common/allion"
	"chromiumos/tast/common/httputil"
	"chromiumos/tast/common/mtbferrors"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/input"
	mtbfchrome "chromiumos/tast/local/mtbf/chrome"
	"chromiumos/tast/local/mtbf/chrome/settings"
	"chromiumos/tast/testing"
)

const (
	jsTimeout = 90 * time.Second

	osSettingPage = `document.querySelector("body > os-settings-ui")
		.shadowRoot.querySelector("#main")
		.shadowRoot.querySelector("os-settings-page")`

	internetPage = osSettingPage +
		`.shadowRoot.querySelector("#basicPage > settings-section.expanded > settings-internet-page")`

	wifiDom = osSettingPage +
		`.shadowRoot.querySelector("#basicPage > settings-section:nth-child(4) > settings-internet-page")
		.shadowRoot.querySelector("#pages > div > network-summary")
		.shadowRoot.querySelector("#WiFi")`

	wifiState        = wifiDom + `.shadowRoot.querySelector("#networkState")`
	checkWifiJs      = wifiState + `.innerText.trim()`
	clickWifiStateJs = wifiState + ".click()"

	wifiIconJs = wifiDom +
		`.shadowRoot.querySelector("div > div.flex.layout.horizontal.center.link-wrapper > cr-icon-button")
		.shadowRoot.querySelector("#maskedImage")`

	wifiKnownNetwork = internetPage +
		`.shadowRoot.querySelector("#pages > settings-subpage > settings-internet-subpage")
		.shadowRoot.querySelector("cr-link-row").shadowRoot.querySelector("#label").innerText == 'Known networks'`

	enterWifiPageJs = wifiIconJs + `.click()`

	wifiEnableElemJsOutside = wifiDom +
		`.shadowRoot.querySelector("#deviceEnabledButton")
		.shadowRoot.querySelector("#bar")`

	wifiEnableElemJsInside = internetPage +
		`.shadowRoot.querySelector("#pages > settings-subpage > settings-internet-subpage")
		.shadowRoot.querySelector("#deviceEnabledButton")
		.shadowRoot.querySelector("#bar")`

	findWifiJs = `Array.from(` + internetPage + `
		.shadowRoot.querySelector("#pages > settings-subpage.iron-selected > settings-internet-subpage")
		.shadowRoot.querySelector("#networkList")
		.shadowRoot.querySelector("#networkList")
		.querySelectorAll("network-list-item"))
		.find(item => item.shadowRoot.querySelector('#divText').innerText === '%v')`

	selectWifiJs = findWifiJs + `.shadowRoot.querySelector('#divText').click();`

	passwordInput = internetPage +
		`.shadowRoot.querySelector("#configDialog")
			.shadowRoot.querySelector("#networkConfig")
			.shadowRoot.querySelector("#wifi-passphrase")
			.shadowRoot.querySelector("#input")
			.shadowRoot.querySelector("#input")`

	clickPasswordJs = passwordInput + `.click()`

	enterPwdBtnJs = internetPage +
		`.shadowRoot.querySelector("#configDialog")
			.shadowRoot.querySelector("#dialog > div.layout.horizontal.center > cr-button.action-button")`

	backJs = internetPage +
		`.shadowRoot.querySelector("#pages > settings-subpage")
		.shadowRoot.querySelector("#closeButton")
		.shadowRoot.querySelector("#icon").click()`

	wifiListDiv = internetPage +
		`.shadowRoot.querySelector("#pages > settings-subpage.iron-selected > settings-internet-subpage")
		.shadowRoot.querySelector("#networkListDiv").style`

	wifiListDisPlay = wifiListDiv + ".display"

	wifiStatusJs = `
		new Promise(function(resolve, reject) {
			chrome.networkingPrivate.getNetworks({"networkType":"WiFi"},
				function (networks) {
					var myWifi = networks.find(device => device.Name === "%v");
					if (myWifi != null) {
						resolve(myWifi.ConnectionState)
					} else {
						reject(false)
					}
				}
			);
		})`

	wifiInfoJs = `new Promise(function(resolve, reject) {
			chrome.networkingPrivate.getNetworks({"networkType":"WiFi"},
				function (networks) {
					var myWiFi = networks.find(device => device.Name === "%v");
					if (myWiFi != null) {
						resolve("name: " + myWiFi.Name + ", ConnectionState: "
							+ myWiFi.ConnectionState + ", Frequency: "
							+ myWiFi.WiFi.Frequency + ", SignalStrength: "
							+ myWiFi.WiFi.SignalStrength)
					} else {
						reject(false)
					}
				}
			);
		})`

	forgetAllJs = `
		var allAps = new Map();

		function putToMap(wifiAp) {
			// console.log("Put to map: " + wifiAp.Name)
			allAps.set(wifiAp.Name, wifiAp)
			// console.log("map size: " + allAps.size)
		}

		function removeFromMap(wifiAp, resolve, reject) {
			// console.log("Remove from map: " + wifiAp.Name)
			allAps.delete(wifiAp.Name)
			// console.log("map size: " + allAps.size)

			if (allAps.size == 0) {
				resolve(true)
			}
		}

		function forgetDisconnectWiFiAP(wifiAp, resolve, reject) {
			// console.log("Diconnect and forget: ", wifiAp.Name)
			
			if (wifiAp.ConnectionState === "Connected") {
				// console.log(wifiAp.Name + " is connected. Will disconnect it ")
				chrome.networkingPrivate.startDisconnect(wifiAp.GUID, doForgetWiFiAp(wifiAp, resolve, reject));
			} else {
				doForgetWiFiAp(wifiAp, resolve, reject)
			}
		}

		function doForgetWiFiAp(wifiAp, resolve, reject) {
			// console.log("doForgetWiFiAp: " + wifiAp.Name)
			chrome.networkingPrivate.forgetNetwork(wifiAp.GUID, removeFromMap(wifiAp, resolve, reject));
		}
		
		new Promise(function(resolve, reject) {
			chrome.networkingPrivate.getNetworks({"networkType":"WiFi", "configured":true},
				function (networks) {
					// console.log("networks: ", networks);

					if (networks.length == 0) {
						resolve(true);
						return;
					}

					networks.forEach(putToMap);
					// console.log("allAps: ", allAps)
					
					for (i = 0; i < networks.length; i++) {
						forgetDisconnectWiFiAP(networks[i], resolve, reject);
					}
				}
			)
		})
	`
)

// Conn represents a CDP connection to WiFi network setting page.
type Conn struct {
	ctx          context.Context
	cr           *chrome.Chrome
	wifiApName   string
	wifiApPwd    string
	cdpConn      *chrome.Conn
	allionSvrURL string
	deviceID     string
}

// Close closes the WiFi setting page.
func (c *Conn) Close(cleanUpSettings bool) {
	testing.ContextLog(c.ctx, "WifiConn Close() called")

	if cleanUpSettings {
		c.CleanUpWifiSettings()
	}

	defer c.cdpConn.Close()
	defer c.cdpConn.CloseTarget(c.ctx)
}

// NewConn creates a Conn object and open WiFi setting page.
func NewConn(ctx context.Context, cr *chrome.Chrome, enableWifi bool, apName, wifiPwd, allionSvrURL, deviceID string) (*Conn, error) {
	c := &Conn{
		ctx:          ctx,
		cr:           cr,
		wifiApName:   apName,
		wifiApPwd:    wifiPwd,
		allionSvrURL: allionSvrURL,
		deviceID:     deviceID,
	}
	var err error

	testing.ContextLogf(c.ctx, "Initialize Conn. enableWifi=%v, apName=%v", apName, enableWifi)
	c.cdpConn, err = settings.OpenOsSettingsPage(ctx, cr)

	if err != nil {
		return nil, err
	}

	if enableWifi {
		//Check if WiFi is enabled before clicking on it
		if err = checkWifiEnabled(ctx, c.cdpConn); err != nil {
			return nil, err
		}
	}

	return c, nil
}

// ConnectToAp connects to the WiFi AP.
func (c *Conn) ConnectToAp() error {
	testing.ContextLog(c.ctx, "ConnectToAp() called")
	if wifiStatus, err := c.getWifiStatusWithRetry(3); err != nil {
		testing.ContextLog(c.ctx, "Failed to get WiFi status")
		return err
	} else if wifiStatus == "Connected" {
		testing.ContextLog(c.ctx, "Alaredy connected! Do nothing")
		return nil
	}

	if err := c.connectToWifiAp(); err != nil {
		testing.ContextLog(c.ctx, "Failed to connectToWifiAp: ", err)
		return err
	}

	return nil
}

func (c *Conn) connectToWifiAp() error {
	var connected bool
	ctx := c.ctx
	conn := c.cdpConn
	testing.ContextLog(ctx, "Click wifi icon. js: ", wifiIconJs)

	if err := conn.WaitForExprWithTimeout(c.ctx, wifiIconJs+" != null ", jsTimeout); err != nil {
		return (mtbferrors.New(mtbferrors.WIFIFatal, err, c.wifiApName))
	}

	if err := conn.Exec(c.ctx, wifiIconJs+".click()"); err != nil {
		return (mtbferrors.New(mtbferrors.WIFIFatal, err, c.wifiApName))
	}

	testing.ContextLog(ctx, "Wifi setting clicked. Wait for text ", wifiKnownNetwork)

	if err := conn.WaitForExprWithTimeout(c.ctx, wifiKnownNetwork, jsTimeout); err != nil {
		return (mtbferrors.New(mtbferrors.WIFIFatal, err, c.wifiApName))
	}

	// Sleep here to avoid nothing happened after clicking wifi AP
	testing.Sleep(c.ctx, 3*time.Second)
	js := fmt.Sprintf(findWifiJs, c.wifiApName)
	testing.ContextLog(ctx, "Finding wifi js: ", js)

	if err := conn.WaitForExprWithTimeout(c.ctx, js+" != null", jsTimeout); err != nil {
		return (mtbferrors.New(mtbferrors.WIFIFatal, err, c.wifiApName))
	}

	testing.Sleep(c.ctx, 3*time.Second)
	js = fmt.Sprintf(selectWifiJs, c.wifiApName)
	testing.ContextLog(ctx, "Select the wifi AP. js: ", js)

	if err := conn.Exec(c.ctx, js); err != nil {
		return (mtbferrors.New(mtbferrors.WIFIFatal, err, c.wifiApName))
	}

	testing.ContextLog(ctx, "select wifi")

	if err := conn.WaitForExprWithTimeout(c.ctx, passwordInput+" != null", jsTimeout); err != nil {
		return (mtbferrors.New(mtbferrors.WIFIPasswd, err, c.wifiApName))
	}

	testing.Sleep(c.ctx, 3*time.Second)

	testing.ContextLog(ctx, "clickPasswordJs: ", clickPasswordJs)

	if err := conn.Exec(c.ctx, clickPasswordJs); err != nil {
		return (mtbferrors.New(mtbferrors.WIFIPasswd, err, c.wifiApName))
	}

	testing.Sleep(c.ctx, 3*time.Second)

	// Setup keyboard.
	kb, err := input.Keyboard(c.ctx)
	if err != nil {
		return (mtbferrors.New(mtbferrors.WIFIPasswd, err, c.wifiApName))
	}

	defer kb.Close()

	// Open QuickView for the test image and check dimensions.
	if err := kb.Type(c.ctx, c.wifiApPwd); err != nil {
		return (mtbferrors.New(mtbferrors.WIFIPasswd, err, c.wifiApName))
	}

	testing.ContextLog(ctx, "Enter password")

	if err := conn.WaitForExprWithTimeout(c.ctx, enterPwdBtnJs+".disabled === false", jsTimeout); err != nil {
		return (mtbferrors.New(mtbferrors.WIFIPasswd, err, c.wifiApName))
	}

	if err := conn.Exec(c.ctx, enterPwdBtnJs+".click()"); err != nil {
		return (mtbferrors.New(mtbferrors.WIFIPasswd, err, c.wifiApName))
	}

	testing.ContextLog(ctx, "Click connect button")
	connected, err = c.checkWifiConnected()

	if !connected {
		// Retry once after 3 seconds
		testing.Sleep(c.ctx, 3*time.Second)
		connected, err = c.checkWifiConnected()

		if err != nil {
			testing.ContextLog(ctx, "Failed to call checkWifiConnected(): ", err)
			return err
		}
	}

	testing.ContextLog(ctx, "WiFi connected: ", connected)

	if !connected {
		testing.ContextLog(ctx, "WifFi is not connected")
		return (mtbferrors.New(mtbferrors.WIFIFatal, err, c.wifiApName))
	}

	// Go back to network setting page for debugging
	if err := conn.Exec(c.ctx, backJs); err != nil {
		testing.ContextLog(ctx, "Can't go back to previous setting page")
		return (mtbferrors.New(mtbferrors.WIFIFatal, err, c.wifiApName))
	}

	return nil
}

// TestConnected tests if the WiFi AP can be connected and the internet can be accessed.
func (c *Conn) TestConnected() error {
	var err error
	var netOk bool
	ctx := c.ctx

	err = c.ForgetAllWiFiAP()
	if err != nil {
		return err
	}

	err = c.connectToWifiAp()
	if err != nil {
		testing.ContextLog(ctx, "Cannot connect wifi: ", err)
		return err
	}

	allionAPI := allion.NewRestAPI(ctx, c.allionSvrURL)

	if err := allionAPI.DisableEthernet(c.deviceID); err != nil {
		testing.ContextLog(ctx, "Error occured while disabling ethernet: ", err)
		// It might be caused ethernet disconnected, so ignore it.
	}

	testing.Sleep(ctx, 2000*time.Millisecond)

	// defer() is not called if s.Fatal() is called. Why??
	defer allionAPI.EnableEthernetWithRetry(c.deviceID, 3)
	netOk, err = c.checkNetwork()

	if err != nil {
		testing.ContextLog(ctx, "Failed to call checkNetwork(): ", err)
		return err
	}

	if !netOk {
		testing.ContextLog(ctx, "Internet can't be accessed")
		return mtbferrors.New(mtbferrors.WIFIInternet, err, c.wifiApName)
	}

	return nil
}

func getWifiSettingStatus(ctx context.Context, conn *chrome.Conn) (string, error) {
	var wifiStatus string
	testing.ContextLog(ctx, "checkWifiJs: ", checkWifiJs)

	if err := conn.WaitForExprWithTimeout(ctx, checkWifiJs+" != null", jsTimeout); err != nil {
		return "", mtbferrors.New(mtbferrors.WIFISetting, err)
	}

	if err := conn.Eval(ctx, checkWifiJs, &wifiStatus); err != nil {
		return "", mtbferrors.New(mtbferrors.WIFISetting, err)
	}

	testing.ContextLog(ctx, "Wifi Status: ", wifiStatus)
	return wifiStatus, nil
}

func checkWifiEnabled(ctx context.Context, conn *chrome.Conn) error {
	testing.ContextLog(ctx, "Sleep 2.5 Sec before sending command")
	testing.Sleep(ctx, 2500*time.Millisecond)
	wifiStatus, err := getWifiSettingStatus(ctx, conn)

	if err != nil {
		testing.ContextLog(ctx, "Failed to call getWifiSettingStatus(): ", err)
		return err
	}

	if wifiStatus == "Off" {
		testing.ContextLog(ctx, "Wifi is not enabled. Try to enable it. js: ", clickWifiStateJs)

		if err := conn.Exec(ctx, clickWifiStateJs); err != nil {
			testing.ContextLog(ctx, "Failed to clickWifiStateJs(): ", err)
			return mtbferrors.New(mtbferrors.WIFISetting, err)
		}

		wifiStatus, err = pollWifiStatus(ctx, conn)

		if err != nil {
			testing.ContextLog(ctx, "Failed to pollWifiStatus(): ", err)
			return mtbferrors.New(mtbferrors.WIFISetting, err)
		}

		//WiFi should be enabled after polling
		testing.ContextLog(ctx, "Wifi Status after enabled: ", wifiStatus)
	}

	return nil
}

// CheckWifi check if WiFi is enabled or disabled.
func (c *Conn) CheckWifi(shouldEnabled bool) (string, error) {
	if shouldEnabled {
		return pollWifiStatus(c.ctx, c.cdpConn)
	}
	return pollWifiForOffStatus(c.ctx, c.cdpConn)
}

func pollWifiStatus(ctx context.Context, conn *chrome.Conn) (string, error) {
	var wifiStatus string

	if err := testing.Poll(ctx, func(context.Context) error {
		var err error
		wifiStatus, err = getWifiSettingStatus(ctx, conn)

		if err != nil {
			return err
		} else if wifiStatus == "" || wifiStatus == "Enabling" || wifiStatus == "Off" || wifiStatus == "No network" {
			return errors.New("failed to get WiFi status in polling")
		}

		return nil
	}, &testing.PollOptions{Interval: 3 * time.Second, Timeout: jsTimeout}); err != nil {
		return "", mtbferrors.New(mtbferrors.WIFIEnable, err, wifiStatus)
	}

	return wifiStatus, nil
}

func pollWifiForOffStatus(ctx context.Context, conn *chrome.Conn) (string, error) {
	var wifiStatus string

	if err := testing.Poll(ctx, func(context.Context) error {
		var err error
		wifiStatus, err = getWifiSettingStatus(ctx, conn)

		if err != nil {
			return err
		} else if wifiStatus != "Off" {
			return errors.New("failed to get WiFi Off status")
		}

		return nil
	}, &testing.PollOptions{Interval: 3 * time.Second, Timeout: jsTimeout}); err != nil {
		return "", mtbferrors.New(mtbferrors.WIFIDisable, err, wifiStatus)
	}

	return wifiStatus, nil
}

func (c *Conn) clickWifi() error {
	testing.ContextLog(c.ctx, "clickWifi() - wifiEnableElemJsOutside: ", wifiEnableElemJsOutside)
	// c.cdpConn.Eval(c.ctx, wifiEnableElemJsOutside+" != null ", &outside)
	err := c.cdpConn.WaitForExprWithTimeout(c.ctx, wifiEnableElemJsOutside+" != null ", 1*time.Second)
	testing.ContextLog(c.ctx, "outside err: ", err)
	outside := (err == nil)
	testing.ContextLog(c.ctx, "outside: ", outside)
	var wifiEnableJs string

	if outside {
		wifiEnableJs = wifiEnableElemJsOutside + ".click()"
	} else {
		if err := c.cdpConn.WaitForExprWithTimeout(c.ctx, wifiEnableElemJsInside+" != null", jsTimeout); err != nil {
			return mtbferrors.New(mtbferrors.WIFIFatal, err, c.wifiApName)
		}
		wifiEnableJs = wifiEnableElemJsInside + ".click()"
	}

	testing.ContextLog(c.ctx, "wifiEnableJs: ", wifiEnableJs)

	if err := c.cdpConn.Exec(c.ctx, wifiEnableJs); err != nil {
		testing.ContextLog(c.ctx, "Exec error: ", mtbferrors.New(mtbferrors.WIFIFatal, err, c.wifiApName))
		return err
	}

	return nil
}

// EnterWifiPage enters WiFi setting page.
func (c *Conn) EnterWifiPage() error {
	testing.ContextLog(c.ctx, "EnterWifiPage(). enterWifiPageJs: ", enterWifiPageJs)
	testing.Sleep(c.ctx, 3*time.Second)

	if err := c.cdpConn.Exec(c.ctx, enterWifiPageJs); err != nil {
		testing.ContextLog(c.ctx, "Exec error: ", err)
		return mtbferrors.New(mtbferrors.WIFIFatal, err, c.wifiApName)
	}

	return nil
}

// LeaveWifiPage leaves WiFi setting page.
func (c *Conn) LeaveWifiPage() error {
	testing.ContextLog(c.ctx, "LeaveWifiPage(). backJs: ", backJs)

	if err := c.cdpConn.Exec(c.ctx, backJs); err != nil {
		return mtbferrors.New(mtbferrors.WIFIFatal, err, c.wifiApName)
	}

	return nil
}

// DisableWifi disables WiFi network.
func (c *Conn) DisableWifi() (string, error) {
	testing.ContextLog(c.ctx, "DisableWifi() entered")
	wifiStatus, err := getWifiSettingStatus(c.ctx, c.cdpConn)

	if wifiStatus == "Off" {
		testing.ContextLog(c.ctx, "WiFi status is Off. Do noting")
		return wifiStatus, nil
	}

	if err := c.clickWifi(); err != nil {
		return "", err
	}

	wifiStatus, err = pollWifiForOffStatus(c.ctx, c.cdpConn)

	if err != nil {
		return "", err
	}

	testing.ContextLog(c.ctx, "Wifi Status after disabled: ", wifiStatus)
	return wifiStatus, nil
}

// EnableWifi enables WiFi network.
func (c *Conn) EnableWifi() (string, error) {
	testing.ContextLog(c.ctx, "EnableWifi() entered")
	wifiStatus, err := getWifiSettingStatus(c.ctx, c.cdpConn)

	if wifiStatus != "Off" {
		testing.ContextLog(c.ctx, "WiFi status is not Off do noting")
		return wifiStatus, nil
	}

	if err := c.clickWifi(); err != nil {
		return "", err
	}

	wifiStatus, err = pollWifiStatus(c.ctx, c.cdpConn)

	if err != nil {
		return "", err
	}

	testing.ContextLog(c.ctx, "Wifi Status after enabled: ", wifiStatus)
	return wifiStatus, nil
}

// CheckWifiListDisplayed checks if WiFi AP list is displayed
func (c *Conn) CheckWifiListDisplayed() (bool, error) {
	testing.ContextLog(c.ctx, "CheckWifiListDisplayed - wait for wifiListDiv: ", wifiListDiv)

	if err := c.cdpConn.WaitForExprWithTimeout(c.ctx, wifiListDiv+" != null", jsTimeout); err != nil {
		testing.ContextLog(c.ctx, "Failed to get Wifi List through cdp: ", err)
		return false, mtbferrors.New(mtbferrors.WIFIAPlist, err)
	}

	testing.ContextLog(c.ctx, "CheckWifiListDisplayed - evaluate wifiListDisPlay: ", wifiListDisPlay)

	var display string
	if err := c.cdpConn.Eval(c.ctx, wifiListDisPlay, &display); err != nil {
		testing.ContextLog(c.ctx, "Failed to get WiFi List display: ", err)
		return false, mtbferrors.New(mtbferrors.WIFIAPlist, err)
	}

	testing.ContextLog(c.ctx, "WiFi list div display: ", display)
	return display != "none", nil
}

func (c *Conn) checkWifiConnected() (bool, error) {
	testing.ContextLog(c.ctx, "performing checkWifiConnected")
	var wifiStatus string
	retry := 0

	if err := testing.Poll(c.ctx, func(context.Context) error {
		var err error
		wifiStatus, err = c.getWifiStatus()

		if wifiStatus == "" {
			testing.ContextLogf(c.ctx, "Failed to get WiFi status in checkWifiConnected retry=%v err=%v", retry, err)
			return errors.New("failed to get WiFi status in polling")
		} else if wifiStatus != "Connected" {
			testing.ContextLogf(c.ctx, "Still not connected! wifiStatus=%v retry=%v", wifiStatus, retry)
			retry++
			return errors.New("wifiStatus is not connected")
		}

		return err
	}, &testing.PollOptions{Interval: 3 * time.Second, Timeout: 60 * time.Second}); err != nil {
		return false, mtbferrors.New(mtbferrors.WIFIGetStat, err, c.wifiApName)
	}

	testing.ContextLog(c.ctx, "myWifi.connectionState: "+wifiStatus)
	return (wifiStatus == "Connected"), nil
}

func (c *Conn) pollWifiStatus() (string, error) {
	testing.ContextLog(c.ctx, "performing pollWifiStatus")
	var wifiStatus string
	retry := 0

	if err := testing.Poll(c.ctx, func(context.Context) error {
		var err error
		wifiStatus, err = c.getWifiStatus()

		if wifiStatus == "" {
			testing.ContextLogf(c.ctx, "Failed to get WiFi status in pollWifiStatus retry=%v err=%v", retry, err)
			retry++
			return errors.New("failed to get WiFi status in polling")
		}

		return err
	}, &testing.PollOptions{Interval: 3 * time.Second, Timeout: 60 * time.Second}); err != nil {
		testing.ContextLogf(c.ctx, "pollWifiStatus() WiFi failed. ssid=%v err=%v", c.wifiApName, err)
		return "", err
	}

	testing.ContextLog(c.ctx, "pollWifiStatus() - myWifi.connectionState: "+wifiStatus)
	return wifiStatus, nil
}

func (c *Conn) getWifiStatusWithRetry(retryCnt int) (string, error) {
	testing.ContextLog(c.ctx, "getWifiStatusWithRetry - retryCnt: ", retryCnt)
	retry := 0
	var err error
	var status string

	for retry < retryCnt {
		status, err = c.getWifiStatus()

		if err == nil {
			testing.ContextLog(c.ctx, "Got WiFi status: ", status)
			return status, nil
		}

		testing.ContextLog(c.ctx, "Failed to WiFi status: ", err)
		testing.ContextLog(c.ctx, "retry: ", retry)
		retry++
	}

	return "", err
}

func (c *Conn) getWifiStatus() (string, error) {
	ctx := c.ctx
	js := fmt.Sprintf(wifiStatusJs, c.wifiApName)
	testing.ContextLog(ctx, "get statues by promise js: ", js)
	var wifiStatus string

	if err := c.cdpConn.EvalPromise(c.ctx, js, &wifiStatus); err != nil {
		return "", mtbferrors.New(mtbferrors.WIFIGetStat, err, c.wifiApName)
	}

	testing.ContextLog(ctx, "wifiStatus: ", wifiStatus)
	return wifiStatus, nil
}

// ForgetAllWiFiAP forget all WiFi settings.
func (c *Conn) ForgetAllWiFiAP() error {
	var result bool
	testing.ContextLog(c.ctx, "forgetAllJs: ", forgetAllJs)

	if err := c.cdpConn.EvalPromise(c.ctx, forgetAllJs, &result); err != nil {
		testing.ContextLog(c.ctx, "Failed to call forgetAllJs: ", err)
		return mtbferrors.New(mtbferrors.WIFIForgetAll, err)
	}

	// Sleep 5 seconds to ensure all WiFi AP are forgotten
	testing.Sleep(c.ctx, 5*time.Second)
	return nil
}

func (c *Conn) disconnectNic(guid string) error {
	jsDisconnect := `chrome.networkingPrivate.startDisconnect("%v")`
	jsDisconnect = fmt.Sprintf(jsDisconnect, guid)

	testing.ContextLog(c.ctx, "jsDisconnect: ", jsDisconnect)

	if err := c.cdpConn.Exec(c.ctx, jsDisconnect); err != nil {
		testing.ContextLog(c.ctx, "Failed to call jsDisconnect: ", err)
		return mtbferrors.New(mtbferrors.WIFIFatal, err, c.wifiApName)
	}

	return nil
}

func (c *Conn) forgetNic(guid string) error {
	jsForget := `chrome.networkingPrivate.forgetNetwork("%v")`
	jsForget = fmt.Sprintf(jsForget, guid)
	testing.ContextLog(c.ctx, "jsForget: ", jsForget)

	if err := c.cdpConn.Exec(c.ctx, jsForget); err != nil {
		testing.ContextLog(c.ctx, "Failed to call jsForget: ", err)
		return mtbferrors.New(mtbferrors.WIFIFatal, err, c.wifiApName)
	}

	return nil
}

func (c *Conn) disableWifiByJs() error {
	disableWiFiJs := `chrome.networkingPrivate.disableNetworkType("WiFi")`
	testing.ContextLog(c.ctx, "disable WiFi disableWiFiJs: ", disableWiFiJs)

	if err := c.cdpConn.Exec(c.ctx, disableWiFiJs); err != nil {
		testing.ContextLog(c.ctx, "Failed to disable WiFi: ", err)
		return (mtbferrors.New(mtbferrors.WFIFDisable, err))
	}

	return nil
}

// CleanUpWifiSettings forget all WiFi APs and disable WiFi in settings
func (c *Conn) CleanUpWifiSettings() {
	if err := c.ForgetAllWiFiAP(); err != nil {
		testing.ContextLog(c.ctx, "Failed to call ForgetAllWiFiAP: ", err)
	}
	if err := c.disableWifiByJs(); err != nil {
		testing.ContextLog(c.ctx, "Failed to call disableWifiByJs: ", err)
	}
}

// GetWifiStrInfo return the strength information of this WiFi AP
func (c *Conn) GetWifiStrInfo() (string, error) {
	js := fmt.Sprintf(wifiInfoJs, c.wifiApName)
	var wifiInfo string

	if err := c.cdpConn.EvalPromise(c.ctx, js, &wifiInfo); err != nil {
		testing.ContextLogf(c.ctx, "Failed to get WiFi into js: %v, err: %v", js, err)
		return "", mtbferrors.New(mtbferrors.WIFIGetStrInfo, err, c.wifiApName)
	}

	return wifiInfo, nil
}

func (c *Conn) checkNetwork() (bool, error) {
	testURL := "http://www.google.com"
	testing.ContextLog(c.ctx, "Test the URL: ", testURL)
	var err error
	var body string
	var statusCode int
	retry := 0

	for retry < 3 {
		body, statusCode, err = httputil.HTTPGetStr(testURL, 10*time.Second)
		testing.ContextLogf(c.ctx, "statusCode=%v, body=%v", statusCode, body)

		if err == nil && statusCode == 200 {
			testing.ContextLogf(c.ctx, "The URL %v is loaded successfully", testURL)
			break
		}

		testing.Sleep(c.ctx, 2*time.Second)
		testing.ContextLogf(c.ctx, "Failed to load the URL %v. err=%v, retry=%v", testURL, err, retry)
		retry++
	}

	if err != nil || statusCode != 200 {
		return false, mtbferrors.New(mtbferrors.WIFIURLAccess, err, testURL, statusCode)
	}

	testing.ContextLog(c.ctx, "Go to test URL: ", testURL)
	conn, err := mtbfchrome.NewConn(c.ctx, c.cr, testURL)
	if err != nil {
		testing.ContextLog(c.ctx, "Can't open chrome: ", err)
		return false, err
	}

	defer conn.Close()
	defer conn.CloseTarget(c.ctx)

	if err := conn.WaitForExprWithTimeout(c.ctx, "document.readyState === 'complete'", 180*time.Second); err != nil {
		testing.ContextLog(c.ctx, "Testing URL dom document is not ready")
		return false, err
	}

	return true, nil
}

// CdpConn returns the cdp connection of WiFi setting page
func (c *Conn) CdpConn() *chrome.Conn {
	return c.cdpConn
}
