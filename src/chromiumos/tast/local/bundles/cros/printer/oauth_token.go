// Copyright 2022 The ChromiumOS Authors.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package printer

import (
	"bytes"
	"context"
	"io/fs"
	"io/ioutil"
	"os"
	"path/filepath"
	"time"

	"chromiumos/tast/ctxutil"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/printing/lp"
	"chromiumos/tast/local/printing/printer"
	"chromiumos/tast/local/printing/usbprinter"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         OAuthToken,
		LacrosStatus: testing.LacrosVariantUnneeded,
		Desc:         "Tests that ensure the oauth token is passed to printer",
		Contacts:     []string{"nmuggli@google.com", "project-bolton@google.com"},
		Attr: []string{
			"group:mainline",
			"informational",
			"group:paper-io",
			"paper-io_printing",
		},
		Timeout:      2 * time.Minute,
		SoftwareDeps: []string{"cros_internal", "cups", "virtual_usb_printer"},
		Data:         []string{"to_print.pdf"},
		Fixture:      "virtualUsbPrinterModulesLoadedWithChromeLoggedIn",
	})
}

func OAuthToken(ctx context.Context, s *testing.State) {
	cleanupCtx := ctx
	ctx, cancel := ctxutil.Shorten(ctx, 5*time.Second)
	defer cancel()

	if err := printer.ResetCups(ctx); err != nil {
		s.Fatal("Failed to reset cupsd: ", err)
	}

	// Create the temp dir to store the HTTP headers.
	httpHeaderFiles := "printer.OAuthToken.httpHeaders"
	tmpDir, err := ioutil.TempDir("", httpHeaderFiles)
	if err != nil {
		s.Fatal("Failed to create temporary directory")
	}
	defer os.RemoveAll(tmpDir)

	printer, err := usbprinter.Start(ctx,
		usbprinter.WithIPPUSBDescriptors(),
		usbprinter.WithHTTPLogDirectory(tmpDir),
		usbprinter.WithGenericIPPAttributes(),
		usbprinter.WaitUntilConfigured())
	if err != nil {
		s.Fatal("Failed to attach virtual printer: ", err)
	}
	defer func(ctx context.Context) {
		if err := printer.Stop(ctx); err != nil {
			s.Error("Failed to stop virtual printer: ", err)
		}
	}(cleanupCtx)

	// Clean out the files in the tmp dir.  These will have requests that are
	// involved with printer setup, so we don't expect those to have the oauth
	// token in the http headers - we just want to check all requests after we
	// start our print job.
	if files, err := fs.Glob(os.DirFS(tmpDir), "*"); err != nil {
		s.Fatalf("Unable to read dir %s: %s", tmpDir, err)
	} else {
		for _, file := range files {
			deleteMe := filepath.Join(tmpDir, file)
			if err := os.Remove(deleteMe); err != nil {
				s.Fatalf("Unable to remove %s: %s", deleteMe, err)
			}
		}
	}

	printerName := printer.ConfiguredName
	oauthTokenString := "qwertyasdf1234="

	job, err := lp.CupsStartPrintJob(ctx, printerName,
		s.DataPath("to_print.pdf"), "-o",
		"chromeos-access-oauth-token="+oauthTokenString)
	if err != nil {
		s.Fatal("Failed to start printer: ", err)
	}

	s.Logf("Waiting for %s to complete", job)
	if err = testing.Poll(ctx, func(ctx context.Context) error {
		if done, err := lp.JobCompleted(ctx, printerName, job); err != nil {
			return err
		} else if !done {
			return errors.Errorf("Job %s is not done yet", job)
		}
		testing.ContextLogf(ctx, "Job %s is complete", job)
		return nil
	}, nil); err != nil {
		s.Fatal("Print job didn't complete: ", err)
	}

	// Look at all of our http header files and make sure they have the correct
	// oauth access token.
	if files, err := fs.Glob(os.DirFS(tmpDir), "*"); err != nil {
		s.Fatalf("Unable to read dir %s: %s", tmpDir, err)
	} else {
		for _, file := range files {
			fileToRead := filepath.Join(tmpDir, file)
			if data, err := ioutil.ReadFile(fileToRead); err != nil {
				s.Fatalf("Unable to read HTTP header file %s: %s", file, err)
			} else {
				if !(bytes.Contains(data,
					[]byte("Authorization: Bearer "+oauthTokenString+"\n"))) {
					s.Fatal("HTTP header does not contain correct oauth token")
				}
			}
		}
	}
}
