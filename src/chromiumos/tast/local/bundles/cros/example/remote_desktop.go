// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package example

import (
	"context"
	"encoding/json"
	"fmt"
	"path/filepath"
	"time"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/session"
	"chromiumos/tast/local/testexec"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         RemoteDesktop,
		Desc:         "Connect to Chrome Remote Desktop for working remotely",
		Contacts:     []string{"shik@chromium.org", "tast-users@chromium.org"},
		Attr:         []string{"disabled", "informational"},
		SoftwareDeps: []string{"chrome"},
		Vars:         []string{"user", "pass", "gaia", "wait"},
	})
}

const appID = "inomeogfingihgjfjlpeplalcfajhgai"
const appCWSURL = "https://chrome.google.com/webstore/detail/chrome-remote-desktop/inomeogfingihgjfjlpeplalcfajhgai"

func isInstalled(ctx context.Context, tconn *chrome.Conn) (bool, error) {
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	// TODO(shik): Find a better way to detect whether chrome.management.getAll() is stablized.  It
	// will try to sync the installed app at login.
	code := fmt.Sprintf(`
		new Promise(resolve => {
		  let count = -1;
		  let updated = new Date();
		  let interval = setInterval(() => {
			chrome.management.getAll((xs) => {
			  if (chrome.runtime.lastError) {
				clearInterval(interval);
				reject(new Error(chrome.runtime.lastError.message));
			  }
			  const now = new Date();
			  if (xs.length != count) {
				count = xs.length;
				updated = now;
			  }
			  const hasApp = xs.some((x) => x.id == %q);
			  if (hasApp || now - updated >= 10000) {
				clearInterval(interval);
				resolve(hasApp);
			  }
			})
		  }, 100);
		})`, appID)
	result := false
	if err := tconn.EvalPromise(ctx, code, &result); err != nil {
		return false, err
	}
	return result, nil
}

func launch(ctx context.Context, cr *chrome.Chrome, tconn *chrome.Conn) (*chrome.Conn, error) {
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	conn, err := cr.NewConn(ctx, "https://remotedesktop.google.com/support?hl=en")
	if err != nil {
		return nil, err
	}

	waitIdle := "new Promise(resolve => window.requestIdleCallback(resolve))"
	if err := conn.EvalPromise(ctx, waitIdle, nil); err != nil {
		return nil, err
	}

	return conn, nil
}

func getAccessCode(ctx context.Context, crd *chrome.Conn) (string, error) {
	ctx, cancel := context.WithTimeout(ctx, 20*time.Second)
	defer cancel()

	genCodeBtn := `document.querySelector('[aria-label="Generate Code"]')`
	if err := crd.WaitForExpr(ctx, genCodeBtn); err != nil {
		return "", err
	}

	clickBtn := genCodeBtn + ".click()"
	if err := crd.Exec(ctx, clickBtn); err != nil {
		return "", err
	}

	codeSpan := `document.querySelector('[aria-label^="Your access code is:"]')`
	if err := crd.WaitForExpr(ctx, codeSpan); err != nil {
		return "", err
	}

	code := ""
	getCode := codeSpan + `.getAttribute('aria-label').match(/\d+/g).join('')`
	if err := crd.EvalPromise(ctx, getCode, &code); err != nil {
		return "", err
	}
	return code, nil
}

func pressEnter(ctx context.Context) error {
	// TODO(shik): Cache this path instead of globbing it everytime.
	const pattern = "/usr/local/lib*/python2.7/site-packages/uinput/cros_type_keys.py"
	paths, err := filepath.Glob(pattern)
	if err != nil {
		return err
	}
	if len(paths) != 1 {
		return errors.Errorf("expect exactly one cros_type_keys.py, but got %d", len(paths))
	}
	cmd := testexec.CommandContext(ctx, "python", paths[0], "-k", "KEY_ENTER")
	if _, err := cmd.CombinedOutput(); err != nil {
		cmd.DumpLog(ctx)
		return err
	}
	return nil
}

func waitConnection(ctx context.Context, crd *chrome.Conn) error {
	return testing.Poll(ctx, func(ctx context.Context) error {
		// TODO(shik): Find a better way to click the getAccessCode button instead of pressing enter.
		if err := pressEnter(ctx); err != nil {
			return testing.PollBreak(err)
		}

		connected := false
		hasStopBtn := `!!document.querySelector('[aria-label="Stop Sharing"]')`
		if err := crd.Eval(ctx, hasStopBtn, &connected); err != nil {
			return testing.PollBreak(err)
		}
		if connected {
			return nil
		}
		return errors.New(`cannot find "Stop Sharing" button`)
	}, &testing.PollOptions{Interval: time.Second})
}

func RemoteDesktop(ctx context.Context, s *testing.State) {
	user, err := session.NormalizeEmail(s.RequiredVar("user"))
	if err != nil {
		s.Fatal("Failed to get normalized email: ", err)
	}
	cr, err := func() (*chrome.Chrome, error) {
		gaia, ok := s.Var("gaia")
		if !ok {
			s.Log("Hint: you can provide gaia id to keep cryptohome and make the login process faster")
			return chrome.New(
				ctx,
				chrome.Auth(user, s.RequiredVar("pass"), ""),
				chrome.GAIALogin(),
			)
		}
		return chrome.New(
			ctx,
			chrome.Auth(user, s.RequiredVar("pass"), gaia),
			chrome.KeepCryptohome(),
		)
	}()
	if err != nil {
		s.Fatal("Failed to start Chrome: ", err)
	}
	defer cr.Close(ctx)

	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to connect Test API: ", err)
	}
	defer tconn.Close()

	installed, err := isInstalled(ctx, tconn)
	if err != nil {
		s.Fatal("Failed to check isInstalled(): ", err)
	}
	if !installed {
		s.Fatal("CRD extension is not installed on that account. Please install it at ", appCWSURL)
	}

	crd, err := launch(ctx, cr, tconn)
	if err != nil {
		s.Fatal("Failed to launch CRD: ", err)
	}

	s.Log("Getting access code")
	accessCode, err := getAccessCode(ctx, crd)
	if err != nil {
		s.Fatal("Failed to getAccessCode: ", err)
	}
	s.Log("Access code: ", accessCode)

	wait := func() bool {
		val, ok := s.Var("wait")
		if !ok {
			return true
		}
		var wait bool
		if err := json.Unmarshal([]byte(val), &wait); err != nil {
			s.Fatal("Failed to unmarshal the varaible `wait`: ", err)
		}
		return wait
	}()

	if wait {
		s.Log("Waiting connection")
		if err := waitConnection(ctx, crd); err != nil {
			s.Fatal("No client connected: ", err)
		}
	}

	defer crd.Close()
}
