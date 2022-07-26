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
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"chromiumos/tast/common/testexec"
	"chromiumos/tast/ctxutil"
	"chromiumos/tast/errors"
	"chromiumos/tast/fsutil"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/browser"
	"chromiumos/tast/local/chrome/browser/browserfixt"
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/chrome/uiauto/faillog"
	"chromiumos/tast/local/chrome/uiauto/nodewith"
	"chromiumos/tast/local/chrome/uiauto/role"
	"chromiumos/tast/local/cryptohome"
	"chromiumos/tast/local/input"
	"chromiumos/tast/local/policyutil"
	"chromiumos/tast/local/sysutil"
	"chromiumos/tast/testing"
)

// The root certificate for the test CA that was used to create client and website certificates below.
// Chrome will need to import it to trust that the website certificate is valid.
// Website server will need to use it to trust that the client certificate is valid.
const rootCertFileName = "cert_settings_page_root_cert.crt"

// Client certificate and key that will be used by Chrome to authenticate on the website.
// It is in a PKCS#12 format because that's what chrome supports for importing client certificates.
const clientCertFileName = "cert_settings_page_client_cert.p12"

// The password for the client cert PCKS#12 archive.
const clientCertFilePassword = "12345"

// Certificate and key pair (both in PEM format) for the test website.
const websiteCertFileName = "cert_settings_page_website_cert.crt"
const websiteKeyFileName = "cert_settings_page_website_key.key"

const pageLoadedRegex = ".*WEBSITE_LOADED.*"

// The ERR_BAD_SSL_CLIENT_AUTH_CERT is the actual correct error. For some reason Chrome
// also can return ERR_SOCKET_NOT_CONNECTED which effectively leads to the same end result
// (Chrome fails to open a page), so we also accept it.
const connectionErrorRegex = ".*(ERR_SOCKET_NOT_CONNECTED|ERR_BAD_SSL_CLIENT_AUTH_CERT).*"

const caInvalidErrorRegex = ".*ERR_CERT_AUTHORITY_INVALID.*"

// The message on the website that indicates that it successfully loaded.
const websiteGreeting = "WEBSITE_LOADED"

func init() {
	testing.AddTest(&testing.Test{
		Func:         CertSettingsPage,
		LacrosStatus: testing.LacrosVariantExists,
		Desc:         "Test that chrome://settings/certificates page can import and use client and CA certificates",
		Contacts: []string{
			"miersh@google.com",
			"chromeos-commercial-networking@google.com",
		},
		Attr:         []string{"group:mainline"},
		SoftwareDeps: []string{"chrome", "lacros"},
		Fixture:      "lacros",
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

// closeCurrentPage presses "ctrl+w" to close the current active window / tab.
func closeCurrentPage(ctx context.Context, s *testing.State) {
	kb, err := input.Keyboard(ctx)
	if err != nil {
		s.Fatal("Failed to find keyboard: ", err)
	}
	defer kb.Close()

	if err := kb.Accel(ctx, "ctrl+w"); err != nil {
		s.Fatal("Failed to close the page: ", err)
	}
}

// copyToDownloads copies the test data file with `fileName` into the Downloads
// directory, so it can be picked from the ChromeOS file picker.
func copyToDownloads(s *testing.State, downloadsPath, fileName string) {
	newPath := filepath.Join(downloadsPath, fileName)
	err := fsutil.CopyFile(s.DataPath(fileName), newPath)
	if err != nil {
		s.Fatalf("Failed to move file %s: %v", fileName, err)
	}
	// Without this the test data files don't have enough permissions and Chrome
	// fails to open them.
	err = os.Chown(newPath, int(sysutil.ChronosUID), int(sysutil.ChronosGID))
	if err != nil {
		s.Fatalf("Failed to chown file %s: %v", fileName, err)
	}
}

// importCACert copies the `fileName` test data file into the Downloads
// directory and uses the Import button on the chrome://settings/certificates
// page to manually import it.
func importCACert(ctx context.Context, s *testing.State, ui *uiauto.Context, downloadsPath string) {
	const fileName = rootCertFileName
	copyToDownloads(s, downloadsPath, fileName)

	const trustCheckboxText = "Trust this certificate for identifying websites"
	if err := uiauto.Combine("import CA cert",
		ui.DoDefault(nodewith.Name("Authorities").Role(role.Tab)),
		ui.WaitUntilExists(nodewith.Name("Authorities").ClassName("tab selected")),
		ui.DoDefault(nodewith.Name("Import").Role(role.Button)),
		ui.DoDefault(nodewith.Name(fileName).Role(role.StaticText)),
		ui.WaitUntilExists(nodewith.Name("Open").Role(role.Button).State("focusable", true)),
		ui.DoDefault(nodewith.Name("Open").Role(role.Button)),
		ui.WaitUntilExists(nodewith.Name(trustCheckboxText).Role(role.CheckBox)),
		ui.DoDefault(nodewith.Name(trustCheckboxText).Role(role.CheckBox)),
		ui.WaitUntilExists(nodewith.Name("OK").Role(role.Button)),
		ui.DoDefault(nodewith.Name("OK").Role(role.Button)),
	)(ctx); err != nil {
		s.Fatal("Failed to import CA cert: ", err)
	}
	s.Log("Imported CA cert: ", fileName)
}

// importClientCert copies the client cert data file into the Downloads
// directory and uses the Import and Bind button on the
// chrome://settings/certificates page to manually import it.
func importClientCert(ctx context.Context, s *testing.State, ui *uiauto.Context, downloadsPath string) {
	const fileName = clientCertFileName
	copyToDownloads(s, downloadsPath, fileName)

	kb, err := input.Keyboard(ctx)
	if err != nil {
		s.Fatal("Failed to find keyboard: ", err)
	}
	defer kb.Close()

	passwordDialog := nodewith.Name("Enter your certificate password").Role(role.Dialog)
	passwordTextBox := nodewith.Role(role.TextField).Editable()
	if err := uiauto.Combine("import client cert",
		ui.DoDefault(nodewith.Name("Your certificates").Role(role.Tab)),
		ui.WaitUntilExists(nodewith.Name("Your certificates").ClassName("tab selected")),
		ui.DoDefault(nodewith.Name("Import and Bind").Role(role.Button)),
		ui.DoDefault(nodewith.Name(fileName).Role(role.StaticText)),
		ui.WaitUntilExists(nodewith.Name("Open").Role(role.Button).State("focusable", true)),
		ui.DoDefault(nodewith.Name("Open").Role(role.Button)),
		ui.DoDefault(passwordTextBox.Ancestor(passwordDialog)),
		kb.TypeAction(clientCertFilePassword),
		ui.DoDefault(nodewith.Name("OK").Role(role.Button)),
	)(ctx); err != nil {
		s.Fatal("Failed to import client certificate: ", err)
	}
	s.Log("Imported client cert: ", fileName)
}

// waitForClientCert calls pkcs11-tool in a loop to determine when the client
// certificate gets propagated into chaps (and can be actually used by ChromeOS).
// This test assumes that the client certificate was imported last, so when it
// is ready, all the certificates should be usable.
func waitForClientCert(ctx context.Context, s *testing.State) {
	// Wait until the certificate is installed.
	if err := testing.Poll(ctx, func(ctx context.Context) error {
		// The argument "--slot 1" means "use user slot only". That's where Import
		// and Bind is supposed to place the cert.
		out, err := testexec.CommandContext(ctx,
			"pkcs11-tool", "--module", "libchaps.so", "--slot", "1", "--list-objects").Output()
		if err != nil {
			return errors.Wrap(err, "failed to get certificate list")
		}
		outStr := string(out)

		// Look for the org name of the `clientCertFileName`.
		if !strings.Contains(outStr, "TEST_CLIENT_ORG") {
			return errors.New("certificate not installed")
		}

		return nil

	}, nil); err != nil {
		s.Fatal("Could not verify that client certificate was installed: ", err)
	}
}

// createWebsite creates a website that requires a client certificate from its
// clients. Its server certificate will not be accepted by default Chrome, so it
// also requires clients to use a special CA certificate.
func createWebsite(s *testing.State) *httptest.Server {
	handleRequest := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintln(w, websiteGreeting)
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

// useWebsite attempts to open `website` in Chrome. If `expectCertPopup` is true,
// the function will wait for and handle the cert selection window. `expectedText`
// specifies what text should be seen on the page (for both successful and
// failed page loads).
func useWebsite(ctx context.Context, s *testing.State, browser *browser.Browser,
	ui *uiauto.Context, website *httptest.Server, expectCertPopup bool,
	expectedText string) error {

	websiteConn, err := browser.NewConn(ctx, "")
	if err != nil {
		return err
	}
	defer websiteConn.Close()

	// A navigation with NewConn wouldn't work, because it cannot fully load the
	// page until the client cert is selected and hangs on that.
	websiteConn.Eval(ctx, "window.location.href = '"+website.URL+"';", nil)

	if expectCertPopup {
		if err := ui.WaitUntilExists(nodewith.Name("Select a certificate").Role(role.Window))(ctx); err != nil {
			return err
		}

		if err := ui.LeftClick(nodewith.Name("OK").Role(role.Button))(ctx); err != nil {
			return err
		}
	}

	if err := ui.WaitUntilExists(nodewith.NameRegex(regexp.MustCompile(expectedText)).First())(ctx); err != nil {
		return err
	}
	return nil
}

// createAndUseWebsite creates a new website that requires the client and the CA
// certs and
func createAndUseWebsite(ctx context.Context, s *testing.State,
	browser *browser.Browser, ui *uiauto.Context, expectCertPopup bool, expectedText string) {

	var loopErr error

	// It can take a bit of time for the imported certs to propagate everywhere and
	// start working. Therefore try several times until the expected result is found.
	for i := 0; i < 3; i++ {
		website := createWebsite(s)
		defer website.Close()

		loopErr = useWebsite(ctx, s, browser, ui, website, expectCertPopup, expectedText)
		closeCurrentPage(ctx, s)
		if loopErr == nil {
			break
		}
	}

	if loopErr != nil {
		s.Fatal("createAndUseWebsite failed: ", loopErr)
	}
}

// useSystemSettings opens a system settings window and checks that the client cert
// is selectable there.
func useSystemSettings(ctx context.Context, s *testing.State,
	chrome *chrome.Chrome, tconn *chrome.TestConn) {
	if err := policyutil.CheckCertificateVisibleInSystemSettings(ctx, tconn, chrome, "TEST_CA_ORG"); err != nil {
		s.Fatal("Failed to select client certificate in system settings: ", err)
	}
	closeCurrentPage(ctx, s)
	s.Log("Client cert is usable in system settings")
}

// deleteClientCert uses the Chrome's cert settings page to delete the client cert.
func deleteClientCert(ctx context.Context, s *testing.State, ui *uiauto.Context) {
	if err := uiauto.Combine("delete CA cert",
		ui.WaitUntilExists(nodewith.Name("org-TEST_CLIENT_ORG").First()),
		ui.DoDefault(nodewith.Name("Show certificates for organization").Role(role.Button)),
		ui.DoDefault(nodewith.Name("More actions").Role(role.Button)),
		ui.DoDefault(nodewith.Name("Delete").Role(role.MenuItem)),
		ui.WaitUntilExists(nodewith.Name("OK").Role(role.Button)),
		ui.DoDefault(nodewith.Name("OK").Role(role.Button)),
		ui.WaitUntilGone(nodewith.Name("org-TEST_CLIENT_ORG")),
	)(ctx); err != nil {
		s.Fatal("Failed to delete CA cert: ", err)
	}
}

func deleteCACert(ctx context.Context, s *testing.State, ui *uiauto.Context) {
	kb, err := input.Keyboard(ctx)
	if err != nil {
		s.Fatal("Failed to find keyboard: ", err)
	}
	defer kb.Close()

	caCertOrg := nodewith.Name("org-TEST_CA_ORG").Role(role.StaticText)
	if err := uiauto.Combine("delete CA cert",
		ui.DoDefault(nodewith.Name("Authorities").Role(role.Tab)),
		ui.WaitUntilExists(nodewith.Name("Authorities").ClassName("tab selected")),
		ui.MakeVisible(caCertOrg),
		ui.LeftClick(caCertOrg),
	)(ctx); err != nil {
		s.Fatal("Failed to delete CA cert: ", err)
	}

	// The UI tree for these elements is not very convenient.
	// Use keyboard to navigate.
	if err := kb.Accel(ctx, "tab"); err != nil {
		s.Fatal("Failed to use keyboard: ", err)
	}
	if err := kb.Accel(ctx, "enter"); err != nil {
		s.Fatal("Failed to use keyboard: ", err)
	}

	caCert := nodewith.Name("TEST_CA_ORG").Role(role.StaticText)
	if err := uiauto.Combine("delete CA cert",
		ui.WaitUntilExists(caCert),
		ui.DoDefault(caCert),
	)(ctx); err != nil {
		s.Fatal("Failed to delete CA cert: ", err)
	}

	if err := kb.Accel(ctx, "tab"); err != nil {
		s.Fatal("Failed to use keyboard: ", err)
	}
	if err := kb.Accel(ctx, "enter"); err != nil {
		s.Fatal("Failed to use keyboard: ", err)
	}

	deleteButton := nodewith.Name("Delete").Role(role.MenuItem)
	okButton := nodewith.Name("OK").Role(role.Button)
	if err := uiauto.Combine("delete CA cert",
		ui.WaitUntilExists(deleteButton),
		ui.DoDefault(deleteButton),
		ui.WaitUntilExists(nodewith.NameContaining("Delete CA certificate").First()),
		ui.WaitUntilExists(okButton),
		ui.DoDefault(okButton),
		ui.WaitUntilGone(nodewith.Name("TEST_CA_ORG")),
	)(ctx); err != nil {
		s.Fatal("Failed to delete CA cert: ", err)
	}
}

func CertSettingsPage(ctx context.Context, s *testing.State) {
	cr := s.FixtValue().(chrome.HasChrome).Chrome()
	browserType := s.Param().(browser.Type)

	// Reserve ten seconds for cleanup.
	cleanupCtx := ctx
	ctx, cancel := ctxutil.Shorten(ctx, 10*time.Second)
	defer cancel()

	browser, closeBrowser, err := browserfixt.SetUp(ctx, cr, browserType)
	if err != nil {
		s.Fatal("Failed to set up browser: ", err)
	}
	defer closeBrowser(cleanupCtx)

	tconn, err := cr.TestAPIConn(ctx)
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
	defer faillog.DumpUITreeWithScreenshotOnError(cleanupCtx, s.OutDir(), s.HasError, cr, "ui_tree")

	downloadsPath, err := cryptohome.DownloadsPath(ctx, cr.NormalizedUser())
	if err != nil {
		s.Fatal("Failed to get user's Download path: ", err)
	}

	// Try opening a website without any certs, that should fail with a CA error.
	createAndUseWebsite(ctx, s, browser, ui, false /*expectCertPopup*/, caInvalidErrorRegex)

	// Import CA and client certs.
	importCACert(ctx, s, ui, downloadsPath)
	importClientCert(ctx, s, ui, downloadsPath)
	waitForClientCert(ctx, s)

	// Try to open the website again, this time it should succeed.
	createAndUseWebsite(ctx, s, browser, ui, true /*expectCertPopup*/, pageLoadedRegex)
	// Also check that client cert is usable in system settings.
	useSystemSettings(ctx, s, cr, tconn)

	// Delete the client cert and check that now the website rejects the connection.
	deleteClientCert(ctx, s, ui)
	createAndUseWebsite(ctx, s, browser, ui, false /*expectCertPopup*/, connectionErrorRegex)

	// Delete the CA cert and check that Chrome gets the CA error again.
	deleteCACert(ctx, s, ui)
	createAndUseWebsite(ctx, s, browser, ui, false /*expectCertPopup*/, caInvalidErrorRegex)
}
