// Copyright 2022 The ChromiumOS Authors.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package network

import (
	"context"
	"fmt"
	"path/filepath"
	"time"

	"github.com/golang/protobuf/ptypes/empty"
	"google.golang.org/grpc"

	"chromiumos/tast/ctxutil"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/bundles/cros/network/shill"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/chrome/uiauto/faillog"
	"chromiumos/tast/local/chrome/uiauto/nodewith"
	"chromiumos/tast/local/chrome/uiauto/ossettings"
	"chromiumos/tast/local/chrome/uiauto/quicksettings"
	"chromiumos/tast/local/chrome/uiauto/role"
	"chromiumos/tast/local/input"
	"chromiumos/tast/services/cros/network"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddService(&testing.Service{
		Register: func(srv *grpc.Server, s *testing.ServiceState) {
			network.RegisterProxySettingServiceServer(srv, &ProxySetupAndVerifyService{s: s})
		},
	})
}

// ProxySetupAndVerifyService implements tast.cros.network.ProxySetupAndVerifyService.
type ProxySetupAndVerifyService struct {
	s     *testing.ServiceState
	cr    *chrome.Chrome
	tconn *chrome.TestConn
	ui    *uiauto.Context
	kb    *input.KeyboardEventWriter
}

// New sets up the login process.
func (s *ProxySetupAndVerifyService) New(ctx context.Context, req *network.NewRequest) (_ *empty.Empty, retErr error) {
	opts := []chrome.Option{
		chrome.LoadSigninProfileExtension(req.ManifestKey),
		chrome.NoLogin(),
	}

	if req.ShouldKeepState {
		opts = append(opts, chrome.KeepState())
	} else {
		// (b/242474992) Proxy values will be preserved across Chrome sessions
		// and will not be wiped out automatically. Therefore, we need to
		// wipe out the proxy values before starting new test.
		if err := s.cleanupProxy(ctx); err != nil {
			return &empty.Empty{}, errors.Wrap(err, "failed to cleanup proxy")
		}
	}

	cleanupCtx := ctx
	ctx, cancel := ctxutil.Shorten(ctx, 10*time.Second)
	defer cancel()

	var err error
	s.cr, err = chrome.New(ctx, opts...)
	if err != nil {
		return &empty.Empty{}, errors.Wrap(err, "failed to start chrome")
	}
	defer func(ctx context.Context) {
		if retErr != nil {
			s.Close(ctx, &network.CloseRequest{Cleanup: true})
		}

	}(cleanupCtx)

	s.tconn, err = s.cr.SigninProfileTestAPIConn(ctx)
	if err != nil {
		return &empty.Empty{}, errors.Wrap(err, "failed to connect to test API")
	}

	s.ui = uiauto.New(s.tconn)

	s.kb, err = input.Keyboard(ctx)
	if err != nil {
		return &empty.Empty{}, errors.Wrap(err, "failed to get keyboard")
	}

	return &empty.Empty{}, nil
}

// Close releases the resources obtained by New.
func (s *ProxySetupAndVerifyService) Close(ctx context.Context, req *network.CloseRequest) (*empty.Empty, error) {
	if s.kb != nil {
		if err := s.kb.Close(); err != nil {
			testing.ContextLog(ctx, "Failed to close keyboard: ", err)
		}
		s.kb = nil
	}

	if s.cr != nil {
		if err := s.cr.Close(ctx); err != nil {
			testing.ContextLog(ctx, "Failed to close chrome: ", err)
		}
		s.cr = nil
	}

	if req.Cleanup {
		if err := s.cleanupProxy(ctx); err != nil {
			testing.ContextLog(ctx, "Failed to cleanup proxy: ", err)
		}
	}
	return &empty.Empty{}, nil
}

// dumpUITreeToFile is a helper function to acquire the ContextOutDir and dumps the UI tree to a file.
func (s *ProxySetupAndVerifyService) dumpUITreeToFile(ctx context.Context, hasError func() bool, namePrefix string) {
	if hasError() {
		outDir, ok := testing.ContextOutDir(ctx)
		if !ok || outDir == "" {
			testing.ContextLog(ctx, "Failed to get output dir")
		} else {
			destPath := filepath.Join(outDir, "service")
			testing.ContextLogf(ctx, "Dumping UI tree to %q", destPath)
			faillog.DumpUITreeToFile(ctx, destPath, s.tconn, fmt.Sprintf("%s.txt", namePrefix))
		}
	}
}

// launchProxySection launches a proxy section via Quick Settings.
// This function also turns on "Allow proxies for shared networks" option.
func (s *ProxySetupAndVerifyService) launchProxySection(ctx context.Context) error {
	if err := quicksettings.NavigateToNetworkDetailedView(ctx, s.tconn); err != nil {
		return errors.Wrap(err, "failed to navigate to network detailed view")
	}

	if err := quicksettings.OpenNetworkSettings(ctx, s.tconn); err != nil {
		return errors.Wrap(err, "failed to open network settings")
	}

	return nil
}

// SetupProxy sets up proxy values.
func (s *ProxySetupAndVerifyService) SetupProxy(ctx context.Context, req *network.ProxyValuesRequest) (_ *empty.Empty, retErr error) {
	cleanupCtx := ctx
	ctx, cancel := ctxutil.Shorten(ctx, 10*time.Second)
	defer cancel()
	defer s.dumpUITreeToFile(cleanupCtx, func() bool { return retErr != nil }, "ui_dump_setup_proxy")

	if err := s.launchProxySection(ctx); err != nil {
		return &empty.Empty{}, errors.Wrap(err, "failed to launch proxy section")
	}

	if err := uiauto.Combine("setup proxy to 'Manual proxy configuration'",
		s.ui.LeftClickUntil(ossettings.ProxyDropDownMenu, s.ui.Exists(ossettings.ManualProxyOption)),
		s.ui.LeftClick(ossettings.ManualProxyOption),
		s.ui.WaitUntilGone(ossettings.ManualProxyOption),
	)(ctx); err != nil {
		return &empty.Empty{}, err
	}

	pv := s.fetchProxyFieldAndValues(req)
	for fieldName, content := range pv {
		testing.ContextLogf(ctx, "Setting proxy value %q to field %q", content.value, fieldName)
		if err := uiauto.Combine(fmt.Sprintf("replace and type text %q to field %q", content.value, fieldName),
			s.ui.EnsureFocused(content.node),
			s.kb.AccelAction("ctrl+a"),
			s.kb.AccelAction("backspace"),
			s.kb.TypeAction(content.value),
		)(ctx); err != nil {
			return &empty.Empty{}, err
		}
	}

	saveButton := ossettings.WindowFinder.HasClass("action-button").Name("Save").Role(role.Button)
	return &empty.Empty{}, uiauto.Combine("save proxy settings",
		// Save changes.
		s.ui.MakeVisible(saveButton),
		s.ui.LeftClick(saveButton),
	)(ctx)
}

// VerifyProxy verifies proxy values.
func (s *ProxySetupAndVerifyService) VerifyProxy(ctx context.Context, req *network.ProxyValuesRequest) (_ *empty.Empty, retErr error) {
	cleanupCtx := ctx
	ctx, cancel := ctxutil.Shorten(ctx, 10*time.Second)
	defer cancel()
	defer s.dumpUITreeToFile(cleanupCtx, func() bool { return retErr != nil }, "ui_dump_verify_proxy")

	if err := s.launchProxySection(ctx); err != nil {
		return &empty.Empty{}, errors.Wrap(err, "failed to launch proxy section")
	}

	pv := s.fetchProxyFieldAndValues(req)
	for fieldName, content := range pv {
		testing.ContextLogf(ctx, "Verify if the value of the field %q is %q", fieldName, content.value)
		if err := s.ui.EnsureFocused(content.node)(ctx); err != nil {
			return &empty.Empty{}, errors.Wrap(err, "failed to ensure node exists and is shown on the screen")
		}
		if info, err := s.ui.Info(ctx, content.node); err != nil {
			return &empty.Empty{}, errors.Wrap(err, "failed to get node info")
		} else if info.Value != content.value {
			return &empty.Empty{}, errors.Errorf("expected value %q for field %q, but got %q", content.value, fieldName, info.Value)
		}
	}
	return &empty.Empty{}, nil
}

// setupAndVerifyProxyValues defines the proxy field and its value.
type setupAndVerifyProxyValues map[string]struct {
	node  *nodewith.Finder
	value string
}

func (s *ProxySetupAndVerifyService) fetchProxyFieldAndValues(req *network.ProxyValuesRequest) setupAndVerifyProxyValues {
	return setupAndVerifyProxyValues{
		"http host":  {ossettings.HTTPHostTextField, req.HttpHost},
		"http port":  {ossettings.HTTPPortTextField, req.HttpPort},
		"https host": {ossettings.HTTPSHostTextField, req.HttpsHost},
		"https port": {ossettings.HTTPSPortTextField, req.HttpsPort},
		"socks host": {ossettings.SocksHostTextField, req.SocksHost},
		"socks port": {ossettings.SocksPortTextField, req.SocksPort},
	}
}

// cleanupProxy cleans up network settings by calling ResetShill() abd combines
// error messages into a single error.
func (s *ProxySetupAndVerifyService) cleanupProxy(ctx context.Context) error {
	if errs := shill.ResetShill(ctx); len(errs) > 0 {

		var errMessage string
		for _, e := range errs {
			errMessage = errMessage + e.Error()
		}
		return errors.Errorf("failed to reset shill: %s", errMessage)
	}
	return nil
}
