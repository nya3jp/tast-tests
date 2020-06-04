// Copyright 2018 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package arc

import (
	"context"
	"strings"
	"time"

	"chromiumos/tast/ctxutil"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/arc"
	"chromiumos/tast/local/bundles/cros/arc/accessibility"
	"chromiumos/tast/local/bundles/cros/arc/chromeproxy"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/testexec"
	"chromiumos/tast/testing"
)

type settingsBridgeParam struct {
	accessibilityFeatures []accessibility.Feature
	runProxySync          bool
}

var stableSettingsBridgeParam = settingsBridgeParam{
	accessibilityFeatures: []accessibility.Feature{
		accessibility.SpokenFeedback,
	},
	runProxySync: true,
}

var unstableSettingsBridgeParam = settingsBridgeParam{
	accessibilityFeatures: []accessibility.Feature{
		accessibility.SwitchAccess,
		accessibility.SelectToSpeak,
		accessibility.FocusHighlight,
	},
	runProxySync: false,
}

func init() {
	testing.AddTest(&testing.Test{
		Func:         SettingsBridge,
		Desc:         "Checks that Chrome settings are persisted in ARC",
		Contacts:     []string{"sarakato@chromium.org", "arc-eng@google.com"},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"chrome"},
		Pre:          arc.Booted(),
		Timeout:      4 * time.Minute,
		Params: []testing.Param{{
			Val:               stableSettingsBridgeParam,
			ExtraSoftwareDeps: []string{"android_p"},
		}, {
			Name:              "unstable",
			Val:               unstableSettingsBridgeParam,
			ExtraSoftwareDeps: []string{"android_p"},
		}, {
			Name:              "vm",
			Val:               stableSettingsBridgeParam,
			ExtraSoftwareDeps: []string{"android_vm"},
		}, {
			Name:              "vm_unstable",
			Val:               unstableSettingsBridgeParam,
			ExtraSoftwareDeps: []string{"android_vm"},
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

// disableAccessibilityFeatures disables the features specified in features.
func disableAccessibilityFeatures(ctx context.Context, tconn *chrome.TestConn, features []accessibility.Feature) error {
	var failedFeatures []string
	for _, feature := range features {
		if err := accessibility.SetFeatureEnabled(ctx, tconn, feature, false); err != nil {
			failedFeatures = append(failedFeatures, string(feature))
			testing.ContextLogf(ctx, "Failed disabling %s: %v", feature, err)
		}
	}

	if len(failedFeatures) > 0 {
		return errors.Errorf("failed to disable following features: %v", failedFeatures)
	}
	return nil
}

// testAccessibilitySync runs the test to ensure spoken feedback settings
// are synchronized between Chrome and Android.
func testAccessibilitySync(ctx context.Context, tconn *chrome.TestConn, a *arc.ARC, features []accessibility.Feature) (retErr error) {
	fullCtx := ctx
	ctx, cancel := ctxutil.Shorten(fullCtx, 10*time.Second)
	defer cancel()

	if res, err := accessibility.IsEnabledAndroid(ctx, a); err != nil {
		return err
	} else if res {
		return errors.New("accessibility is unexpectedly enabled on boot")
	}

	defer func() {
		if err := disableAccessibilityFeatures(ctx, tconn, features); err != nil {
			if retErr == nil {
				retErr = err
			} else {
				testing.ContextLog(ctx, "Failed to disable accessibliity features: ", err)
			}
		}
	}()

	for _, feature := range features {
		testing.ContextLog(ctx, "Testing ", feature)
		if feature == accessibility.SwitchAccess {
			// Ensure that disable switch access confirmation dialog does not get shown.
			// If there is an err here, switch access will not be enabled, meaning that switch access
			// will not be disabled in the above disableA11yFeatures(). In this situation, the "switch access
			// disable dialog" will not be shown.
			if err := tconn.Eval(ctx, `chrome.autotestPrivate.disableSwitchAccessDialog();`, nil); err != nil {
				return err
			}
		}
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

// proxySettingsTestCase contains fields necessary to test proxy settings.
type proxySettingsTestCase struct {
	name       string                  // subtestcase name
	config     chromeproxy.ProxyConfig // config value to be set
	host       string                  // expected host name
	port       string                  // expected port
	bypassList string                  // expected bypassList
	pacURL     string                  // expected proxy auto-config file URL
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

// testProxySync runs the test to ensure that proxy settings are
// synchronized between Chrome and Android.
func testProxySync(ctx context.Context, s *testing.State, tconn *chrome.TestConn, a *arc.ARC) error {
	for _, tc := range []proxySettingsTestCase{{
		name: "Direct",
		config: chromeproxy.ProxyConfig{
			Mode: chromeproxy.ModeDirect,
		},
	}, {
		name: "FixedServers",
		config: chromeproxy.ProxyConfig{
			Mode: chromeproxy.ModeFixedServers,
			Rules: chromeproxy.ProxyRules{
				SingleProxy: chromeproxy.ProxyServer{
					Host: "proxy",
					Port: 8080,
				},
				BypassList: []string{"foobar.com", "*.de"},
			},
		},
		host:       "proxy",
		port:       "8080",
		bypassList: "foobar.com,*.de",
	}, {
		name: "AutoDetect",
		config: chromeproxy.ProxyConfig{
			Mode: chromeproxy.ModeAutoDetect,
		},
		host:   "localhost",
		port:   "-1",
		pacURL: "http://wpad/wpad.dat",
	}, {
		name: "PacScript",
		config: chromeproxy.ProxyConfig{
			Mode: chromeproxy.ModePacScript,
			PacScript: chromeproxy.PacScript{
				URL: "http://example.com",
			},
		},
		host:   "localhost",
		port:   "-1",
		pacURL: "http://example.com",
	}} {
		s.Run(ctx, tc.name, func(ctx context.Context, s *testing.State) {
			if err := runProxyTest(ctx, tconn, a, tc); err != nil {
				s.Error("Failed to run proxy sync: ", err)
			}
		})
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
	if err := chromeproxy.SetSettings(ctx, tconn, chromeproxy.ProxySettings{Value: p.config}); err != nil {
		return errors.Wrap(err, "setting chrome proxy failed")
	}

	return testing.Poll(ctx, func(ctx context.Context) error {
		return checkProxySettings(ctx, a, p)
	}, &testing.PollOptions{Timeout: 30 * time.Second})
}

func SettingsBridge(ctx context.Context, s *testing.State) {
	param := s.Param().(settingsBridgeParam)
	d := s.PreValue().(arc.PreData)
	a := d.ARC
	cr := d.Chrome

	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Creating test API connection failed: ", err)
	}

	// Run accessibility test.
	if err := testAccessibilitySync(ctx, tconn, a, param.accessibilityFeatures); err != nil {
		s.Error("Failed to sync accessibility: ", err)
	}

	// Run proxy settings test.
	if param.runProxySync {
		testProxySync(ctx, s, tconn, a)
	}
}
