// Copyright 2022 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package dlp

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"time"

	"github.com/golang/protobuf/ptypes/empty"
	"google.golang.org/grpc"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/bundles/cros/dlp/clipboard"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/browser"
	"chromiumos/tast/local/chrome/browser/browserfixt"
	"chromiumos/tast/local/chrome/lacros"
	"chromiumos/tast/local/chrome/lacros/lacrosfixt"
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/chrome/uiauto/nodewith"
	"chromiumos/tast/local/chrome/uiauto/role"
	"chromiumos/tast/local/chrome/uiauto/state"
	"chromiumos/tast/local/chrome/webutil"
	"chromiumos/tast/local/cryptohome"
	"chromiumos/tast/local/input"
	"chromiumos/tast/local/screenshot"
	pb "chromiumos/tast/services/cros/dlp"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddService(&testing.Service{
		Register: func(srv *grpc.Server, s *testing.ServiceState) {
			pb.RegisterDataLeakPreventionServiceServer(srv, &DataLeakPreventionService{s: s})
		},
	})
}

type DataLeakPreventionService struct {
	s      *testing.ServiceState
	chrome *chrome.Chrome
}

// EnrollAndLogin enrolls the device and logs in with the provided account credentials.
func (service *DataLeakPreventionService) EnrollAndLogin(ctx context.Context, req *pb.EnrollAndLoginRequest) (_ *empty.Empty, retErr error) {

	opts := []chrome.Option{
		chrome.GAIAEnterpriseEnroll(chrome.Creds{User: req.Username, Pass: req.Password}),
		chrome.GAIALogin(chrome.Creds{User: req.Username, Pass: req.Password}),
		chrome.EnableFeatures(req.EnabledFeatures),
		chrome.DMSPolicy(req.DmserverUrl),
		chrome.EncryptedReportingAddr(fmt.Sprintf("%v/record", req.ReportingServerUrl)),
		chrome.CustomLoginTimeout(chrome.EnrollmentAndLoginTimeout),
	}

	bt := browser.TypeAsh
	var lcfg *lacrosfixt.Config
	if req.EnableLacros {
		bt = browser.TypeLacros
		lcfg = lacrosfixt.NewConfig(lacrosfixt.Mode(lacros.NotSpecified), lacrosfixt.Selection(lacros.NotSelected))
	}

	cr, err := browserfixt.NewChrome(ctx, bt, lcfg, opts...)
	if err != nil {
		return nil, errors.Wrap(err, "failed to start chrome")
	}

	service.chrome = cr
	return &empty.Empty{}, nil
}

// StopChrome disconnects from Chrome.
func (service *DataLeakPreventionService) StopChrome(ctx context.Context, req *empty.Empty) (*empty.Empty, error) {
	var lastErr error

	if service.chrome == nil {
		return nil, errors.New("no active Chrome instance")
	}

	if err := service.chrome.Close(ctx); err != nil {
		testing.ContextLog(ctx, "Failed to close Chrome: ", err)
		lastErr = errors.Wrap(err, "failed to close Chrome")
	}
	service.chrome = nil

	return &empty.Empty{}, lastErr
}

// createHTMLTextPage creates an HTML page with some text.
func createHTMLTextPage(path string) error {
	textContent := []byte("<!DOCTYPE html><html lang='en'><head><meta charset='utf-8'><title>Random Text</title></head><body>Sample text about random things.</body></html>")
	if err := os.WriteFile(path, textContent, 0644); err != nil {
		return errors.Wrap(err, "failed write a file")
	}
	return nil
}

// createHTMLTextAreaPage creates an HTML page with a text area element.
func createHTMLTextAreaPage(path string) (*nodewith.Finder, error) {
	inputContent := []byte("<!DOCTYPE html><html lang='en'><head><meta charset='utf-8'><title>Editable Text Box</title></head><body><textarea aria-label='textarea' rows='1' cols='100'></textarea></body></html>")
	if err := os.WriteFile(path, inputContent, 0644); err != nil {
		return nil, errors.Wrap(err, "failed write a file")
	}
	textAreaNode := nodewith.Name("textarea").Role(role.TextField).State(state.Editable, true).First()
	return textAreaNode, nil
}

// setupBrowser returns a Browser instance for the given browser type and a closure to close the browser instance.
func setupBrowser(ctx context.Context, chrome *chrome.Chrome, browserType pb.BrowserType) (*browser.Browser, func(ctx context.Context) error, error) {
	bt := browser.TypeAsh
	if browserType == pb.BrowserType_LACROS {
		bt = browser.TypeLacros
	}

	br, closeBrowser, err := browserfixt.SetUp(ctx, chrome, bt)
	if err != nil {
		return nil, nil, errors.Wrap(err, "failed to open the browser")
	}

	return br, closeBrowser, nil
}

// ClipboardCopyPaste performs a copy and paste action.
func (service *DataLeakPreventionService) ClipboardCopyPaste(ctx context.Context, req *pb.ActionRequest) (_ *empty.Empty, retErr error) {

	baseDir := "/tmp"
	textFilename := "text.html"
	if err := createHTMLTextPage(filepath.Join(baseDir, textFilename)); err != nil {
		return &empty.Empty{}, errors.Wrap(err, "error while creating the HTML text page")
	}

	inputFilename := "input.html"
	textAreaNode, err := createHTMLTextAreaPage(filepath.Join(baseDir, inputFilename))
	if err != nil {
		return &empty.Empty{}, errors.Wrap(err, "error while creating the HTML input page")
	}

	br, closeBrowser, err := setupBrowser(ctx, service.chrome, req.BrowserType)
	if err != nil {
		return &empty.Empty{}, errors.Wrap(err, "error while setting up the browser")
	}
	defer closeBrowser(ctx)

	sourceServer := httptest.NewServer(http.FileServer(http.Dir(baseDir)))
	defer sourceServer.Close()

	sourceURL := sourceServer.URL + "/" + textFilename
	sourceConn, err := br.NewConn(ctx, sourceURL)
	if err != nil {
		return &empty.Empty{}, errors.Wrap(err, "failed to open the source page")
	}
	defer sourceConn.Close()

	if err := webutil.WaitForQuiescence(ctx, sourceConn, 10*time.Second); err != nil {
		return &empty.Empty{}, errors.Wrap(err, "failed to wait to achieve quiescence")
	}

	// Connect to Test API.
	tconn, err := service.chrome.TestAPIConn(ctx)
	if err != nil {
		return &empty.Empty{}, errors.Wrap(err, "failed to connect to test API")
	}

	keyboard, err := input.VirtualKeyboard(ctx)
	if err != nil {
		return &empty.Empty{}, errors.Wrap(err, "failed to get keyboard")
	}
	defer keyboard.Close()

	if err := tconn.WaitForExpr(ctx, "chrome.clipboard"); err != nil {
		return &empty.Empty{}, errors.Wrap(err, "failed to wait for chrome.clipboard API to become available")
	}

	if err := uiauto.Combine("copy all text from source website",
		keyboard.AccelAction("Ctrl+A"),
		keyboard.AccelAction("Ctrl+C"))(ctx); err != nil {
		return &empty.Empty{}, errors.Wrap(err, "failed to copy text from source browser")
	}

	copiedString, err := clipboard.GetClipboardContent(ctx, tconn)
	if err != nil {
		return &empty.Empty{}, errors.Wrap(err, "failed to get clipboard content")
	}

	destServer := httptest.NewServer(http.FileServer(http.Dir(baseDir)))
	defer destServer.Close()

	destURL := destServer.URL + "/" + inputFilename
	destConn, err := br.NewConn(ctx, destURL)
	if err != nil {
		return &empty.Empty{}, errors.Wrap(err, "failed to open the destination page")
	}
	defer destConn.Close()

	if err := webutil.WaitForQuiescence(ctx, destConn, 10*time.Second); err != nil {
		return &empty.Empty{}, errors.Wrap(err, "failed to wait to achieve quiescence")
	}

	ui := uiauto.New(tconn)

	if err := uiauto.Combine("Pasting into text box",
		ui.WaitUntilExists(textAreaNode.Visible()),
		ui.LeftClick(textAreaNode),
		ui.WaitUntilExists(textAreaNode.Focused()),
		keyboard.AccelAction("Ctrl+V"),
	)(ctx); err != nil {
		return &empty.Empty{}, errors.Wrap(err, "failed to paste into text box")
	}

	// We are only testing report mode, since we expect the text to be copied.
	if err := clipboard.CheckPastedContent(ctx, ui, copiedString); err != nil {
		return &empty.Empty{}, errors.Wrap(err, "failed to verify pasted content")
	}

	return &empty.Empty{}, nil
}

// Print performs a print action.
func (service *DataLeakPreventionService) Print(ctx context.Context, req *pb.ActionRequest) (_ *empty.Empty, retErr error) {

	baseDir := "/tmp"
	textFilename := "text.html"
	if err := createHTMLTextPage(filepath.Join(baseDir, textFilename)); err != nil {
		return &empty.Empty{}, errors.Wrap(err, "error while creating the HTML text page")
	}

	br, closeBrowser, err := setupBrowser(ctx, service.chrome, req.BrowserType)
	if err != nil {
		return &empty.Empty{}, errors.Wrap(err, "error while setting up the browser")
	}
	defer closeBrowser(ctx)

	server := httptest.NewServer(http.FileServer(http.Dir(baseDir)))
	defer server.Close()

	conn, err := br.NewConn(ctx, server.URL+"/"+textFilename)
	if err != nil {
		return &empty.Empty{}, errors.Wrap(err, "failed to open page")
	}
	defer conn.Close()

	if err := webutil.WaitForQuiescence(ctx, conn, 10*time.Second); err != nil {
		return &empty.Empty{}, errors.Wrap(err, "failed to wait to achieve quiescence")
	}

	keyboard, err := input.VirtualKeyboard(ctx)
	if err != nil {
		return &empty.Empty{}, errors.Wrap(err, "failed to get keyboard")
	}
	defer keyboard.Close()

	// Test printing using hotkey (Ctrl + P).
	if err := keyboard.Accel(ctx, "Ctrl+P"); err != nil {
		return &empty.Empty{}, errors.Wrap(err, "failed to type printing hotkey")
	}

	// Take a screenshot using hotkey (Ctrl+F5)
	if err := keyboard.Accel(ctx, "Ctrl+F5"); err != nil {
		return &empty.Empty{}, errors.Wrap(err, "failed to type screenshot hotkey")
	}

	downloadsPath, err := cryptohome.DownloadsPath(ctx, service.chrome.NormalizedUser())
	if err != nil {
		return &empty.Empty{}, errors.Wrap(err, "failed to retrieve user's Download path")
	}
	// Clean up previous screenshots.
	if err := screenshot.RemoveScreenshots(downloadsPath); err != nil {
		return &empty.Empty{}, errors.Wrap(err, "failed to remove screenshots")
	}

	return &empty.Empty{}, nil

}

// Screenshot takes a screenshot.
func (service *DataLeakPreventionService) Screenshot(ctx context.Context, req *pb.ActionRequest) (_ *empty.Empty, retErr error) {

	baseDir := "/tmp"
	textFilename := "text.html"
	if err := createHTMLTextPage(filepath.Join(baseDir, textFilename)); err != nil {
		return &empty.Empty{}, errors.Wrap(err, "error while creating the HTML text page")
	}

	br, closeBrowser, err := setupBrowser(ctx, service.chrome, req.BrowserType)
	if err != nil {
		return &empty.Empty{}, errors.Wrap(err, "error while setting up the browser")
	}
	defer closeBrowser(ctx)

	server := httptest.NewServer(http.FileServer(http.Dir(baseDir)))
	defer server.Close()

	conn, err := br.NewConn(ctx, server.URL+"/"+textFilename)
	if err != nil {
		return &empty.Empty{}, errors.Wrap(err, "failed to open page")
	}
	defer conn.Close()

	if err := webutil.WaitForQuiescence(ctx, conn, 10*time.Second); err != nil {
		return &empty.Empty{}, errors.Wrap(err, "failed to wait to achieve quiescence")
	}

	keyboard, err := input.VirtualKeyboard(ctx)
	if err != nil {
		return &empty.Empty{}, errors.Wrap(err, "failed to get keyboard")
	}
	defer keyboard.Close()

	// Take a screenshot using hotkey (Ctrl+F5)
	if err := keyboard.Accel(ctx, "Ctrl+F5"); err != nil {
		return &empty.Empty{}, errors.Wrap(err, "failed to type screenshot hotkey")
	}

	downloadsPath, err := cryptohome.DownloadsPath(ctx, service.chrome.NormalizedUser())
	if err != nil {
		return &empty.Empty{}, errors.Wrap(err, "failed to retrieve user's Download path")
	}
	// Clean up previous screenshots.
	if err := screenshot.RemoveScreenshots(downloadsPath); err != nil {
		return &empty.Empty{}, errors.Wrap(err, "failed to remove screenshots")
	}

	return &empty.Empty{}, nil

}
