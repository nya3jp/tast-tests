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
	"chromiumos/tast/local/testexec"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         SettingsBridge,
		Desc:         "Checks that Chrome settings are persisted in ARC",
		Contacts:     []string{"sarakato@chromium.org", "arc-eng@google.com"},
		Attr:         []string{"informational"},
		SoftwareDeps: []string{"android_both", "chrome"},
		Timeout:      4 * time.Minute,
	})
}

// enableAccessibility enables spoken feedback on Chrome.
func enableAccessibility(ctx context.Context, conn *chrome.Conn) error {
	const script = "chrome.accessibilityFeatures.spokenFeedback.set({value: true})"
	return conn.Exec(ctx, script)
}

// isAccessibilityEnabled checks whether accessibility is enabled on Android.
func isAccessibilityEnabled(ctx context.Context, a *arc.ARC) (bool, error) {
	res, err := a.Command(ctx, "settings", "--user", "0", "get", "secure", "accessibility_enabled").Output(testexec.DumpLogOnError)
	if err != nil {
		return false, err
	}
	return strings.TrimSpace(string(res)) == "1", nil
}

// testSpokenFeedbackSync runs the test to ensure spoken feedback settings
// are synchronized between Chrome and Android.
func testSpokenFeedbackSync(ctx context.Context, tconn *chrome.Conn, a *arc.ARC) error {
	if res, err := isAccessibilityEnabled(ctx, a); err != nil {
		return err
	} else if res {
		return errors.New("accessibility is unexpectedly enabled on boot")
	}

	if err := enableAccessibility(ctx, tconn); err != nil {
		return err
	}

	return testing.Poll(ctx, func(ctx context.Context) error {
		if res, err := isAccessibilityEnabled(ctx, a); err != nil {
			return err
		} else if !res {
			return errors.New("accessibility not enabled")
		}
		return nil
	}, &testing.PollOptions{Timeout: 30 * time.Second})
}

// waitFontScale checks whether current font scale is set to expected value (fontScale).
func waitFontScale(ctx context.Context, a *arc.ARC, fontScale string) error {
	return testing.Poll(ctx, func(ctx context.Context) error {
		currentScale, err := getFontScale(ctx, a)
		if err != nil {
			return err
		}
		if currentScale != fontScale {
			return errors.Errorf("current scale is: %s", currentScale)
		}
		return nil
	}, &testing.PollOptions{Timeout: 30 * time.Second})
}

// getFontScale obtains current font scale from Android.
func getFontScale(ctx context.Context, a *arc.ARC) (string, error) {
	res, err := a.Command(ctx, "settings", "--user", "0", "get", "system", "font_scale").Output(testexec.DumpLogOnError)
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(res)), nil
}

// setChromeFontScale sets the font scale on Chrome.
func setChromeFontScale(ctx context.Context, conn *chrome.Conn, size string) error {
	script := fmt.Sprintf(`
		chrome.fontSettings.setDefaultFontSize({pixelSize: %s}, () => {});
		chrome.fontSettings.setDefaultFixedFontSize({pixelSize: %s}, () => {});`, size, size)
	return conn.Exec(ctx, script)
}

// setCaptionTextSize sets the caption text size on Chrome.
func setCaptionTextSize(ctx context.Context, conn *chrome.Conn, size string) error {
	script := fmt.Sprintf(`
		chrome.fontSettings.setCaptionTextSize({captionTextSize: "%v"}, () => {});`, size)
	return conn.Exec(ctx, script)
}

// setCaptionTextColor sets the caption text color on Chrome.
func setCaptionTextColor(ctx context.Context, conn *chrome.Conn, color string) error {
	script := fmt.Sprintf(`
		chrome.fontSettings.setCaptionTextColor({captionTextColor: "%v"}, () => {});`, color)
	return conn.Exec(ctx, script)
}

// setCaptionBackgroundColor sets the caption background color on Chrome.
func setCaptionBackgroundColor(ctx context.Context, conn *chrome.Conn, color string) error {
	script := fmt.Sprintf(`
		chrome.fontSettings.setCaptionBackgroundColor({captionBackgroundColor: "%v"}, () => {});`, color)
	return conn.Exec(ctx, script)
}

// setCaptionTextOpacity sets the caption text opacity on Chrome.
func setCaptionTextOpacity(ctx context.Context, conn *chrome.Conn, opacity int) error {
	script := fmt.Sprintf(`
		chrome.fontSettings.setCaptionTextOpacity({captionTextOpacity: %v}, () => {});`, opacity)
	return conn.Exec(ctx, script)
}

// setCaptionBackgroundOpacity sets the caption background opacity on Chrome.
func setCaptionBackgroundOpacity(ctx context.Context, conn *chrome.Conn, opacity int) error {
	script := fmt.Sprintf(`
		chrome.fontSettings.setCaptionBackgroundOpacity({captionBackgroundOpacity: %v}, () => {});`, opacity)
	return conn.Exec(ctx, script)
}

// setCaptionTextShadow sets the caption text shadow on Chrome.
func setCaptionTextShadow(ctx context.Context, conn *chrome.Conn, color string) error {
	script := fmt.Sprintf(`
		chrome.fontSettings.setCaptionTextShadow({captionTextShadow: "%v"}, () => {});`, color)
	return conn.Exec(ctx, script)
}

// testFontSizeSync runs the test to ensure that font size settings
// are synchronized between Chrome and Android.
func testFontSizeSync(ctx context.Context, tconn *chrome.Conn, a *arc.ARC) error {
	// const values specifying font values for testing.
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

// testCaptionSettingsSync runs the test to ensure that caption settings
// are synchronized between Chrome and Android.
func testCaptionSettingsSync(ctx context.Context, tconn *chrome.Conn, a *arc.ARC) error {
	// const values specifying font values for testing.
	const (
		captionTextSize           = "43%"
		expectedFontScale         = "0.43"
		captionTextColor          = "0,0,0"
		captionTextOpacity        = 50
		expectedForegroundColor   = "-2147483648"
		captionBackgroundColor    = "255,0,0"
		captionBackgroundOpacity  = 50
		expectedBackgroundColor   = "-2130771968"
		captionTextShadow         = "2px 2px 4px rgba(0, 0, 0, 0.5)"
		expectedEdgeType          = "4"
		expectedCaptioningEnabled = "1"
	)
	if err := setCaptionTextSize(ctx, tconn, captionTextSize); err != nil {
		return err
	}

	if err := setCaptionTextColor(ctx, tconn, captionTextColor); err != nil {
		return err
	}

	if err := setCaptionBackgroundColor(ctx, tconn, captionBackgroundColor); err != nil {
		return err
	}

	if err := setCaptionTextShadow(ctx, tconn, captionTextShadow); err != nil {
		return err
	}

	if err := setCaptionTextOpacity(ctx, tconn, captionTextOpacity); err != nil {
		return err
	}

	if err := setCaptionBackgroundOpacity(ctx, tconn, captionBackgroundOpacity); err != nil {
		return err
	}

	testing.ContextLog(ctx, "Evan123 log")
	actualFontScale, err := a.Command(ctx, "settings", "--user", "0", "get", "secure", "accessibility_captioning_font_scale").Output(testexec.DumpLogOnError)
	actualBackgroundColor, err := a.Command(ctx, "settings", "--user", "0", "get", "secure", "accessibility_captioning_background_color").Output(testexec.DumpLogOnError)
	actualCaptioningEnabled, err := a.Command(ctx, "settings", "--user", "0", "get", "secure", "accessibility_captioning_enabled").Output(testexec.DumpLogOnError)
	actualEdgeType, err := a.Command(ctx, "settings", "--user", "0", "get", "secure", "accessibility_captioning_edge_type").Output(testexec.DumpLogOnError)
	actualForegroundColor, err := a.Command(ctx, "settings", "--user", "0", "get", "secure", "accessibility_captioning_foreground_color").Output(testexec.DumpLogOnError)

	if err != nil {
		return err
	}

	if strings.TrimSpace(string(actualFontScale)) != expectedFontScale {
		return errors.Errorf("Actual font scale is %s, expected %s", actualFontScale, expectedFontScale)
	}

	if strings.TrimSpace(string(actualBackgroundColor)) != expectedBackgroundColor {
		return errors.Errorf("Actual background color is %s, expected %s", actualBackgroundColor, expectedBackgroundColor)
	}

	if strings.TrimSpace(string(actualCaptioningEnabled)) != expectedCaptioningEnabled {
		return errors.Errorf("Actual captioning enabled is %s, expected %s", actualCaptioningEnabled, expectedCaptioningEnabled)
	}

	if strings.TrimSpace(string(actualEdgeType)) != expectedEdgeType {
		return errors.Errorf("Actual edge type is %s, expected %s", actualEdgeType, expectedEdgeType)
	}

	if strings.TrimSpace(string(actualForegroundColor)) != expectedForegroundColor {
		return errors.Errorf("Actual foreground color is %s, expected %s", actualForegroundColor, expectedForegroundColor)
	}

	return nil
}

// proxyMode represents values for mode property, which determines
// behaviour of Chrome's proxy usage.
type proxyMode string

const (
	direct       proxyMode = "direct"
	fixedServers           = "fixed_servers"
	autoDetect             = "auto_detect"
	pacScript              = "pac_script"
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
// global_http_proxy_host|global_http_proxy_port|global_proxy_pac_url|global_http_proxy_exclusion_list.
func getAndroidProxy(ctx context.Context, a *arc.ARC, proxyString string) (string, error) {
	res, err := a.Command(ctx, "settings", "get", "global", proxyString).Output(testexec.DumpLogOnError)
	if err != nil {
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
	script := fmt.Sprintf(
		`new Promise((resolve) => {
			chrome.proxy.settings.set({
				value: {
					mode: 'fixed_servers',
					rules: {
						singleProxy: {
							host: '%s',
							port: %s
						},
					bypassList: ['%s']
					}
				},
				scope: 'regular'
			}, () => {resolve()});
		})`, host, port, bypassList)
	return conn.EvalPromise(ctx, script, nil)
}

// setChromeProxyPac runs the command to set Chrome proxy settings using a specified pac script.
func setChromeProxyPac(ctx context.Context, conn *chrome.Conn, pacScript string) error {
	script := fmt.Sprintf(
		`new Promise((resolve) => {
			chrome.proxy.settings.set({
				value: {
					mode: 'pac_script',
					pacScript: {
						url: '%s'
					}
				},
				scope: 'regular'
			}, () => {resolve()});
		})`, pacScript)
	return conn.EvalPromise(ctx, script, nil)
}

// setChromeProxyMode runs the command to set proxy mode in Chrome.
func setChromeProxyMode(ctx context.Context, conn *chrome.Conn, mode string) error {
	script := fmt.Sprintf(
		`new Promise((resolve) => {
			chrome.proxy.settings.set({
				value: {
					mode: '%s'
				},
				scope: 'regular'
			}, () => {resolve()});
		})`, mode)
	return conn.EvalPromise(ctx, script, nil)
}

// setChromeProxy sets the Chrome proxy, as specified by p.mode.
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
		return errors.New("unrecognized proxy mode")

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
func checkProxySettings(ctx context.Context, a *arc.ARC, p proxySettingsTestCase) error {
	currHost, err := getAndroidProxy(ctx, a, "global_http_proxy_host")
	if err != nil {
		return err
	}
	if currHost != p.host {
		return errors.Errorf("host does not match, got %q, want %q", currHost, p.host)
	}

	currPort, err := getAndroidProxy(ctx, a, "global_http_proxy_port")
	if err != nil {
		return err
	}
	if currPort != p.port {
		return errors.Errorf("port does not match, got %q, want %q", currPort, p.port)
	}

	currBypassList, err := getAndroidProxy(ctx, a, "global_http_proxy_exclusion_list")
	if err != nil {
		return err
	}
	if currBypassList != p.bypassList {
		return errors.Errorf("bypassList does not match, got %q, want %q", currBypassList, p.bypassList)
	}

	currPacURL, err := getAndroidProxy(ctx, a, "global_proxy_pac_url")
	if err != nil {
		return err
	}
	if currPacURL != p.pacURL {
		return errors.Errorf("pacURL does not match, got %q, want %q", currPacURL, p.pacURL)
	}

	return nil
}

// runProxyTest performs necessary tasks to ensure that proxy settings are
// synchronized between Chrome and Android.
// Proxy settings in Chrome are set, then the proxy settings in Android are checked to see if they match.
func runProxyTest(ctx context.Context, tconn *chrome.Conn, a *arc.ARC, p proxySettingsTestCase) error {
	if err := setChromeProxy(ctx, tconn, p); err != nil {
		return errors.Wrap(err, "setting chrome proxy failed")
	}

	return testing.Poll(ctx, func(ctx context.Context) error {
		if err := checkProxySettings(ctx, a, p); err != nil {
			return err
		}
		return nil
	}, &testing.PollOptions{Timeout: 30 * time.Second})
}

func SettingsBridge(ctx context.Context, s *testing.State) {
	cr, err := chrome.New(ctx, chrome.ARCEnabled(), chrome.ExtraArgs("--force-renderer-accessibility"))
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

	// Run spoken feedback test.
	if err := testSpokenFeedbackSync(ctx, tconn, a); err != nil {
		s.Error("Failed to ensure spoken feedback sync: ", err)
	}

	// Run font size test.
	if err := testFontSizeSync(ctx, tconn, a); err != nil {
		s.Error("Failed to sync font size: ", err)
	}

	// Run caption settings test.
	if err := testCaptionSettingsSync(ctx, tconn, a); err != nil {
		s.Error("Failed to sync caption settings: ", err)
	}

	// Run proxy settings test.
	if err := testProxySync(ctx, tconn, a); err != nil {
		s.Error("Failed to sync proxy settings: ", err)
	}
}
