// Copyright 2022 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package network

import (
	"context"
	"path/filepath"
	"time"

	"github.com/golang/protobuf/ptypes/empty"
	"google.golang.org/grpc"

	"chromiumos/tast/ctxutil"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/bundles/cros/network/proxysettings"
	"chromiumos/tast/local/bundles/cros/network/shill"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/chrome/uiauto/faillog"
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

	if !req.ClearProxySettings {
		opts = append(opts, chrome.KeepState())
	} else {
		// TODO(b/242474992): Proxy values will be preserved across Chrome sessions
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
	outDir, ok := testing.ContextOutDir(ctx)
	if !ok {
		testing.ContextLog(ctx, "Failed to get output dir")
		return
	}
	faillog.DumpUITreeOnError(ctx, filepath.Join(outDir, "ProxySetupAndVerifyService_"+namePrefix), hasError, s.tconn)
}

// Setup sets up proxy values.
func (s *ProxySetupAndVerifyService) Setup(ctx context.Context, req *network.ProxyConfigs) (_ *empty.Empty, retErr error) {
	cleanupCtx := ctx
	ctx, cancel := ctxutil.Shorten(ctx, 10*time.Second)
	defer cancel()

	ps, err := proxysettings.CollectFromSigninScreen(ctx, s.tconn)
	if err != nil {
		return &empty.Empty{}, errors.Wrap(err, "failed to create proxy settings")
	}
	defer ps.Close(cleanupCtx, s.kb)
	defer s.dumpUITreeToFile(cleanupCtx, func() bool { return retErr != nil }, "ui_dump_setup_proxy")

	proxyValues := s.fetchProxyFieldAndValues(req)
	for _, pv := range proxyValues {
		if err := ps.Setup(ctx, s.cr, s.kb, pv); err != nil {
			return &empty.Empty{}, errors.Wrapf(err, "failed to setup the contents for proxy fields of %s", pv.HostName())
		}
	}

	return &empty.Empty{}, nil
}

// FetchConfigurations returns proxy hosts and ports.
func (s *ProxySetupAndVerifyService) FetchConfigurations(ctx context.Context, _ *empty.Empty) (_ *network.ProxyConfigs, retErr error) {
	cleanupCtx := ctx
	ctx, cancel := ctxutil.Shorten(ctx, 10*time.Second)
	defer cancel()

	ps, err := proxysettings.CollectFromSigninScreen(ctx, s.tconn)
	if err != nil {
		return nil, errors.Wrap(err, "failed to create proxy settings")
	}
	defer ps.Close(cleanupCtx, s.kb)
	defer s.dumpUITreeToFile(cleanupCtx, func() bool { return retErr != nil }, "ui_dump_verify_proxy")

	currentConfigs := &network.ProxyConfigs{}
	emptyConfigs := []*proxysettings.Config{
		{Protocol: proxysettings.HTTP},
		{Protocol: proxysettings.HTTPS},
		{Protocol: proxysettings.Socks},
	}

	for _, pv := range emptyConfigs {
		resultPv, err := ps.Content(ctx, pv)
		if err != nil {
			return nil, errors.Wrapf(err, "failed to get the contents for proxy fields of %s", pv.HostName())
		}

		switch resultPv.Protocol {
		case proxysettings.HTTP:
			currentConfigs.HttpHost = resultPv.Host
			currentConfigs.HttpPort = resultPv.Port
		case proxysettings.HTTPS:
			currentConfigs.HttpsHost = resultPv.Host
			currentConfigs.HttpsPort = resultPv.Port
		case proxysettings.Socks:
			currentConfigs.SocksHost = resultPv.Host
			currentConfigs.SocksPort = resultPv.Port
		default:
			return nil, errors.Errorf("unknown protocol: %s", resultPv.Protocol)
		}
	}

	return currentConfigs, nil
}

func (s *ProxySetupAndVerifyService) fetchProxyFieldAndValues(req *network.ProxyConfigs) []*proxysettings.Config {
	return []*proxysettings.Config{
		{
			Protocol: proxysettings.HTTP,
			Host:     req.HttpHost,
			Port:     req.HttpPort,
		},
		{
			Protocol: proxysettings.HTTPS,
			Host:     req.HttpsHost,
			Port:     req.HttpsPort,
		},
		{
			Protocol: proxysettings.Socks,
			Host:     req.SocksHost,
			Port:     req.SocksPort,
		},
	}
}

// cleanupProxy cleans up network settings by calling ResetShill() and combines
// error messages into a single error.
func (s *ProxySetupAndVerifyService) cleanupProxy(ctx context.Context) error {
	if errs := shill.ResetShill(ctx); len(errs) > 0 {
		for _, err := range errs {
			testing.ContextLog(ctx, "ResetShill error: ", err)
		}
		return errors.Wrap(errs[0], "failed to reset shill")
	}
	return nil
}
