// Copyright 2022 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package network

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"net/http/httptest"
	"os"
	"time"

	"chromiumos/tast/ctxutil"
	"chromiumos/tast/fsutil"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/browser"
	"chromiumos/tast/local/chrome/browser/browserfixt"
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/chrome/uiauto/faillog"
	"chromiumos/tast/local/chrome/uiauto/filesapp"
	"chromiumos/tast/local/chrome/uiauto/nodewith"
	"chromiumos/tast/local/chrome/uiauto/role"
	"chromiumos/tast/local/input"
	"chromiumos/tast/testing"
)

// The root certificate for the test CA that was used to create client and website certificates below.
// Chrome will need to import it to trust that the website certificate is valid.
// Website server will need to use it to trust that the client certificate is valid.
const rootCertFileName = "cert_settings_page_root_cert.crt"

// Client certificate that will be used by Chrome to authenticate on the website.
// The password for it is "12345".
const clientCertFileName = "cert_settings_page_client_cert.p12"

// Certificate and key pair for the test website.
const websiteCertFileName = "cert_settings_page_website_cert.crt"
const websiteKeyFileName = "cert_settings_page_website_key.key"

func init() {
	testing.AddTest(&testing.Test{
		Func: CertSettingsPage,
		Desc: "Test that chrome://settings/certificates page can import and use client and CA certificates",
		Contacts: []string{
			"miersh@google.com",
			"chromeos-commercial-networking@google.com",
		},
		Attr:         []string{"group:mainline"},
		SoftwareDeps: []string{"chrome", "lacros"},
		Fixture:      "lacrosUI",
		Timeout:      5 * time.Minute,
		Data: []string{clientCertFileName, rootCertFileName,
			websiteCertFileName, websiteKeyFileName},
		Params: []testing.Param{{
			ExtraSoftwareDeps: []string{"lacros_stable"},
			ExtraAttr:         []string{"informational"},
			Val:               browser.TypeLacros,
		}},
	})
}

// copyToDownloads copies the test data file with `fileName` into the Downloads
// directory, so it can be picked from the file picker.
func copyToDownloads(s *testing.State, fileName string) {
	newPath := filesapp.DownloadPath + "/" + fileName
	err := fsutil.CopyFile(s.DataPath(fileName), newPath)
	if err != nil {
		s.Fatalf("Failed to move file %s: %v", fileName, err)
	}
	// Without this the test data files don't have enough permissions and Chrome
	// fails to open them.
	err = os.Chmod(newPath, 0664)
	if err != nil {
		log.Fatalf("Failed to change file permissions on file %s: %v", fileName, err)
	}
}

// importCACert copies the `fileName` test data file into the Downloads
// directory and uses uses the chrome://settings/certificates page to manually
// import it.
func importCACert(ctx context.Context, s *testing.State, ui *uiauto.Context) {
	const fileName = rootCertFileName
	copyToDownloads(s, fileName)

	const trustCheckboxText = "Trust this certificate for identifying websites"
	if err := uiauto.Combine("import CA cert",
		ui.LeftClick(nodewith.Name("Authorities").Role(role.Tab)),
		ui.WaitUntilExists(nodewith.Name("Authorities").ClassName("tab selected")),
		ui.LeftClick(nodewith.Name("Import").Role(role.Button)),
		ui.LeftClick(nodewith.Name(fileName).Role(role.StaticText)),
		ui.LeftClick(nodewith.Name("Open").Role(role.Button)),
		ui.LeftClick(nodewith.Name(trustCheckboxText).Role(role.CheckBox)),
		ui.LeftClick(nodewith.Name("OK").Role(role.Button)),
	)(ctx); err != nil {
		s.Fatal("Failed to import CA cert: ", err)
	}
	s.Log("Imported CA cert: ", fileName)
}

// importClientCert copies the client cert data file into the Downloads
// directory and uses the chrome://settings/certificates page to manually
// import it.
func importClientCert(ctx context.Context, s *testing.State, ui *uiauto.Context) {
	const fileName = clientCertFileName
	copyToDownloads(s, fileName)

	kb, err := input.Keyboard(ctx)
	if err != nil {
		s.Fatal("Failed to find keyboard: ", err)
	}

	passwordDialog := nodewith.Name("Enter your certificate password").Role(role.Dialog)
	passwordTextBox := nodewith.Role(role.TextField).Editable()
	if err := uiauto.Combine("import client cert",
		ui.LeftClick(nodewith.Name("Your certificates").Role(role.Tab)),
		ui.WaitUntilExists(nodewith.Name("Your certificates").ClassName("tab selected")),
		ui.LeftClick(nodewith.Name("Import and Bind").Role(role.Button)),
		ui.LeftClick(nodewith.Name(fileName).Role(role.StaticText)),
		ui.LeftClick(nodewith.Name("Open").Role(role.Button)),
		ui.LeftClick(passwordTextBox.Ancestor(passwordDialog)),
		kb.TypeAction("12345"),
		ui.LeftClick(nodewith.Name("OK").Role(role.Button)),
	)(ctx); err != nil {
		s.Fatal("Failed to import client certificate: ", err)
	}
	s.Log("Imported client cert: ", fileName)
}

// createWebsite creates a website that requires a client certificate from its
// clients. Its server certificate will not be accepted by default Chrome, so it
// also requires clients to use a special CA certificate.
func createWebsite(s *testing.State) *httptest.Server {
	handleRequest := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintln(w, "Hello, client")
	})
	testServer := httptest.NewUnstartedServer(handleRequest)

	websiteCert, err := tls.LoadX509KeyPair(s.DataPath(websiteCertFileName),
		s.DataPath(websiteKeyFileName))
	if err != nil {
		s.Fatal("Failed to load website cert: ", err)
	}

	rootCertPem, err := ioutil.ReadFile(s.DataPath(rootCertFileName))
	if err != nil {
		s.Fatal("Failed to read root cert: ", err)
	}
	rootCertPool := x509.NewCertPool()
	rootCertPool.AppendCertsFromPEM(rootCertPem)

	testServer.TLS = &tls.Config{
		// Requires all clients to present a valid client certificate.
		ClientAuth: tls.RequireAndVerifyClientCert,
		// The certificate that the website presents to the client, so the client can trust it.
		// The client must have a corresponding root certificate for this to work.
		Certificates: []tls.Certificate{websiteCert},
		// Root certificates that the website will use to validate client certificates.
		ClientCAs: rootCertPool,
	}

	testServer.StartTLS()

	return testServer
}

// createAndUseWebsite creates a website and makes Chrome to open it, tests that
// Chrome successfully imported client and CA certificates.
func createAndUseWebsite(ctx context.Context, s *testing.State,
	browser *browser.Browser, ui *uiauto.Context) {
	websiteServer := createWebsite(s)
	defer websiteServer.Close()

	websiteConn, err := browser.NewConn(ctx, "")
	if err != nil {
		s.Fatal("Failed to connect to Chrome: ", err)
	}
	defer websiteConn.Close()
	// A navigation with NewConn wouldn't work, because it cannot fully load the
	// page until the client cert is selected and hangs on that.
	websiteConn.Eval(ctx, "window.location.href = '"+websiteServer.URL+"';", nil)

	// If Chrome failed to use the imported CA cert, it will complain about the
	// website here, won't show the cert selection dialog and won't show the
	// "Hello, client" text.

	// The website requests a client certificate and if Chrome has any, it will
	// show a dialog to choose one. In this test scenario Chrome should have
	// exactly one client certificate, so just pressing "OK" is enough.
	ui.WaitUntilExists(nodewith.Name("Select a certificate").Role(role.Window))(ctx)
	if err := ui.LeftClick(nodewith.Name("OK").Role(role.Button))(ctx); err != nil {
		s.Fatal("Failed to click on Open")
	}

	// Verify that the website accepted the certificate and fully loaded.
	ui.WaitUntilExists(nodewith.Name("Hello, client"))(ctx)
	s.Log("SUCCESS: \"Hello, client\" found")
}

func CertSettingsPage(ctx context.Context, s *testing.State) {
	chrome := s.FixtValue().(chrome.HasChrome).Chrome()
	browserType := s.Param().(browser.Type)

	// Reserve ten seconds for cleanup.
	cleanupCtx := ctx
	ctx, cancel := ctxutil.Shorten(ctx, 10*time.Second)
	defer cancel()

	browser, closeBrowser, err := browserfixt.SetUp(ctx, s.FixtValue(), browserType)
	if err != nil {
		s.Fatal("Failed to set up browser: ", err)
	}
	defer closeBrowser(cleanupCtx)

	tconn, err := chrome.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to create Test API connection: ", err)
	}

	s.Logf("Opening a new tab in %v browser", browserType)
	conn, err := browser.NewConn(ctx, "chrome://settings/certificates")
	if err != nil {
		s.Fatalf("Failed to open a new tab in %v browser: %v", browserType, err)
	}
	defer conn.Close()

	ui := uiauto.New(tconn)
	defer faillog.DumpUITreeOnError(cleanupCtx, s.OutDir(), s.HasError, tconn)

	importClientCert(ctx, s, ui)
	importCACert(ctx, s, ui)
	// After importing, the certs can take a bit of time to start working.
	testing.Sleep(ctx, 1*time.Second)

	createAndUseWebsite(ctx, s, browser, ui)
}
