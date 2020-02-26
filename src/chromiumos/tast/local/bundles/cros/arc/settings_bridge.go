// Copyright 2018 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package arc

import (
	"context"
	"fmt"
	"strings"
	"time"

	"chromiumos/tast/ctxutil"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/arc"
	"chromiumos/tast/local/bundles/cros/arc/accessibility"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/testexec"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         SettingsBridge,
		Desc:         "Checks that Chrome settings are persisted in ARC",
		Contacts:     []string{"sarakato@chromium.org", "arc-eng@google.com"},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"chrome"},
		Timeout:      4 * time.Minute,
		Params: []testing.Param{{
			ExtraSoftwareDeps: []string{"android"},
			Pre:               arc.Booted(),
		}, {
			Name:              "vm",
			ExtraAttr:         []string{"informational"},
			ExtraSoftwareDeps: []string{"android_vm"},
			Pre:               arc.VMBooted(),
		}},
	})
}

// checkAndroidAccessibility checks that Android accessibility Settings is expectedly enabled/disabled.
func checkAndroidAccessibility(ctx context.Context, a *arc.ARC, enable bool) error {
	if res, err := accessibility.IsEnabledAndroid(ctx, a); err != nil {
		return err
	} else if res != enable {
		return errors.Errorf("accessibility_enabled is %t in Android", res)
	}

	services, err := accessibility.EnabledAndroidAccessibilityServices(ctx, a)
	if err != nil {
		return err
	}
	enabled := len(services) == 1 && services[0] == accessibility.ArcAccessibilityHelperService
	disabled := len(services) == 1 && len(services[0]) == 0
	if (enable && !enabled) || (!enable && !disabled) {
		return errors.Errorf("enabled accessibility services are not expected: %v", services)
	}

	return nil
}

// testAccessibilitySync runs the test to ensure spoken feedback settings
// are synchronized between Chrome and Android.
func testAccessibilitySync(ctx context.Context, tconn *chrome.TestConn, a *arc.ARC) (retErr error) {
	fullCtx := ctx
	ctx, cancel := ctxutil.Shorten(fullCtx, 10*time.Second)
	defer cancel()

	if res, err := accessibility.IsEnabledAndroid(ctx, a); err != nil {
		return err
	} else if res {
		return errors.New("accessibility is unexpectedly enabled on boot")
	}

	features := []accessibility.Feature{
		accessibility.SpokenFeedback, accessibility.SwitchAccess, accessibility.SelectToSpeak, accessibility.FocusHighlight,
	}

	defer func() {
		for _, feature := range features {
			if err := accessibility.SetFeatureEnabled(fullCtx, tconn, feature, false); err != nil {
				if retErr == nil {
					retErr = errors.Wrapf(err, "failed disabling %s", feature)
				} else {
					retErr = errors.Wrapf(err, "failed disabling %s while cleaning up; and the previous error is %v", feature, retErr)
				}
			}
		}
	}()

	for _, feature := range features {
		testing.ContextLog(ctx, "Testing ", feature)
		for _, enable := range []bool{true, false} {
			if err := accessibility.SetFeatureEnabled(ctx, tconn, feature, enable); err != nil {
				return err
			}

			if err := testing.Poll(ctx, func(ctx context.Context) error {
				if err := checkAndroidAccessibility(ctx, a, enable); err != nil {
					return err
				}
				return nil
			}, &testing.PollOptions{Timeout: 10 * time.Second}); err != nil {
				return errors.Wrapf(err, "could not toggle %s to %t", feature, enable)
			}
		}
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
func setChromeProxyFixedServers(ctx context.Context, tconn *chrome.TestConn, host, port, bypassList string) error {
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
	return tconn.EvalPromise(ctx, script, nil)
}

// setChromeProxyPac runs the command to set Chrome proxy settings using a specified pac script.
func setChromeProxyPac(ctx context.Context, tconn *chrome.TestConn, pacScript string) error {
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
	return tconn.EvalPromise(ctx, script, nil)
}

// setChromeProxyMode runs the command to set proxy mode in Chrome.
func setChromeProxyMode(ctx context.Context, tconn *chrome.TestConn, mode string) error {
	script := fmt.Sprintf(
		`new Promise((resolve) => {
			chrome.proxy.settings.set({
				value: {
					mode: '%s'
				},
				scope: 'regular'
			}, () => {resolve()});
		})`, mode)
	return tconn.EvalPromise(ctx, script, nil)
}

// setChromeProxy sets the Chrome proxy, as specified by p.mode.
func setChromeProxy(ctx context.Context, tconn *chrome.TestConn, p proxySettingsTestCase) error {
	switch p.mode {
	case fixedServers:
		if err := setChromeProxyFixedServers(ctx, tconn, p.host, p.port, p.bypassList); err != nil {
			return err
		}
	case pacScript:
		if err := setChromeProxyPac(ctx, tconn, p.pacURL); err != nil {
			return err
		}
	case autoDetect:
		if err := setChromeProxyMode(ctx, tconn, string(autoDetect)); err != nil {
			return err
		}
	case direct:
		if err := setChromeProxyMode(ctx, tconn, string(direct)); err != nil {
			return err
		}
	default:
		return errors.New("unrecognized proxy mode")

	}

	return nil
}

// testProxySync runs the test to ensure that proxy settings are
// synchronized between Chrome and Android.
func testProxySync(ctx context.Context, tconn *chrome.TestConn, a *arc.ARC) error {
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
func runProxyTest(ctx context.Context, tconn *chrome.TestConn, a *arc.ARC, p proxySettingsTestCase) error {
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
	d := s.PreValue().(arc.PreData)
	a := d.ARC
	cr := d.Chrome

	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Creating test API connection failed: ", err)
	}

	// Run accessibility test.
	if err := testAccessibilitySync(ctx, tconn, a); err != nil {
		s.Error("Failed to sync accessibility: ", err)
	}

	// Run proxy settings test.
	if err := testProxySync(ctx, tconn, a); err != nil {
		s.Error("Failed to sync proxy settings: ", err)
	}
}
