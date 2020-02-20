// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package policy

import (
	"context"
	"fmt"
	"net/http"
	"time"

	//"chromiumos/tast/local/arc"
	"chromiumos/tast/local/policy"
	//"chromiumos/tast/common/policy"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/bundles/cros/policy/policyutil"
	"chromiumos/tast/local/bundles/cros/policy/pre"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/testing"
)

const (
	extensionID       = "gkjmnbdackjbjnahbilidcpgdegbdmhh"
	extensionURL      = "chrome-extension://" + extensionID + "/main.html"
	webstorePort      = ":8080"
	webstoreURL       = "http://127.0.0.1" + webstorePort
	devToolsAvailable = 1
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         ARCCorporateKeys,
		Desc:         "Verify ARC has access to Chomre OS corporate keys",
		Contacts:     []string{"edmanp@google.com", "arc-eng-muc@google.com"},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"android_p", "chrome"},
		Data:         []string{"platformKeysTestExtension.crx"},
		//Pre:          arc.Booted(),
		Pre:     pre.User,
		Timeout: 5 * time.Minute,
	})
}

func ARCCorporateKeys(ctx context.Context, s *testing.State) {
	cr := s.PreValue().(*pre.PreData).Chrome
	fdms := s.PreValue().(*pre.PreData).FakeDMS

	// Launch fake webstore HTTP server for test extension.
	go serveExtension(s.DataPath("platformKeysTestExtension.crx"))

	// Perform cleanup.
	if err := policyutil.ResetChrome(ctx, fdms, cr); err != nil {
		s.Fatal("Failed to clean up: ", err)
	}

	// Update policies.
	s.Log("Set user policies")
	policies := []policy.Policy{
		&policy.ExtensionInstallForcelist{Val: []string{extensionID + ";" + webstoreURL}},
		&policy.DeveloperToolsAvailability{Val: devToolsAvailable},
	}
	if err := policyutil.ServeAndRefresh(ctx, fdms, cr, policies); err != nil {
		s.Fatal("Failed to update policies: ", err)
	}

	// Workaround to ensure test extension is force installed.
	s.Log("Ensure platformKeys test extension is installed")
	ensureExtensionIsInstalled(ctx, cr)

	conn, err := cr.NewConn(ctx, extensionURL)
	if err != nil {
		s.Fatal("Could not connect to test extensions: ", err)
	}
	defer conn.Close()

	// Clicks on a button given its ID and waits till its operation has status OK.
	click := func(id string) {
		element := fmt.Sprintf(`document.getElementById(%q)`, id)
		status := fmt.Sprintf(`document.getElementById(%q).value`, id+"-error")
		err := conn.WaitForExpr(ctx, fmt.Sprintf(`%v !== null`, element))
		if err != nil {
			s.Fatalf("Could not find button with id %q: %v", id, err)
		}
		err = conn.Exec(ctx, fmt.Sprintf(`%v.click()`, element))
		if err != nil {
			s.Fatalf("Could not click button with id %q: %v", id, err)
		}
		err = conn.WaitForExpr(ctx, fmt.Sprintf(`%v.match('OK') !== null`, status))
		if err != nil {
			s.Fatalf("Status for %q is not OK: %v", id, err)
		}
	}

	s.Log("Generate and import a certificate")
	click("generate")
	click("create-cert")
	click("import-cert")
	click("list-certs")
}

func serveExtension(extensionPath string) {
	const webstoreXML = `<?xml version="1.0" encoding="UTF-8"?>
    <gupdate protocol="2.0" xmlns="http://www.google.com/update2/response">
      <app appid="gkjmnbdackjbjnahbilidcpgdegbdmhh">
        <updatecheck codebase="` + webstoreURL + `/extension.crx" version="82"/>
      </app>
    </gupdate>`

	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintf(w, webstoreXML)
	})

	http.HandleFunc("/extension.crx", func(w http.ResponseWriter, r *http.Request) {
		http.ServeFile(w, r, extensionPath)
	})

	http.ListenAndServe(webstorePort, nil)
}

func ensureExtensionIsInstalled(ctx context.Context, cr *chrome.Chrome) error {
	conn, err := cr.NewConn(ctx, "chrome://extensions")
	if err != nil {
		return errors.Wrap(err, "could not connect to test extensions")
	}
	defer conn.Close()

	err1 := conn.Exec(ctx, `document.querySelector('extensions-manager').shadowRoot.querySelector('extensions-toolbar').shadowRoot.querySelector('cr-toggle').click()`)
	err2 := conn.Exec(ctx, `document.querySelector('extensions-manager').shadowRoot.querySelector('extensions-toolbar').shadowRoot.querySelector('#updateNow').click()
  `)
	if err1 != nil || err2 != nil {
		return errors.Wrap(err, "could not update test extensions")
	}

	err = conn.WaitForExpr(ctx, `document.querySelector('extensions-manager').shadowRoot.getElementById('items-list').shadowRoot.getElementById('gkjmnbdackjbjnahbilidcpgdegbdmhh') !== null`)
	if err != nil {
		return errors.Wrap(err, "could not find test extension")
	}

	return nil
}
