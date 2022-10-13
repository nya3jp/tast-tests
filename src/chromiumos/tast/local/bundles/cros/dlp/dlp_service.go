// Copyright 2022 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package dlp

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
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
	var opts []chrome.Option

	if req.EnableLacros {
		req.EnabledFeatures += ", LacrosSupport"
	}

	opts = append(opts, chrome.GAIAEnterpriseEnroll(chrome.Creds{User: req.Username, Pass: req.Password}))
	opts = append(opts, chrome.GAIALogin(chrome.Creds{User: req.Username, Pass: req.Password}))
	opts = append(opts, chrome.EnableFeatures(req.EnabledFeatures))
	opts = append(opts, chrome.DMSPolicy(req.DmserverUrl))
	opts = append(opts, chrome.EncryptedReportingAddr(fmt.Sprintf("%v/record", req.ReportingServerUrl)))
	opts = append(opts, chrome.CustomLoginTimeout(chrome.EnrollmentAndLoginTimeout))

	var lcfg *lacrosfixt.Config
	var bt browser.Type
	if req.EnableLacros {
		bt = browser.TypeLacros
		lcfg = lacrosfixt.NewConfig(lacrosfixt.Mode(lacros.NotSpecified), lacrosfixt.Selection(lacros.NotSelected))
	} else {
		bt = browser.TypeAsh
	}

	cr, err := browserfixt.NewChrome(ctx, bt, lcfg, opts...)
	if err != nil {
		return nil, errors.Wrap(err, "failed to start chrome")
	}

	service.chrome = cr
	return &empty.Empty{}, nil
}

// ClipboardCopyPaste performs a copy and paste action.
func (service *DataLeakPreventionService) ClipboardCopyPaste(ctx context.Context, req *pb.ActionRequest) (_ *empty.Empty, retErr error) {

	baseDir := "/tmp"

	// Create an html page with some text.
	textFilename := "text.html"
	textContent := []byte("<!DOCTYPE html><html lang='en'><head><meta charset='utf-8'><title>Random Text 1</title></head><body>Sample text about random things.</body></html>")
	if err := os.WriteFile(baseDir+"/"+textFilename, textContent, 0644); err != nil {
		return &empty.Empty{}, errors.Wrap(err, "failed write a file")
	}

	// Create an html page with an input box.
	inputFilename := "input.html"
	inputContent := []byte("<!DOCTYPE html><html lang='en'><head><meta charset='utf-8'><title>Editable Text Box</title></head><body><textarea aria-label='textarea' rows='1' cols='100'></textarea></body></html>")
	if err := os.WriteFile(baseDir+"/"+inputFilename, inputContent, 0644); err != nil {
		return &empty.Empty{}, errors.Wrap(err, "failed write a file")
	}

	sourceServer := httptest.NewServer(http.FileServer(http.Dir(baseDir)))
	defer sourceServer.Close()

	destServer := httptest.NewServer(http.FileServer(http.Dir(baseDir)))
	defer destServer.Close()

	browserType := browser.TypeAsh
	if req.BrowserType == pb.BrowserType_LACROS {
		browserType = browser.TypeLacros
	}

	br, closeBrowser, err := browserfixt.SetUp(ctx, service.chrome, browserType)
	if err != nil {
		return &empty.Empty{}, errors.Wrap(err, "failed to open the browser")
	}
	defer closeBrowser(ctx)

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

	textBoxNode := nodewith.Name("textarea").Role(role.TextField).State(state.Editable, true).First()
	if err := uiauto.Combine("Pasting into text box",
		ui.WaitUntilExists(textBoxNode.Visible()),
		ui.LeftClick(textBoxNode),
		ui.WaitUntilExists(textBoxNode.Focused()),
		keyboard.AccelAction("Ctrl+V"),
	)(ctx); err != nil {
		return &empty.Empty{}, errors.Wrap(err, "failed to paste into text box")
	}

	parsedSourceURL, _ := url.Parse(sourceServer.URL)

	if req.Mode == pb.Mode_WARN_CANCEL {
		bubbleClass, err := clipboard.WarnBubble(ctx, ui, parsedSourceURL.Hostname())
		if err != nil {
			return &empty.Empty{}, errors.Wrap(err, "expected notification but found an error")
		}
		cancelButton := nodewith.Name("Cancel").Role(role.Button).Ancestor(bubbleClass)
		if err := ui.LeftClick(cancelButton)(ctx); err != nil {
			return &empty.Empty{}, errors.Wrap(err, "failed to click the cancel button")
		}
	} else if req.Mode == pb.Mode_WARN_PROCEED {
		bubbleClass, err := clipboard.WarnBubble(ctx, ui, parsedSourceURL.Hostname())
		if err != nil {
			return &empty.Empty{}, errors.Wrap(err, "expected notification but found an error")
		}
		pasteButton := nodewith.Name("Paste anyway").Role(role.Button).Ancestor(bubbleClass)
		if err := ui.LeftClick(pasteButton)(ctx); err != nil {
			return &empty.Empty{}, errors.Wrap(err, "failed to click the paste button")
		}
	}

	return &empty.Empty{}, nil

}

// Print performs a print action.
func (service *DataLeakPreventionService) Print(ctx context.Context, req *pb.ActionRequest) (_ *empty.Empty, retErr error) {

	keyboard, err := input.VirtualKeyboard(ctx)
	if err != nil {
		return &empty.Empty{}, errors.Wrap(err, "failed to get keyboard")
	}
	defer keyboard.Close()

	browserType := browser.TypeAsh
	if req.BrowserType == pb.BrowserType_LACROS {
		browserType = browser.TypeLacros
	}

	br, closeBrowser, err := browserfixt.SetUp(ctx, service.chrome, browserType)
	if err != nil {
		return &empty.Empty{}, errors.Wrap(err, "failed to open the browser")
	}
	defer closeBrowser(ctx)

	// Create an html page with some text.
	baseDir := "/tmp"
	textFilename := "text.html"
	textContent := []byte("<!DOCTYPE html><html lang='en'><head><meta charset='utf-8'><title>Random Text 1</title></head><body>Sample text about random things.</body></html>")
	if err := os.WriteFile(baseDir+"/"+textFilename, textContent, 0644); err != nil {
		return &empty.Empty{}, errors.Wrap(err, "failed write a file")
	}

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

	// Test printing using hotkey (Ctrl + P).
	if err := keyboard.Accel(ctx, "Ctrl+P"); err != nil {
		return &empty.Empty{}, errors.Wrap(err, "failed to type printing hotkey")
	}

	if req.Mode == pb.Mode_WARN_PROCEED {
		if err := keyboard.Accel(ctx, "Enter"); err != nil {
			return &empty.Empty{}, errors.Wrap(err, "failed to hit Enter")
		}
	} else if req.Mode == pb.Mode_WARN_CANCEL {
		if err := keyboard.Accel(ctx, "Esc"); err != nil {
			return &empty.Empty{}, errors.Wrap(err, "failed to hit Esc")
		}
	}

	return &empty.Empty{}, nil

}

// Screenshot takes a screenshot.
func (service *DataLeakPreventionService) Screenshot(ctx context.Context, req *pb.ActionRequest) (_ *empty.Empty, retErr error) {

	keyboard, err := input.VirtualKeyboard(ctx)
	if err != nil {
		return &empty.Empty{}, errors.Wrap(err, "failed to get keyboard")
	}
	defer keyboard.Close()

	browserType := browser.TypeAsh
	if req.BrowserType == pb.BrowserType_LACROS {
		browserType = browser.TypeLacros
	}

	br, closeBrowser, err := browserfixt.SetUp(ctx, service.chrome, browserType)
	if err != nil {
		return &empty.Empty{}, errors.Wrap(err, "failed to open the browser")
	}
	defer closeBrowser(ctx)

	// Create an html page with some text.
	baseDir := "/tmp"
	textFilename := "text.html"
	textContent := []byte("<!DOCTYPE html><html lang='en'><head><meta charset='utf-8'><title>Random Text 1</title></head><body>Sample text about random things.</body></html>")
	if err := os.WriteFile(baseDir+"/"+textFilename, textContent, 0644); err != nil {
		return &empty.Empty{}, errors.Wrap(err, "failed write a file")
	}

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

	// Take a screenshot using hotkey (Ctrl+F5)
	if err := keyboard.Accel(ctx, "Ctrl+F5"); err != nil {
		return &empty.Empty{}, errors.Wrap(err, "failed to type screenshot hotkey")
	}

	if req.Mode == pb.Mode_WARN_PROCEED {
		// Hit Enter, which is equivalent to clicking on the "Capture anyway" button.
		if err := keyboard.Accel(ctx, "Enter"); err != nil {
			return &empty.Empty{}, errors.Wrap(err, "failed to hit Enter")
		}
	} else if req.Mode == pb.Mode_WARN_CANCEL {
		// Hit Esc, which is equivalent to clicking on the "Cancel" button.
		if err := keyboard.Accel(ctx, "Esc"); err != nil {
			return &empty.Empty{}, errors.Wrap(err, "failed to hit Esc")
		}
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
