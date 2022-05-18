// Copyright 2022 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// This package provides a set of functions to collect and setup proxy settings
// from both login screen and Settings app.
// This package only works for "Manual Settings Configuration " at the moment.

package proxysettings

import (
	"context"
	"fmt"
	"time"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/apps"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/chrome/uiauto/checked"
	"chromiumos/tast/local/chrome/uiauto/nodewith"
	"chromiumos/tast/local/chrome/uiauto/ossettings"
	"chromiumos/tast/local/chrome/uiauto/quicksettings"
	"chromiumos/tast/local/chrome/uiauto/restriction"
	"chromiumos/tast/local/chrome/uiauto/role"
	"chromiumos/tast/local/input"
	"chromiumos/tast/testing"
)

// Protocol represents the type of proxy protocols.
type Protocol int

const (
	// HTTP represents the HTTP proxy protocol.
	HTTP Protocol = iota
	// HTTPS represents the HTTPS proxy protocol.
	HTTPS
	// Socks represents the SOCKS proxy protocol.
	Socks
)

// Config represents the proxy configuration.
type Config struct {
	// Protocol is the type of proxy protocol.
	Protocol Protocol
	// Host is the proxy host.
	Host string
	// Port is the proxy port.
	Port string
}

// HostNode returns the node for the proxy host.
func (c *Config) HostNode() *nodewith.Finder {
	switch c.Protocol {
	case HTTP:
		return ossettings.HTTPHostTextField
	case HTTPS:
		return ossettings.HTTPSHostTextField
	case Socks:
		return ossettings.SocksHostTextField
	}
	return nil
}

// HostName returns the name of the proxy host.
func (c *Config) HostName() string {
	switch c.Protocol {
	case HTTP:
		return "http host"
	case HTTPS:
		return "https host"
	case Socks:
		return "socks host"
	}
	return ""
}

// PortNode returns the node for the proxy port.
func (c *Config) PortNode() *nodewith.Finder {
	switch c.Protocol {
	case HTTP:
		return ossettings.HTTPPortTextField
	case HTTPS:
		return ossettings.HTTPSPortTextField
	case Socks:
		return ossettings.SocksPortTextField
	}
	return nil
}

// PortName returns the name of the proxy port.
func (c *Config) PortName() string {
	switch c.Protocol {
	case HTTP:
		return "http port"
	case HTTPS:
		return "https port"
	case Socks:
		return "socks port"
	}
	return ""
}

// ProxySettings represents the proxy-setting page.
// The page could be within the OSSettings window or within the dialog/window on sign-in screen.
// Use `CollectProxySettings` when DUT is logged in already, `CollectProxySettingsFromSigninScreen` otherwise.
// The caller is responsible for calling `Close` to close the OSSettings window or within the dialog/window on sign-in screen.
type ProxySettings struct {
	tconn      *chrome.TestConn
	ui         *uiauto.Context
	isLoggedIn bool
}

// Collect launches the network settings while DUT is logged in.
// This function should not be called while DUT is in the signin screen.
// TODO(b/245560680): Update this method to navigate directly to the detailed
// view of the active WiFi network when this functionality is available.
func Collect(ctx context.Context, tconn *chrome.TestConn) (*ProxySettings, error) {
	ps, err := CollectFromSigninScreen(ctx, tconn)
	if err != nil {
		return nil, err
	}

	if err := expandProxyOption(ctx, tconn); err != nil {
		return nil, errors.Wrap(err, "failed to expand proxy option on settings")
	}
	ps.isLoggedIn = true

	return ps, nil
}

// CollectFromSigninScreen launches the network settings while
// DUT is in the signin screen.
// This function should not be called while DUT is logged in.
// TODO(b/244330490): Update this method to open the network settings by
// clicking the network in the network list in the Quick Settings.
func CollectFromSigninScreen(ctx context.Context, tconn *chrome.TestConn) (*ProxySettings, error) {
	if err := quicksettings.NavigateToNetworkDetailedView(ctx, tconn, false); err != nil {
		return nil, errors.Wrap(err, "failed to navigate to network detailed view")
	}

	if err := quicksettings.OpenNetworkSettings(ctx, tconn, false); err != nil {
		return nil, errors.Wrap(err, "failed to open network settings")
	}

	return &ProxySettings{
		tconn:      tconn,
		ui:         uiauto.New(tconn),
		isLoggedIn: false,
	}, nil
}

// Close clears ProxySettings object and closes Settings app if applied.
func (ps *ProxySettings) Close(ctx context.Context, kb *input.KeyboardEventWriter) {
	if ps.isLoggedIn {
		if err := apps.Close(ctx, ps.tconn, apps.Settings.ID); err != nil {
			testing.ContextLog(ctx, "Failed to close Settings app: ", err)
		}
	} else {
		if err := kb.AccelAction("esc")(ctx); err != nil {
			testing.ContextLog(ctx, "Failed to close Settings window: ", err)
		}
	}
}

// expandProxyOption expands the proxy option within the OS-Settings.
func expandProxyOption(ctx context.Context, tconn *chrome.TestConn) error {
	app := ossettings.New(tconn)
	if err := app.WaitUntilExists(ossettings.ShowProxySettingsTab)(ctx); err != nil {
		return errors.Wrap(err, "failed to find 'Shared networks' toggle button")
	}

	if err := uiauto.Combine("expand 'Proxy' section",
		app.LeftClick(ossettings.ShowProxySettingsTab),
		app.WaitForLocation(ossettings.SharedNetworksToggleButton),
	)(ctx); err != nil {
		return err
	}

	if toggleInfo, err := app.Info(ctx, ossettings.SharedNetworksToggleButton); err != nil {
		return errors.Wrap(err, "failed to get toggle button info")
	} else if toggleInfo.Checked == checked.True {
		testing.ContextLog(ctx, "'Allow proxies for shared networks' is already turned on")
		return nil
	}

	return uiauto.Combine("turn on 'Allow proxies for shared networks' option",
		app.LeftClick(ossettings.SharedNetworksToggleButton),
		app.LeftClick(ossettings.ConfirmButton),
	)(ctx)
}

// Setup sets up proxy values.
// This function is safe to call regardless the network setup page is
// launched or not both OS Settings or on-screen dialog when in login screen.
// To do this, node ancestors are for better flexibility.
func (ps *ProxySettings) Setup(ctx context.Context, cr *chrome.Chrome, kb *input.KeyboardEventWriter, config *Config) error {
	// The port cannot be set alone without the host.
	if config.Host == "" {
		return errors.New("host content is empty")
	}

	if err := uiauto.Combine("setup proxy to 'Manual proxy configuration'",
		ps.ui.LeftClickUntil(ossettings.ProxyDropDownMenu, ps.ui.WithTimeout(3*time.Second).WaitUntilExists(ossettings.ManualProxyOption)),
		ps.ui.LeftClick(ossettings.ManualProxyOption),
		ps.ui.WaitUntilGone(ossettings.ManualProxyOption),
	)(ctx); err != nil {
		return err
	}

	sameProtocolToggle := nodewith.Name("Use the same proxy for all protocols").Role(role.ToggleButton)
	if err := ps.ui.WaitUntilExists(sameProtocolToggle)(ctx); err != nil {
		return errors.Wrap(err, `failed to check "Use the same proxy for all protocols" is enabled or not`)
	}

	// Option "Use the same proxy for all protocols" might be turned on
	// automatically when setting up proxy values. Turn it off to ensure
	// that the rest of proxy values can be set up correctly.
	if err := uiauto.IfFailThen(
		ps.ui.WaitUntilCheckedState(sameProtocolToggle, false),
		ps.ui.WithTimeout(30*time.Second).LeftClickUntil(
			sameProtocolToggle,
			ps.ui.WithTimeout(5*time.Second).WaitUntilCheckedState(sameProtocolToggle, false),
		),
	)(ctx); err != nil {
		return errors.Wrap(err, `failed to disable "Use the same proxy for all protocols"`)
	}

	if err := uiauto.Combine(fmt.Sprintf("setup proxy information with host: %q, port: %q", config.Host, config.Port),
		ps.ui.EnsureFocused(config.HostNode()),
		kb.AccelAction("ctrl+a"),
		kb.TypeAction(config.Host),
		ps.ui.EnsureFocused(config.PortNode()),
		kb.AccelAction("ctrl+a"),
		kb.TypeAction(config.Port),
	)(ctx); err != nil {
		return err
	}

	saveButton := ossettings.WindowFinder.HasClass("action-button").Name("Save").Role(role.Button)
	return uiauto.Combine("save proxy settings",
		ps.ui.MakeVisible(saveButton),
		ps.ui.WaitForLocation(saveButton),
		// Ensure "Save" button has been clicked and become not clickable.
		ps.ui.WithInterval(time.Second).LeftClickUntil(saveButton, ps.ui.CheckRestriction(saveButton, restriction.Disabled)),
	)(ctx)
}

// Content returns the proxy values with specified protocol.
// This function is safe to call when network setup page is launched on
// both OS Settings or on-screen dialog when in login screen. To do this,
// node ancestors are for better flexibility.
func (ps *ProxySettings) Content(ctx context.Context, config *Config) (*Config, error) {
	if err := ps.ui.WaitUntilExists(config.HostNode())(ctx); err != nil {
		return nil, errors.Wrapf(err, "failed to ensure node %q exists and is shown on the screen", config.HostName())
	}

	infoHostNode, err := ps.ui.Info(ctx, config.HostNode())
	if err != nil {
		return nil, errors.Wrapf(err, "failed to get node info for field %q", config.HostName())
	}

	if err := ps.ui.WaitUntilExists(config.PortNode())(ctx); err != nil {
		return nil, errors.Wrapf(err, "failed to ensure node %q exists and is shown on the screen", config.HostName())
	}

	infoPortNode, err := ps.ui.Info(ctx, config.PortNode())
	if err != nil {
		return nil, errors.Wrapf(err, "failed to get node info for field %q", config.PortName())
	}

	return &Config{
		Protocol: config.Protocol,
		Host:     infoHostNode.Value,
		Port:     infoPortNode.Value,
	}, nil
}
