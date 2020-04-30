// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package wifi

import (
	"context"
	"fmt"
	"time"

	"chromiumos/tast/common/allion"
	"chromiumos/tast/common/mtbferrors"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/input"
	mtbfchrome "chromiumos/tast/local/mtbf/chrome"
	"chromiumos/tast/local/mtbf/chrome/settings"
	"chromiumos/tast/testing"
)

const jsTimeout = 90 * time.Second

const osSettingPage = `document.querySelector("body > os-settings-ui")
	.shadowRoot.querySelector("#main")
	.shadowRoot.querySelector("os-settings-page")`

const internetPage = osSettingPage +
	`.shadowRoot.querySelector("#basicPage > settings-section.expanded > settings-internet-page")`

const wifiDom = osSettingPage +
	`.shadowRoot.querySelector("#basicPage > settings-section:nth-child(4) > settings-internet-page")
	.shadowRoot.querySelector("#pages > div > network-summary")
	.shadowRoot.querySelector("#WiFi")`

const wifiState = wifiDom + `.shadowRoot.querySelector("#networkState")`
const checkWifiJs = wifiState + `.innerText.trim()`
const clickWifiStateJs = wifiState + ".click()"

const wifiIconJs = wifiDom +
	`.shadowRoot.querySelector("#outerBox > div.flex.layout.horizontal.center > cr-icon-button")
	.shadowRoot.querySelector("#icon")`

const wifiKnownNetwork = internetPage +
	`.shadowRoot.querySelector("#pages > settings-subpage > settings-internet-subpage")
	 .shadowRoot.querySelector("cr-link-row").shadowRoot.querySelector("#label").innerText == 'Known networks'`

const enterWifiPageJs = wifiDom +
	`.shadowRoot.querySelector("#outerBox > div.flex.layout.horizontal.center > cr-icon-button")
	 .shadowRoot.querySelector("#icon").click()`

const wifiEnableElemJsOutside = wifiDom +
	`.shadowRoot.querySelector("#deviceEnabledButton")
	 .shadowRoot.querySelector("#bar")`

const wifiEnableElemJsInside = internetPage +
	`.shadowRoot.querySelector("#pages > settings-subpage > settings-internet-subpage")
	 .shadowRoot.querySelector("#deviceEnabledButton")
	 .shadowRoot.querySelector("#bar")`

const findWifiJs = `Array.from(` + internetPage + `
	.shadowRoot.querySelector("#pages > settings-subpage.iron-selected > settings-internet-subpage")
	.shadowRoot.querySelector("#networkList")
	.shadowRoot.querySelectorAll('#container > iron-list > cr-network-list-item'))
		.find(element => element.shadowRoot.querySelector('#divText > div')
		.textContent === '%v')`

const selectWifiJs = findWifiJs + `.shadowRoot.querySelector('#divText').click();`

const passwordInput = internetPage +
	`.shadowRoot.querySelector("#configDialog")
		.shadowRoot.querySelector("#networkConfig")
		.shadowRoot.querySelector("#wifi-passphrase")
		.shadowRoot.querySelector("#input")
		.shadowRoot.querySelector("#input")`

const clickPasswordJs = passwordInput + `.click()`

const enterPwdBtnJs = internetPage +
	`.shadowRoot.querySelector("#configDialog")
		.shadowRoot.querySelector("#dialog > div.layout.horizontal.center > cr-button.action-button")`

const backJs = internetPage +
	`.shadowRoot.querySelector("#pages > settings-subpage")
	.shadowRoot.querySelector("#closeButton")
	.shadowRoot.querySelector("#icon").click()`

const wifiListDiv = internetPage +
	`.shadowRoot.querySelector("#pages > settings-subpage.iron-selected > settings-internet-subpage")
	 .shadowRoot.querySelector("#networkListDiv").style`

const wifiListDisPlay = wifiListDiv + ".display"

// Conn represents a CDP connection to WiFi network setting page.
type Conn struct {
	ctx          context.Context
	cr           *chrome.Chrome
	wifiApName   string
	wifiApPwd    string
	wifiGUID     string
	ethernetGUID string
	cdpConn      *chrome.Conn
	allionSvrURL string
	deviceID     string
}

// Close closes the WiFi setting page.
func (c *Conn) Close() {
	testing.ContextLog(c.ctx, "WifiConn Close() called")
	defer c.cdpConn.Close()
	defer c.cdpConn.CloseTarget(c.ctx)
}

// NewConn creates a Conn object and open WiFi setting page.
func NewConn(ctx context.Context, cr *chrome.Chrome, enableWifi bool, apName string, wifiPwd string, allionSvrURL string, deviceID string) (*Conn, error) {
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

	if apName != "" {
		c.wifiGUID, err = c.getNicGUID(true)
		if err != nil {
			c.Close()
			return nil, err
		}
	}

	c.ethernetGUID, err = c.getNicGUID(false)
	if err != nil {
		c.Close()
		return nil, err
	}

	return c, nil
}

// ConnectToAp connects to the WiFi AP.
func (c *Conn) ConnectToAp() error {
	testing.ContextLog(c.ctx, "ConnectToAp() called")
	if wifiStatus, err := c.getWifiStatus(); err != nil {
		testing.ContextLog(c.ctx, "Failed to get WiFi status")
		return err
	} else if wifiStatus == "Connected" {
		testing.ContextLog(c.ctx, "Alaredy connected! Do nothing")
		return nil
	}

	testing.ContextLog(c.ctx, "The AP is not connected. Try to reconnect")
	if err := c.disconnectNic(c.wifiGUID); err != nil {
		testing.ContextLog(c.ctx, "Failed to disconnectNic: ", err)
		return err
	}

	if err := c.forgetNic(c.wifiGUID); err != nil {
		testing.ContextLog(c.ctx, "Failed to forgetNic: ", err)
		return err
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
	var wifiConnStatus string
	var err error
	var netOk bool
	ctx := c.ctx

	wifiConnStatus, err = c.pollWifiStatus()
	if err != nil {
		testing.ContextLog(ctx, "Failed to call pollWifiStatus(): ", err)
		return mtbferrors.New(mtbferrors.WIFIGetStat, err, c.wifiApName)
	} else if wifiConnStatus != "" {
		c.disconnectNic(c.wifiGUID)
		c.forgetNic(c.wifiGUID)
	}

	err = c.connectToWifiAp()
	if err != nil {
		testing.ContextLog(ctx, "Cannot connect wifi: ", err)
		return err
	}

	testing.ContextLog(ctx, "Ethernet GUID: ", c.ethernetGUID)
	allionAPI := allion.NewRestAPI(ctx, c.allionSvrURL)
	//c.DisconnectEthernet()
	allionAPI.DisableEthernet(c.deviceID)

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

// DisconnectEthernet disconnects from ethernet.
func (c *Conn) DisconnectEthernet() {
	testing.ContextLog(c.ctx, "Disconnect ethernet")
	//commented out for debugging
	//c.disconnectNic(c.ethernetGUID)
}

// ConnectEthernet connects to ethernent.
func (c *Conn) ConnectEthernet() {
	testing.ContextLog(c.ctx, "Connect ethernet")
	//commented out for debugging
	//c.connectNic(c.ethernetGUID)
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
	var wifiStatus string

	if err := testing.Poll(c.ctx, func(context.Context) error {
		var err error
		wifiStatus, err = c.getWifiStatus()

		if wifiStatus == "" {
			return errors.New("failed to get WiFi status in polling")
		} else if wifiStatus != "Connected" {
			return errors.New("wifiStatus is not connected")
		}

		return err
	}, &testing.PollOptions{Interval: 3 * time.Second, Timeout: jsTimeout}); err != nil {
		return false, mtbferrors.New(mtbferrors.WIFIGetStat, err, c.wifiApName)
	}

	testing.ContextLog(c.ctx, "myWifi.connectionState: "+wifiStatus)
	return (wifiStatus == "Connected"), nil
}

func (c *Conn) pollWifiStatus() (string, error) {
	var wifiStatus string

	if err := testing.Poll(c.ctx, func(context.Context) error {
		var err error
		wifiStatus, err = c.getWifiStatus()

		if wifiStatus == "" {
			return errors.New("failed to get WiFi status in polling")
		}

		return err
	}, &testing.PollOptions{Interval: 3 * time.Second, Timeout: jsTimeout}); err != nil {
		testing.ContextLogf(c.ctx, "pollWifiStatus() WiFi failed. ssid=%v err=%v", c.wifiApName, err)
		return "", err
	}

	testing.ContextLog(c.ctx, "pollWifiStatus() - myWifi.connectionState: "+wifiStatus)
	return wifiStatus, nil
}

func (c *Conn) getWifiStatus() (string, error) {
	js := `
		var myWifi;

		function isMyWifi(device) {
			return device.Name === "%v";
		}

		chrome.networkingPrivate.getVisibleNetworks("WiFi",
			function (networks) {
				myWifi = networks.find(isMyWifi);
			}
		)
	`
	conn := c.cdpConn
	ctx := c.ctx

	jsCleanUp := `myState = null;`
	defer conn.Exec(ctx, jsCleanUp)

	js = fmt.Sprintf(js, c.wifiApName)

	testing.ContextLog(ctx, "js: ", js)

	if err := conn.Exec(ctx, js); err != nil {
		testing.ContextLog(ctx, "Failed to get Wifi status by JS: ", err)
		return "", mtbferrors.New(mtbferrors.WIFIGetStat, err, c.wifiApName)
	}

	if err := conn.WaitForExprWithTimeout(ctx, "myWifi != null", 10*time.Second); err != nil {
		testing.ContextLog(ctx, "Failed to get myWifi status through cdp: ", err)
		return "", mtbferrors.New(mtbferrors.WIFIGetStat, err, c.wifiApName)
	}

	var wifiStatus string
	conn.Eval(ctx, "myWifi.ConnectionState", &wifiStatus)
	testing.ContextLog(ctx, "wifiStatus: ", wifiStatus)
	return wifiStatus, nil
}

// ForgetAllWiFiAP forget all WiFi settings.
func (c *Conn) ForgetAllWiFiAP() error {
	jsForgetAllWiFi := `
		var allForgot = false;

		function forgetWiFiAP(nic) {
			//console.log("Diconnect and forget: ", nic)
			chrome.networkingPrivate.startDisconnect(nic.GUID);
			chrome.networkingPrivate.forgetNetwork(nic.GUID);
		}

		chrome.networkingPrivate.getNetworks({"networkType":"WiFi", "configured":true},
			function (networks) {
				myNic = networks.forEach(forgetWiFiAP);
				allForgot = true;
			}
		)
	`

	testing.ContextLog(c.ctx, "jsForgetAllWiFi: ", jsForgetAllWiFi)

	if err := c.cdpConn.Exec(c.ctx, jsForgetAllWiFi); err != nil {
		testing.ContextLog(c.ctx, "Failed to call jsForgetAllWiFi: ", err)
		return mtbferrors.New(mtbferrors.WIFIForgetAll, err)
	}

	if err := c.cdpConn.WaitForExprWithTimeout(c.ctx, " allForgot = true ", jsTimeout); err != nil {
		return (mtbferrors.New(mtbferrors.WIFIForgetAll, err))
	}

	// Sleep 5 seconds to ensure all WiFi AP are fogotten
	testing.Sleep(c.ctx, 5*time.Second)
	return nil
}

func (c *Conn) getNicGUID(isWifi bool) (string, error) {
	jsFindNic := `
		var myNic;

		chrome.networkingPrivate.requestNetworkScan();

		function isMyNic(nic) {
			return nic.Name === "%v";
		}

		chrome.networkingPrivate.getNetworks({"networkType":"%v"},
			function (networks) {
				myNic = networks.find(isMyNic);
			}
		)
	`

	jsCleanUp := `myNic = null;`
	defer c.cdpConn.Exec(c.ctx, jsCleanUp)

	if isWifi {
		jsFindNic = fmt.Sprintf(jsFindNic, c.wifiApName, "WiFi")
	} else {
		jsFindNic = fmt.Sprintf(jsFindNic, "Ethernet", "Ethernet")
	}

	testing.ContextLog(c.ctx, "getNicGUID - jsFindNic: ", jsFindNic)

	if err := testing.Poll(c.ctx, func(context.Context) error {
		var err error

		if err := c.cdpConn.Exec(c.ctx, jsFindNic); err != nil {
			return errors.Wrap(err, "failed to find NIC")
		}

		if err := c.cdpConn.WaitForExprWithTimeout(c.ctx, "myNic != null", 10*time.Second); err != nil {
			return errors.Wrap(err, "failed to get myNic status")
		}

		return err
	}, &testing.PollOptions{Interval: 5 * time.Second, Timeout: jsTimeout}); err != nil {
		return "", mtbferrors.New(mtbferrors.WIFIGuid, err, isWifi, c.wifiApName)
	}

	var guid string
	c.cdpConn.Eval(c.ctx, "myNic.GUID", &guid)
	testing.ContextLog(c.ctx, "The NIC GUID: ", guid)

	return guid, nil
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

func (c *Conn) checkNetwork() (bool, error) {
	testURL := "https://www.google.com"

	testing.ContextLog(c.ctx, "Go to test URL: ", testURL)
	conn, err := mtbfchrome.NewConn(c.ctx, c.cr, testURL)
	if err != nil {
		testing.ContextLog(c.ctx, "Can't open chrome: ", err)
		return false, err
	}

	defer conn.Close()
	defer conn.CloseTarget(c.ctx)

	if err := conn.WaitForExprWithTimeout(c.ctx, "document.readyState === 'complete'", jsTimeout); err != nil {
		testing.ContextLog(c.ctx, "Testing URL dom document is not ready")
		return false, err
	}

	return true, nil
}

// CdpConn returns the cdp connection of WiFi setting page
func (c *Conn) CdpConn() *chrome.Conn {
	return c.cdpConn
}
