// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package gmail

import (
	"context"
	"time"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/android/ui"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/uiauto/launcher"
	"chromiumos/tast/local/input"
	"chromiumos/tast/testing"
)

// App provides controls of Gmail application.
type App struct{}

// NewApp creates an App instance and opens Gmail application at start.
// It also tries to initialize it in case Gmail is launched the first time.
func NewApp(ctx context.Context, cr *chrome.Chrome, d *ui.Device) (*App, error) {
	const appName = "Gmail"
	gmail := &App{}
	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		return nil, errors.Wrap(err, "failed to create test API connection")
	}
	defer tconn.Close()
	kb, err := input.Keyboard(ctx)
	if err != nil {
		return gmail, errors.Wrap(err, `failed to open the keyboard`)
	}
	defer kb.Close()
	if err := launcher.SearchAndLaunch(tconn, kb, appName)(ctx); err != nil {
		return gmail, errors.Wrap(err, "failed to launch Gmail app")
	}
	if err := gmail.initGmail(ctx, d); err != nil {
		return gmail, errors.Wrap(err, "failed to initialize Gmail")
	}
	return gmail, nil
}

// initGmail initializes Gmail APP if it hasn't been initialized before.
func (*App) initGmail(ctx context.Context, d *ui.Device) error {
	const (
		dialogID            = "com.google.android.gm:id/customPanel"
		dismissID           = "com.google.android.gm:id/gm_dismiss_button"
		customPanelMaxCount = 10
		waitTime            = time.Second * 3
		timeout             = time.Second * 5
	)

	gotIt := d.Object(ui.Text("GOT IT"))
	if err := gotIt.WaitForExists(ctx, waitTime); err != nil {
		testing.ContextLog(ctx, `Failed to find "GOT IT" button, believing splash screen has been dismissed already`)
		return nil
	}
	if err := gotIt.Click(ctx); err != nil {
		return errors.Wrap(err, `failed to click "GOT IT" button`)
	}
	// Sometimes, the account information might not be ready yet. In that case
	// a warning dialog appears. If the warning message does not appear, it
	// is fine.
	pleaseAdd := d.Object(ui.Text("Please add at least one email address"))
	if err := pleaseAdd.WaitForExists(ctx, timeout); err == nil {
		// Even though the warning dialog appears, the email address should
		// appear already. Therefore, here simply clicks the 'OK' button to
		// dismiss the warning dialog and moves on.
		if err := testing.Sleep(ctx, waitTime); err != nil {
			return errors.Wrap(err, "failed to wait for the email address appearing")
		}
		okButton := d.Object(ui.ClassName("android.widget.Button"), ui.Text("OK"))
		if err := okButton.Exists(ctx); err != nil {
			return errors.Wrap(err, "failed to find the ok button")
		}
		if err := okButton.Click(ctx); err != nil {
			return errors.Wrap(err, "failed to click the OK button")
		}
	}
	takeMe := d.Object(ui.Text("TAKE ME TO GMAIL"))
	if err := takeMe.WaitForExists(ctx, timeout); err != nil {
		return errors.Wrap(err, `"TAKE ME TO GMAIL" is not shown`)
	}
	if err := takeMe.Click(ctx); err != nil {
		return errors.Wrap(err, `failed to click "TAKE ME TO GMAIL" button`)
	}
	// After clicking 'take me to gmail', it might show a series of dialogs to
	// finalize the setup. Here skips those dialogs by clicking their 'ok'
	// buttons.
	for i := 0; i < customPanelMaxCount; i++ {
		dialog := d.Object(ui.ID(dialogID))
		if err := dialog.WaitForExists(ctx, timeout); err != nil {
			return nil
		}
		dismiss := d.Object(ui.ID(dismissID))
		if err := dismiss.Exists(ctx); err != nil {
			return errors.Wrap(err, "dismiss button not found")
		}
		if err := dismiss.Click(ctx); err != nil {
			return errors.Wrap(err, "failed to click the dismiss button")
		}
	}
	return errors.New("too many dialog popups")
}

// Send sends an email through Gmail application to the specified receiver.
func (*App) Send(ctx context.Context, d *ui.Device, receiver, subject, content string) error {
	const (
		newMailID        = "com.google.android.gm:id/compose_button"
		newMailClassName = "android.widget.ImageButton"
		subjectID        = "com.google.android.gm:id/subject"
		subjectClassName = "android.widget.EditText"
		contentID        = "com.google.android.gm:id/composearea_tap_trap_bottom"
		contentClassName = "android.view.View"
		sendID           = "com.google.android.gm:id/send"
		sendClassName    = "android.widget.TextView"
		timeout          = time.Second * 5
	)

	kb, err := input.Keyboard(ctx)
	if err != nil {
		return errors.Wrap(err, `failed to open the keyboard`)
	}
	defer kb.Close()

	newMail := d.Object(ui.ID(newMailID), ui.ClassName(newMailClassName))
	if err := newMail.WaitForExists(ctx, timeout); err != nil {
		return errors.Wrap(err, `compose button is not shown`)
	}
	if err := newMail.Click(ctx); err != nil {
		return errors.Wrap(err, `failed to click Compose mail button`)
	}

	subjectRow := d.Object(ui.ID(subjectID), ui.ClassName(subjectClassName))
	if err := subjectRow.WaitForExists(ctx, timeout); err != nil {
		return errors.Wrap(err, `subject row is not shown`)
	}

	if err := kb.Type(ctx, receiver); err != nil {
		return errors.Wrap(err, `failed to do keyboard type`)
	}
	if err := kb.Accel(ctx, "Enter"); err != nil {
		return errors.Wrap(err, `failed to send keyboard event`)
	}

	if err := subjectRow.WaitForExists(ctx, timeout); err != nil {
		return errors.Wrap(err, `subject row is not shown`)
	}
	if err := subjectRow.Click(ctx); err != nil {
		return errors.Wrap(err, `failed to click Subject row`)
	}
	if err := kb.Type(ctx, subject); err != nil {
		return errors.Wrap(err, `failed to do keyboard type`)
	}
	if err := kb.Accel(ctx, "Enter"); err != nil {
		return errors.Wrap(err, `failed to send keyboard event`)
	}

	contentField := d.Object(ui.ID(contentID), ui.ClassName((contentClassName)))
	if err := contentField.WaitForExists(ctx, timeout); err != nil {
		return errors.Wrap(err, `content field is not shown`)
	}
	if err := contentField.Click(ctx); err != nil {
		return errors.Wrap(err, `failed to click content field`)
	}
	if err := kb.Type(ctx, content); err != nil {
		return errors.Wrap(err, `failed to do keyboard type`)
	}

	send := d.Object(ui.ID(sendID), ui.ClassName(sendClassName))
	if err := send.WaitForExists(ctx, timeout); err != nil {
		return errors.Wrap(err, `subject row is not shown`)
	}
	if err := send.Click(ctx); err != nil {
		return errors.Wrap(err, `failed to click Subject row`)
	}

	// Wait for sending mail.
	if err := testing.Sleep(ctx, time.Second); err != nil {
		return errors.Wrap(err, "failed to wait")
	}

	return nil
}
