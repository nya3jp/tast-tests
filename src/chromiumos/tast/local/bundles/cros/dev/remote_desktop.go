// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package example

import (
	"context"
	"strconv"

	"chromiumos/tast/local/chrome"
	"chromiumos/tast/testing"
)

func init() {
	// Example usage:
	// $ tast run -var=user=<username> -var=pass=<password> <dut ip> dev.RemoteDesktop
	// <username> and <password> are the credentials of the test GAIA account.
	testing.AddTest(&testing.Test{
		Func:         RemoteDesktop,
		Desc:         "Connect to Chrome Remote Desktop for working remotely",
		Contacts:     []string{"shik@chromium.org", "tast-users@chromium.org"},
		Attr:         []string{"disabled"},
		SoftwareDeps: []string{"chrome"},
		Vars:         []string{"user", "pass", "wait"},
	})
}

func ensureAppInstalled(ctx context.Context, cr *chrome.Chrome, tconn *chrome.Conn) error {
	// The companion extension for the Chrome Remote Desktop website https://remotedesktop.google.com.
	const appCWSURL = "https://chrome.google.com/webstore/detail/chrome-remote-desktop/inomeogfingihgjfjlpeplalcfajhgai?hl=en"

	cws, err := cr.NewConn(ctx, appCWSURL)
	if err != nil {
		return err
	}
	defer cws.Close()
	defer cws.CloseTarget(ctx)

	// The confirm button might not be clickable at first, so we keep retrying
	// until we see "Remove from Chrome".
	const code = `
		new Promise((resolve) => {
		  chrome.automation.getDesktop((root) => {
		    const getButton = (name) => {
		      return root.find({
		        role: 'button',
		        attributes: {name},
		      });
		    };
		    let addClicked = false;
		    const interval = setInterval(() => {
		      if (!addClicked) {
		        const addButton = getButton('Add to Chrome');
		        if (addButton !== null) {
		          addButton.doDefault();
		          addClicked = true;
		        }
		      }
		      const confirmButton = getButton('Add extension');
		      if (confirmButton !== null) {
		        confirmButton.doDefault();
		      }
		      const removeButton = getButton('Remove from Chrome');
		      if (removeButton !== null) {
		        resolve();
		        clearInterval(interval);
		      }
		    }, 1000)
		  });
		});`
	return tconn.EvalPromise(ctx, code, nil)
}

func launch(ctx context.Context, cr *chrome.Chrome, tconn *chrome.Conn) (*chrome.Conn, error) {
	conn, err := cr.NewConn(ctx, "https://remotedesktop.google.com/support?hl=en")
	if err != nil {
		return nil, err
	}

	const waitIdle = "new Promise(resolve => window.requestIdleCallback(resolve))"
	if err := conn.EvalPromise(ctx, waitIdle, nil); err != nil {
		return nil, err
	}

	return conn, nil
}

func getAccessCode(ctx context.Context, crd *chrome.Conn) (string, error) {
	const genCodeBtn = `document.querySelector('[aria-label="Generate Code"]')`
	if err := crd.WaitForExpr(ctx, genCodeBtn); err != nil {
		return "", err
	}

	const clickBtn = genCodeBtn + ".click()"
	if err := crd.Exec(ctx, clickBtn); err != nil {
		return "", err
	}

	const codeSpan = `document.querySelector('[aria-label^="Your access code is:"]')`
	if err := crd.WaitForExpr(ctx, codeSpan); err != nil {
		return "", err
	}

	var code string
	const getCode = codeSpan + `.getAttribute('aria-label').match(/\d+/g).join('')`
	if err := crd.EvalPromise(ctx, getCode, &code); err != nil {
		return "", err
	}
	return code, nil
}

func waitConnection(ctx context.Context, tconn *chrome.Conn) error {
	// The share button might not be clickable at first, so we keep retrying
	// until we see "Stop Sharing".
	const waitShareBtn = `
		new Promise((resolve) => {
		  chrome.automation.getDesktop((root) => {
		    const getButton = (name) => {
		      return root.find({
		        role: 'button',
		        attributes: {name},
		      });
		    };
		    const interval = setInterval(() => {
		      const shareButton = getButton('Share');
		      if (shareButton !== null) {
		        shareButton.doDefault();
		      }
		      const stopButton = getButton('Stop Sharing');
		      if (stopButton !== null) {
		        clearInterval(interval);
		        resolve();
		      }
		    }, 1000);
		  });
		});`

	if err := tconn.EvalPromise(ctx, waitShareBtn, nil); err != nil {
		return err
	}
	return nil
}

func RemoteDesktop(ctx context.Context, s *testing.State) {
	// TODO(shik): Fix GAIALogin() with KeepCryptohome() to make login faster.
	// TODO(shik): The button names only work in English locale, and adding
	// "lang=en-US" for Chrome does not work.
	cr, err := chrome.New(
		ctx,
		chrome.Auth(s.RequiredVar("user"), s.RequiredVar("pass"), ""),
		chrome.GAIALogin(),
	)
	if err != nil {
		s.Fatal("Failed to start Chrome: ", err)
	}
	defer cr.Close(ctx)

	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to connect Test API: ", err)
	}

	if err := ensureAppInstalled(ctx, cr, tconn); err != nil {
		s.Fatal("Failed to install CRD app: ", err)
	}

	crd, err := launch(ctx, cr, tconn)
	if err != nil {
		s.Fatal("Failed to launch CRD: ", err)
	}
	defer crd.Close()

	s.Log("Getting access code")
	accessCode, err := getAccessCode(ctx, crd)
	if err != nil {
		s.Fatal("Failed to getAccessCode: ", err)
	}
	s.Log("Access code: ", accessCode)

	wait := func() bool {
		strVal, ok := s.Var("wait")
		if !ok {
			return true
		}
		boolVal, err := strconv.ParseBool(strVal)
		if err != nil {
			s.Fatal("Failed to parse the variable `wait`: ", err)
		}
		return boolVal
	}()

	if wait {
		s.Log("Waiting connection")
		if err := waitConnection(ctx, tconn); err != nil {
			s.Fatal("No client connected: ", err)
		}
	}
}
