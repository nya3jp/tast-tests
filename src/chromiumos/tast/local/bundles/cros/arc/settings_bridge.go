// Copyright 2018 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package arc

import (
	"context"
	"fmt"
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
	script := `
	window.__spoken_feedback_set_complete = false;
	chrome.accessibilityFeatures.spokenFeedback.set({value: true});
	chrome.accessibilityFeatures.spokenFeedback.get({}, () => {
		window.__spoken_feedback_set_complete = true;
	})`
	return conn.Exec(ctx, script)
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

// waitFontScale checks whether current font scale is set to expected value (fontScale).
func waitFontScale(ctx context.Context, a *arc.ARC, fontScale string) error {
	return testing.Poll(ctx, func(ctx context.Context) error {
		currentScale, err := getFontScale(ctx, a)
		if err != nil {
			return err
		}
		if currentScale == fontScale {
			return nil
		}
		return errors.Errorf("current scale is: %s", currentScale)
	}, &testing.PollOptions{Timeout: 30 * time.Second})
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
	script := fmt.Sprintf(`
		chrome.fontSettings.setDefaultFontSize(
			{pixelSize: %s},
			function() {});
		chrome.fontSettings.setDefaultFixedFontSize(
			{pixelSize: %s},
			function() {});`, size, size)
	return conn.Exec(ctx, script)
}

// proxyMode represents specifies values for |mode| property, which determines
// behaviour of Chrome's proxy usage.
type proxyMode string

const (
	direct       proxyMode = "direct"
	fixedServers proxyMode = "fixed_servers"
	autoDetect   proxyMode = "auto_detect"
	pacScript    proxyMode = "pac_script"
)

// proxySettingsTestCase contains fields necessary to test proxy settings.
type proxySettingsTestCase struct {
	mode       proxyMode // mode for test case
	host       string    // its host
	port       string    // its port
	bypassList string    // list of servers to be exluded from being proxied
	pacURL     string    // proxy auto-config file URL
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
	proxy := strings.TrimSpace(string(res))
	if proxy == "null" {
		return "", nil
	}
	return proxy, nil
}

// setChromeProxyFixedServers runs the command to set Chrome proxy settings using a fixed server.
func setChromeProxyFixedServers(ctx context.Context, conn *chrome.Conn, host, port, bypassList string) error {
	script := fmt.Sprintf(`
		new Promise((resolve) => {
        chrome.proxy.settings.set(
                {
                    value: {
                        mode: "fixed_servers",
                        rules: {
                            singleProxy: {
                                host: "%s",
                                port: %s
														},
                            bypassList: ["%s"]}
												},
                    scope: 'regular'
                },
                function() { window.__proxy_set_complete = true; resolve();});
		})
	`, host, port, bypassList)
	return conn.EvalPromise(ctx, script, nil)
}

// setChromeProxyPac runs the command to set Chrome proxy settings using a specified pac script.
func setChromeProxyPac(ctx context.Context, conn *chrome.Conn, pacScript string) error {
	script := fmt.Sprintf(`
            new Promise((resolve) => {
		    chrome.proxy.settings.set(
			{
			    value: {
				mode: "pac_script",
				pacScript: {
				    url: "%s"
				}
			    },
			    scope: 'regular'
			},
                function() { window.__proxy_set_complete = true; resolve(); });
	    })`, pacScript)
	return conn.EvalPromise(ctx, script, nil)
}

// setChromeProxyMode runs the command to set proxy mode in Chrome.
func setChromeProxyMode(ctx context.Context, conn *chrome.Conn, mode string) error {
	script := fmt.Sprintf(`
	new Promise((resolve, reject) => {
            chrome.proxy.settings.set(
                {
                    value: {
                        mode: "%s"
                    },
                    scope: 'regular'
                },
                function() { window.__proxy_set_complete = true; resolve(); });
	})`, mode)
	return conn.EvalPromise(ctx, script, nil)
}

// setChromeProxy will set the Chrome proxy, as specified by |mode|.
func setChromeProxy(ctx context.Context, conn *chrome.Conn, p proxySettingsTestCase) error {
	switch p.mode {
	case fixedServers:
		if err := setChromeProxyFixedServers(ctx, conn, p.host, p.port, p.bypassList); err != nil {
			return err
		}
	case pacScript:
		if err := setChromeProxyPac(ctx, conn, p.pacURL); err != nil {
			return err
		}
	case autoDetect:
		if err := setChromeProxyMode(ctx, conn, string(autoDetect)); err != nil {
			return err
		}
	case direct:
		if err := setChromeProxyMode(ctx, conn, string(direct)); err != nil {
			return err
		}
	default:
		return errors.New("unrecognised proxy mode")

	}

	return testing.Poll(ctx, func(ctx context.Context) error {
		var proxySetComplete bool
		if err := conn.Eval(ctx, `window.__proxy_set_complete`, &proxySetComplete); err != nil {
			return err
		}
		if !proxySetComplete {
			return errors.New("proxy not set")
		}
		return nil
	}, &testing.PollOptions{Timeout: 30 * time.Second})
}

// testSpoeknFeedbackEnabled runs the test to ensure spoken feedback settings
// are synchronized between Chrome and Android.
func testSpokenFeedbackSync(ctx context.Context, tconn *chrome.Conn, a *arc.ARC) error {
	res, err := isAccessibilityEnabled(ctx, a)
	if err != nil {
		return err
	}
	if res {
		return errors.New("accessibility is unexpectedly enabled on boot")
	}

	if err = enableAccessibility(ctx, tconn); err != nil {
		return err
	}

	return testing.Poll(ctx, func(ctx context.Context) error {
		res, err := isAccessibilityEnabled(ctx, a)
		if err != nil {
			return err
		}
		if res {
			return nil
		}
		return errors.New("accessibility not enabled")
	}, &testing.PollOptions{Timeout: 30 * time.Second})
}

// testFontSizeSync runs the test to ensure that font size settings
// are synchronized between Chrone and Android.
func testFontSizeSync(ctx context.Context, tconn *chrome.Conn, a *arc.ARC) error {
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
	if err := waitFontScale(ctx, a, smallestAndroidFontScale); err != nil {
		return err
	}
	if err := setChromeFontScale(ctx, tconn, superLargeChromeFontSize); err != nil {
		return err
	}
	if err := waitFontScale(ctx, a, largestAndroidFontScale); err != nil {
		return err
	}
	return nil
}

// testProxySync runs the test to ensure that proxy settings are
// synchronized between Chrome and Android.
func testProxySync(ctx context.Context, tconn *chrome.Conn, a *arc.ARC) error {
	for _, tc := range []proxySettingsTestCase{
		{mode: direct},
		{mode: fixedServers,
			host:       "proxy",
			port:       "8080",
			bypassList: "foobar.com,*.de"},
		{mode: autoDetect,
			host:   "localhost",
			port:   "-1",
			pacURL: "http://wpad/wpad.dat"},
		{mode: pacScript,
			host:   "localhost",
			port:   "-1",
			pacURL: "http://example.com"}} {
		if err := runProxyTest(ctx, tconn, a, tc); err != nil {
			return err
		}
	}
	return nil
}

// checkProxySettings checks that current Android proxy settings match with expected values.
func checkProxySetting(ctx context.Context, a *arc.ARC, p proxySettingsTestCase) error {
	currHost, err := getAndroidProxy(ctx, a, "global_http_proxy_host")
	if err != nil {
		return err
	}
	if currHost != p.host {
		return errors.Errorf("host does not match, got:%q, want: %q", currHost, p.host)
	}

	currPort, err := getAndroidProxy(ctx, a, "global_http_proxy_port")
	if err != nil {
		return err
	}
	if currPort != p.port {
		return errors.Errorf("port does not match, got:%q, want: %q", currPort, p.port)
	}

	currBypassList, err := getAndroidProxy(ctx, a, "global_http_proxy_exclusion_list")
	if err != nil {
		return err
	}
	if currBypassList != p.bypassList {
		return errors.Errorf("bypassList does not match, got:%q, want: %q", currBypassList, p.bypassList)
	}

	currPacURL, err := getAndroidProxy(ctx, a, "global_proxy_pac_url")
	if err != nil {
		return err
	}
	if currPacURL != p.pacURL {
		return errors.Errorf("pacURL does not match, got:%q, want: %q", currPacURL, p.pacURL)
	}

	return nil
}

// runProxyTest performs necessary tasks to ensure that proxy settings are
// synchronized between Chrome and Android.
// First Chrome proxy settings are set, then we check the current proxy
// settings in Android match these.
func runProxyTest(ctx context.Context, tconn *chrome.Conn, a *arc.ARC, p proxySettingsTestCase) error {
	if err := setChromeProxy(ctx, tconn, p); err != nil {
		return errors.Errorf("setting chrome proxy failed: ", err)
	}

	if err := testing.Poll(ctx, func(ctx context.Context) error {
		if err := checkProxySetting(ctx, a, p); err != nil {
			return err
		}
		return nil
	}, &testing.PollOptions{Timeout: 30 * time.Second}); err != nil {
		return errors.Errorf("timed out waiting for proxy settings change: ", err)
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
	if err := testSpokenFeedbackSync(ctx, tconn, a); err != nil {
		s.Error("Failed to ensure spoken feedback sync: ", err)
	}

	// Run font size test, to ensure font size is in sync between Chrome and Android.
	if err := testFontSizeSync(ctx, tconn, a); err != nil {
		s.Error("Failed to sync font size: ", err)
	}

	// Run proxy settings test, to ensure proxy settings are synchronized between Chrome and Android.
	if err := testProxySync(ctx, tconn, a); err != nil {
		s.Error("Failed to sync proxy settings: ", err)
	}
}
