// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package camera

import (
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strconv"

	"github.com/golang/protobuf/ptypes/empty"
	"golang.org/x/net/context"
	"google.golang.org/grpc"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/testexec"
	"chromiumos/tast/local/upstart"
	cameraboxpb "chromiumos/tast/services/cros/camerabox"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddService(&testing.Service{
		Register: func(srv *grpc.Server, s *testing.ServiceState) {
			cameraboxpb.RegisterChartServiceServer(srv, &ChartService{s: s})
		},
	})
}

type ChartService struct {
	s *testing.ServiceState
	// cr displays chart on its chrome tab.
	cr *chrome.Chrome
	// conn controls the chrome tab displaying chart.
	conn *chrome.Conn
	// dir is the directory saving chart files.
	dir string
	// server serves http page for chart to be displayed.
	server *httptest.Server
}

func (c *ChartService) initChrome(ctx context.Context) (retErr error) {
	if c.cr != nil {
		return nil
	}

	// Prepare chrome.
	cr, err := chrome.New(ctx, chrome.ExtraArgs())
	if err != nil {
		return errors.Wrap(err, "unable to new chrome")
	}
	c.cr = cr
	defer func() {
		if retErr != nil && c.cr != nil {
			c.cr.Close(ctx)
			c.cr = nil
		}
	}()

	// Close all other opened window.
	tconn, err := c.cr.TestAPIConn(ctx)
	if err != nil {
		return err
	}
	defer tconn.Close()

	if err := tconn.Call(ctx, nil, `(async () => {
		const windows = await tast.promisify(chrome.windows.getAll)(undefined)
		for (const {id} of windows) {
			await tast.promisify(chrome.windows.remove)(id);
		}
	})`); err != nil {
		return errors.Wrap(err, "failed to close all opened windows")
	}

	// Open a blank tab.
	conn, err := c.cr.NewConn(ctx, "")
	if err != nil {
		return errors.Wrap(err, "failed to connect to blank tab")
	}
	c.conn = conn

	// Fix display brightness.
	if err := upstart.StopJob(ctx, "powerd"); err != nil {
		return errors.Wrap(err, "failed to stop powerd")
	}
	defer func() {
		if retErr != nil {
			if err := upstart.StartJob(ctx, "powerd"); err != nil {
				testing.ContextLog(ctx, "Failed to restore powerd: ", err)
			}
		}
	}()
	if err := setBacklightBrightness(ctx, 96); err != nil {
		return err
	}

	return nil
}

func (c *ChartService) initHTTPServer() error {
	dir, err := ioutil.TempDir("", "chart_dir")
	if err != nil {
		return errors.Wrap(err, "failed to create directory for cached charts")
	}

	c.dir = dir
	c.server = httptest.NewServer(http.FileServer(http.Dir(c.dir)))
	return nil
}

func (c *ChartService) Send(ctx context.Context, req *cameraboxpb.SendRequest) (*empty.Empty, error) {
	if err := c.initHTTPServer(); err != nil {
		return nil, errors.Wrap(err, "failed to init http server for displaying charts")
	}

	path := filepath.Join(c.dir, req.Name)
	if err := ioutil.WriteFile(path, req.Content, 0644); err != nil {
		return nil, errors.Wrapf(err, "failed to write chart file %v", req.Name)
	}
	return &empty.Empty{}, nil
}

// setBacklightBrightness sets the backlight brightness.
func setBacklightBrightness(ctx context.Context, brightness uint) error {
	brightnessArg := "--set_brightness_percent=" + strconv.FormatUint(uint64(brightness), 10)
	if err := testexec.CommandContext(ctx, "backlight_tool", brightnessArg).Run(testexec.DumpLogOnError); err != nil {
		return errors.Wrapf(err, "unable to set backlight brightness to %v", brightness)
	}
	return nil
}

func (c *ChartService) Display(ctx context.Context, req *cameraboxpb.DisplayRequest) (*empty.Empty, error) {
	if err := c.initChrome(ctx); err != nil {
		return nil, errors.Wrap(err, "failed to initialize chrome")
	}

	url := c.server.URL + "/" + req.Name
	if err := c.conn.Navigate(ctx, url); err != nil {
		return nil, errors.Wrapf(err, "failed to open chart url %v", url)
	}

	return &empty.Empty{}, nil
}

func (c *ChartService) Cleanup(ctx context.Context, req *empty.Empty) (*empty.Empty, error) {
	restorePowerd := func() error {
		if err := upstart.StartJob(ctx, "powerd"); err != nil {
			return errors.Wrap(err, "failed to restore powerd")
		}
		return nil
	}
	closeChrome := func() error {
		if c.cr == nil {
			return nil
		}
		defer func() {
			c.cr = nil
			c.conn = nil
		}()
		if err := c.cr.Close(ctx); err != nil {
			return errors.Wrap(err, "failed to close chrome")
		}
		return nil
	}
	closeHTTPServer := func() error {
		if c.server == nil {
			return nil
		}
		defer func() {
			c.dir = ""
			c.server = nil
		}()
		c.server.Close()
		if err := os.RemoveAll(c.dir); err != nil {
			return errors.Wrapf(err, "failed to remove chart directory %v", c.dir)
		}
		return nil
	}

	var firstErr error
	for _, fn := range []func() error{
		restorePowerd,
		closeChrome,
		closeHTTPServer,
	} {
		if err := fn(); err != nil {
			if firstErr != nil {
				testing.ContextLog(ctx, "Failed in cleanup stage: ", err)
			} else {
				firstErr = err
			}
		}
	}

	if firstErr != nil {
		return nil, firstErr
	}
	return &empty.Empty{}, nil
}
