// Copyright 2018 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package arc

import (
	"context"
	"strings"
	"time"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/arc"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         SettingsBridge,
		Desc:         "Checks that Chrome settings are persisted in ARC",
		Attr:         []string{"informational"},
		SoftwareDeps: []string{"android", "chrome_login"},
		Timeout:      4 * time.Minute,
	})
}

// enableAccessibility enables spoken feedback on Chrome.
func enableAccessibility(ctx context.Context, conn *chrome.Conn) error {
	err := conn.Exec(ctx, `
	window.__spoken_feedback_set_complete = false;
	chrome.accessibilityFeatures.spokenFeedback.set({value: true});
	chrome.accessibilityFeatures.spokenFeedback.get({}, () => {
		window.__spoken_feedback_set_complete = true;
	})`)
	if err != nil {
		return err
	}
	return nil
}

// isAccessibilityEnabled checks whether accessibility is enabled on Android.
func isAccessibilityEnabled(ctx context.Context, a *arc.ARC) (bool, error) {
	cmd := a.Command(ctx, "settings", "--user", "0", "get", "secure", "accessibility_enabled")
	res, err := cmd.Output()
	if err != nil {
		cmd.DumpLog(ctx)
		return false, err
	}
	if strings.TrimSpace(string(res)) == "1" {
		return true, nil
	}
	return false, nil
}

// isFontScale checks whether current font scale is set to expected value (fontScale).
func isFontScale(ctx context.Context, a *arc.ARC, fontScale string) error {
	if err := testing.Poll(ctx, func(ctx context.Context) error {
		currentScale, err := getFontScale(ctx, a)
		if err != nil {
			return err
		}
		if currentScale == fontScale {
			return nil
		}
		return errors.New("current scale is:" + currentScale)
	}, &testing.PollOptions{Timeout: 30 * time.Second}); err != nil {
		return err
	}
	return nil
}

// getFontScale obtains the current font scale from Android.
func getFontScale(ctx context.Context, a *arc.ARC) (string, error) {
	cmd := a.Command(ctx, "settings", "--user", "0", "get", "system", "font_scale")
	res, err := cmd.Output()
	if err != nil {
		cmd.DumpLog(ctx)
		return "", err
	}
	return strings.TrimSpace(string(res)), nil
}

// setChromeFontScale sets the font scale on Chrome.
func setChromeFontScale(ctx context.Context, conn *chrome.Conn, size string) error {
	if err := conn.Exec(ctx, `
		window.__font_size_set_complete = false;
		chrome.fontSettings.setDefaultFontSize(
			{pixelSize: `+size+`},
			function() { window.__font_size_set_complete = true; });
			window.__font_size_fixed_set_complete = false;
		chrome.fontSettings.setDefaultFixedFontSize(
			{pixelSize: `+size+`},
			function() { window.__font_size_fixed_set_complete = true; });`); err != nil {
		return err
	}
	return nil
}

// proxySettingsTestCase contains fields necessary to test proxy settings.
type proxySettingsTestCase struct {
	mode       string
	host       string
	port       string
	bypassList string
	pacURL     string
}

// getAndroidProxy obtains specified proxy value from Android.
// proxy is one of:
// global_http_proxy_host/port|global_proxy_pac_url|global_http_proxy_exclusion_list.
func getAndroidProxy(ctx context.Context, a *arc.ARC, proxyString string) (string, error) {
	cmd := a.Command(ctx, "settings", "get", "global", proxyString)
	res, err := cmd.Output()
	if err != nil {
		cmd.DumpLog(ctx)
		return "", err
	}
	if strings.TrimSpace(string(res)) == "null" {
		return "", nil
	}
	return strings.TrimSpace(string(res)), nil
}

// getChromeProxyFixedServers runs the command to set Chrome proxy settings using a fixed server.
func getChromeProxyFixedServers(ctx context.Context, conn *chrome.Conn, host, port, bypassList string) error {
	if err := conn.Exec(ctx, `
	window.__proxy_set_complete = false;
        chrome.proxy.settings.set(
                {
                    value: {
                        mode: "fixed_servers",
                        rules: {
                            singleProxy: {
                                host: "`+host+`",
                                port: `+port+`
			},
                            bypassList: ["`+bypassList+`"]}
                    },
                    scope: 'regular'
                },
                function() { window.__proxy_set_complete = true });`); err != nil {
		return err
	}
	return nil
}

// getChromeProxyPac runs the command to set Chrome proxy settings using a specified pac script.
func getChromeProxyPac(ctx context.Context, conn *chrome.Conn, pacScript string) error {
	if err := conn.Exec(ctx, `
	window.__proxy_set_complete = false;
            chrome.proxy.settings.set(
                {
                    value: {
                        mode: "pac_script",
                        pacScript: {
                            url: "`+pacScript+`"
                        }
                    },
                    scope: 'regular'
                },
                function() { window.__proxy_set_complete = true });`); err != nil {
		return err
	}
	return nil
}

// getChromeProxyMode runs the command to set proxy mode in Chrome.
func getChromeProxyMode(ctx context.Context, conn *chrome.Conn, mode string) error {
	if err := conn.Exec(ctx, `
            chrome.proxy.settings.set(
                {
                    value: {
                        mode: "`+mode+`"
                    },
                    scope: 'regular'
                },
                function() { window.__proxy_set_complete = true });`); err != nil {
		return err
	}
	return nil
}

// setChromeProxy will set the Chrome proxy, as specified by |mode|.
func (p proxySettingsTestCase) setChromeProxy(ctx context.Context, conn *chrome.Conn) error {
	if p.mode == "fixed_servers" {
		if err := getChromeProxyFixedServers(ctx, conn, p.host, p.port, p.bypassList); err != nil {
			return err
		}
	} else if p.mode == "pac_script" {
		if err := getChromeProxyPac(ctx, conn, p.pacURL); err != nil {
			return err
		}
	} else {
		if err := getChromeProxyMode(ctx, conn, p.mode); err != nil {
			return err
		}
	}

	if err := testing.Poll(ctx, func(ctx context.Context) error {
		var proxySetComplete bool
		if err := conn.Eval(ctx, `window.__proxy_set_complete`, &proxySetComplete); err != nil {
			return err
		}
		if !proxySetComplete {
			return errors.New("proxy not set")
		}
		return nil
	}, &testing.PollOptions{Timeout: 30 * time.Second}); err != nil {
		return err
	}
	return nil
}

// testSpoeknFeedbackEnabled runs the test to ensure spoken feedback settings
// are synchronized between Chrome and Android.
func testSpokenFeedbackEnabled(ctx context.Context, tconn *chrome.Conn, a *arc.ARC) error {
	res, err := isAccessibilityEnabled(ctx, a)
	if err != nil {
		return err
	}
	if res {
		return errors.New("Accessibility is unexpectedly enabled on boot")
	}

	if err = enableAccessibility(ctx, tconn); err != nil {
		return err
	}

	if err = testing.Poll(ctx, func(ctx context.Context) error {
		res, err := isAccessibilityEnabled(ctx, a)
		if err != nil {
			return err
		}
		if res {
			return nil
		}
		return errors.New("Accessibility not enabled")
	}, &testing.PollOptions{Timeout: 30 * time.Second}); err != nil {
		return err
	}
	return nil
}

// testFontSizeSettings runs the test to ensure that font size settings
// are synchronized between Chrone and Android.
func testFontSizeSettings(ctx context.Context, tconn *chrome.Conn, a *arc.ARC) error {
	// const values specifying predetermined font values for testing.
	const (
		superSmallChromeFontSize = "4"
		superLargeChromeFontSize = "100"
		smallestAndroidFontScale = "0.85"
		largestAndroidFontScale  = "1.3"
	)
	if err := setChromeFontScale(ctx, tconn, superSmallChromeFontSize); err != nil {
		return err
	}
	if err := isFontScale(ctx, a, smallestAndroidFontScale); err != nil {
		return err
	}
	if err := setChromeFontScale(ctx, tconn, superLargeChromeFontSize); err != nil {
		return err
	}
	if err := isFontScale(ctx, a, largestAndroidFontScale); err != nil {
		return err
	}
	return nil
}

// testProxySettings runs the test to ensure that proxy settings are
// synchronized between Chrome and Android.
func testProxySettings(ctx context.Context, tconn *chrome.Conn, a *arc.ARC) error {
	directProxyTest := proxySettingsTestCase{
		mode:       "direct",
		host:       "",
		port:       "",
		bypassList: "",
		pacURL:     ""}
	if err := directProxyTest.runProxyTest(ctx, tconn, a); err != nil {
		return err
	}

	fixedServerProxyTest := proxySettingsTestCase{
		mode:       "fixed_servers",
		host:       "proxy",
		port:       "8080",
		bypassList: "foobar.com,*.de"}
	if err := fixedServerProxyTest.runProxyTest(ctx, tconn, a); err != nil {
		return err
	}

	autoDetectProxyTest := proxySettingsTestCase{
		mode:       "auto_detect",
		host:       "localhost",
		port:       "-1",
		bypassList: "",
		pacURL:     "http://wpad/wpad.dat"}
	if err := autoDetectProxyTest.runProxyTest(ctx, tconn, a); err != nil {
		return err
	}

	pacScriptProxyTest := proxySettingsTestCase{
		mode:       "pac_script",
		host:       "localhost",
		port:       "-1",
		bypassList: "",
		pacURL:     "http://example.com"}
	if err := pacScriptProxyTest.runProxyTest(ctx, tconn, a); err != nil {
		return err
	}
	return nil
}

// checkProxySettings checks that current Android proxy settings match with expected values.
func (p proxySettingsTestCase) checkProxySetting(ctx context.Context, a *arc.ARC) error {
	currHost, err := getAndroidProxy(ctx, a, "global_http_proxy_host")
	if err != nil {
		return err
	}
	if currHost != p.host {
		return errors.Errorf("host does not match, got:'%s', want: '%s'", currHost, p.host)
	}

	currPort, err := getAndroidProxy(ctx, a, "global_http_proxy_port")
	if err != nil {
		return err
	}
	if currPort != p.port {
		return errors.Errorf("port does not match, got:'%s', want: '%s'", currPort, p.port)
	}

	currBypassList, err := getAndroidProxy(ctx, a, "global_http_proxy_exclusion_list")
	if err != nil {
		return err
	}
	if currBypassList != p.bypassList {
		return errors.Errorf("bypassList does not match, got:'%s', want: '%s'", currBypassList, p.bypassList)
	}

	currPacURL, err := getAndroidProxy(ctx, a, "global_proxy_pac_url")
	if err != nil {
		return err
	}
	if currPacURL != p.pacURL {
		return errors.Errorf("pacURL does not match, got:'%s', want: '%s'", currPacURL, p.pacURL)
	}

	return nil
}

// runProxyTest performs necessary tasks to ensure that proxy settings are
// synchronized between Chrome and Android.
// First Chrome proxy settings are set, then we check the current proxy
// settings in Android match these.
func (p proxySettingsTestCase) runProxyTest(ctx context.Context, tconn *chrome.Conn, a *arc.ARC) error {
	if err := p.setChromeProxy(ctx, tconn); err != nil {
		return errors.Errorf("set chrome proxy failed: ", err)
	}

	if err := testing.Poll(ctx, func(ctx context.Context) error {
		if err := p.checkProxySetting(ctx, a); err != nil {
			return err
		}
		return nil
	}, &testing.PollOptions{Timeout: 30 * time.Second}); err != nil {
		return errors.Errorf("Timed out waiting for proxy settings change: ", err)
	}
	return nil
}

func SettingsBridge(ctx context.Context, s *testing.State) {
	cr, err := chrome.New(ctx, chrome.ARCEnabled(), chrome.ExtraArgs([]string{"--force-renderer-accessibility"}))
	if err != nil {
		s.Fatal("Failed to connect to Chrome: ", err)
	}
	defer cr.Close(ctx)

	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Creating test API connection failed: ", err)
	}

	a, err := arc.New(ctx, s.OutDir())
	if err != nil {
		s.Fatal("Failed to start ARC: ", err)
	}
	defer a.Close()

	// Run spoken feedback test, to ensure spoken feedback settings are in sync between Chrome and Android.
	if err := testSpokenFeedbackEnabled(ctx, tconn, a); err != nil {
		s.Fatal("Failed to ensure spoken feedback sync: ", err)
	}

	// Run font size test, to ensure font size is in sync between Chrome and Android.
	if err := testFontSizeSettings(ctx, tconn, a); err != nil {
		s.Fatal("Failed to sync font size: ", err)
	}

	// Run proxy settings test, to ensure proxy settings are synchronized between Chrome and Android.
	if err := testProxySettings(ctx, tconn, a); err != nil {
		s.Fatal("Failed to sync proxy settings: ", err)
	}
}
