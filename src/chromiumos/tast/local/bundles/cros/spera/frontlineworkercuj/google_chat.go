// Copyright 2022 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package frontlineworkercuj

import (
	"context"
	"fmt"
	"time"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/browser"
	"chromiumos/tast/local/chrome/cuj"
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/chrome/uiauto/nodewith"
	"chromiumos/tast/local/chrome/uiauto/role"
	"chromiumos/tast/local/input"
)

const (
	// chatAccount indicates the chatting account.
	chatAccount = "testuser@gmail.com"

	// defaultUIWaitTime indicates the default time to wait for UI elements to appear.
	defaultUIWaitTime = 5 * time.Second
)

// GoogleChat holds the information used to do Google Chat testing.
type GoogleChat struct {
	ui    *uiauto.Context
	uiHdl cuj.UIActionHandler
	kb    *input.KeyboardEventWriter
	conn  *chrome.Conn
	br    *browser.Browser
}

// NewGoogleChat returns the the manager of Google Chat, caller will able to control Google Chat app through this object.
func NewGoogleChat(ctx context.Context, br *browser.Browser, ui *uiauto.Context, uiHdl cuj.UIActionHandler, kb *input.KeyboardEventWriter) *GoogleChat {
	return &GoogleChat{
		ui:    ui,
		uiHdl: uiHdl,
		kb:    kb,
		br:    br,
	}
}

// maybeCloseWelcomeDialog closes the welcome dialog if it exists.
func (g *GoogleChat) maybeCloseWelcomeDialog(ctx context.Context) error {
	welcomeImage := nodewith.Name("Welcome to Google Chat").Role(role.Image)
	button := nodewith.Name("Get started").Role(role.Button).Focusable()
	welcomeDialog := nodewith.NameContaining("Find and start chats").Role(role.AlertDialog)
	close := nodewith.Name("Close").Role(role.Button).Ancestor(welcomeDialog)
	return uiauto.Combine("close the welcome dialogs",
		uiauto.IfSuccessThen(g.ui.WithTimeout(defaultUIWaitTime).WaitUntilExists(welcomeImage), g.uiHdl.Click(button)),
		uiauto.IfSuccessThen(g.ui.WithTimeout(defaultUIWaitTime).WaitUntilExists(welcomeDialog), g.uiHdl.Click(close)),
	)(ctx)
}

// Launch launches the Google Chat standalone app.
func (g *GoogleChat) Launch(ctx context.Context) (err error) {
	g.conn, err = g.br.NewConn(ctx, cuj.GoogleChatURL)
	if err != nil {
		return errors.Wrapf(err, "failed to open URL: %s", cuj.GoogleChatURL)
	}

	installGoogleChatButton := nodewith.Name("Install Google Chat").Role(role.Button).Focusable()
	installAlertDialog := nodewith.Name("Install app?").Role(role.AlertDialog).Focused()
	installButton := nodewith.Name("Install").Role(role.Button).Ancestor(installAlertDialog).Default()

	installGoogleChat := uiauto.NamedCombine("install Google Chat app",
		g.ui.DoDefault(installGoogleChatButton),
		g.ui.DoDefaultUntil(
			installButton,
			g.ui.WithTimeout(defaultUIWaitTime).WaitUntilGone(installAlertDialog),
		),
	)

	openLinkButton := nodewith.Name("To open this link, choose an app").Role(role.Button).Focusable()
	chooseAppDialog := nodewith.Name("To open this link, choose an app").Role(role.Dialog).Focusable()
	googleChatOption := nodewith.Name("Google Chat").Role(role.Button).Ancestor(chooseAppDialog)
	openButton := nodewith.Name("Open").Role(role.Button).Default()
	tryDesktopApp := nodewith.Name("Try the Chat desktop app").Role(role.Dialog)
	notNowButton := nodewith.Name("Not now").Role(role.Button).Ancestor(tryDesktopApp)

	openGoogleChat := func(ctx context.Context) error {
		found, err := g.ui.IsNodeFound(ctx, openLinkButton)
		if err != nil {
			return err
		}
		if found {
			if err := uiauto.Combine("open the Google Chat app",
				g.uiHdl.Click(openLinkButton),
				g.uiHdl.Click(googleChatOption),
				g.uiHdl.Click(openButton),
			)(ctx); err != nil {
				return err
			}
		}
		return uiauto.IfSuccessThen(
			g.ui.WithTimeout(defaultUIWaitTime).WaitUntilExists(tryDesktopApp),
			g.uiHdl.Click(notNowButton),
		)(ctx)
	}

	return uiauto.Combine("install the Google Chat and open the app",
		uiauto.IfSuccessThen(
			g.ui.WithTimeout(longerUIWaitTime).WaitUntilExists(installGoogleChatButton),
			installGoogleChat),
		g.maybeCloseWelcomeDialog,
		openGoogleChat,
	)(ctx)
}

// StartChat starts a chat.
func (g *GoogleChat) StartChat(ctx context.Context) error {
	navigationHovered := nodewith.Role(role.Navigation).Hovered()
	chatGroup := nodewith.NameContaining("Chat").Role(role.Group)
	startChatButton := nodewith.Name("Start a chat").Role(role.Button).Ancestor(chatGroup).Focusable()
	findContainer := nodewith.Name("Find or start conversations").Role(role.GenericContainer).Focusable()
	textBox := nodewith.NameStartingWith("Type person").Role(role.TextField).Ancestor(findContainer)
	textBoxFocused := textBox.Focused()
	accountOption := nodewith.NameContaining(chatAccount).Role(role.ListBoxOption).Focusable()
	messageFieldName := fmt.Sprintf("Message %s. History is on.", chatAccount)
	messageField := nodewith.Name(messageFieldName).Role(role.TextField).Editable()
	workSapceDialog := nodewith.Name("Google Workspace tools").Role(role.AlertDialog)
	okButton := nodewith.Name("OK").Role(role.Button).Ancestor(workSapceDialog)
	return uiauto.Combine("start a conversation",
		uiauto.IfSuccessThen(g.ui.WithTimeout(defaultUIWaitTime).WaitUntilExists(navigationHovered), g.uiHdl.Click(navigationHovered)),
		g.ui.DoDefault(startChatButton),
		uiauto.IfFailThen(
			g.ui.WithTimeout(defaultUIWaitTime).WaitUntilExists(textBoxFocused),
			g.ui.DoDefault(textBox),
		),
		g.kb.TypeAction(chatAccount),
		g.ui.DoDefaultUntil(
			accountOption,
			g.ui.WithTimeout(defaultUIWaitTime).WaitUntilExists(messageField),
		),
		uiauto.IfSuccessThen(
			g.ui.WithTimeout(defaultUIWaitTime).WaitUntilExists(workSapceDialog),
			g.ui.DoDefault(okButton),
		),
		g.ui.DoDefault(messageField),
		g.kb.TypeAction("hi"),
		g.kb.AccelAction("Enter"),
	)(ctx)
}
