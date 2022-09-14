// Copyright 2022 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package lacros

import (
	"context"
	"os"
	"time"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/bundles/cros/lacros/migrate"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/browser"
	"chromiumos/tast/local/chrome/lacros"
	"chromiumos/tast/local/chrome/lacros/lacrosfixt"
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/chrome/uiauto/nodewith"
	"chromiumos/tast/local/chrome/uiauto/role"
	"chromiumos/tast/local/input"
	"chromiumos/tast/local/policyutil"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         Migrate,
		LacrosStatus: testing.LacrosVariantExists,
		Desc:         "Test functionality of Ash-to-Lacros profile migration",
		Contacts: []string{
			"neis@google.com", // Test author
			"ythjkt@google.com",
			"hidehiko@google.com",
			"lacros-team@google.com",
		},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"chrome", "lacros"},
		Params: []testing.Param{{
			Name: "primary",
			Val:  []lacrosfixt.Option{lacrosfixt.Mode(lacros.LacrosPrimary)},
		}, {
			Name: "only",
			Val:  []lacrosfixt.Option{lacrosfixt.Mode(lacros.LacrosOnly)},
		}},
	})
}

func Migrate(ctx context.Context, s *testing.State) {
	kb, err := input.Keyboard(ctx)
	if err != nil {
		s.Fatal("Failed to get keyboard: ", err)
	}
	defer kb.Close()

	prepareAshProfile(ctx, s, kb)
	cr, err := migrate.Run(ctx, s.Param().([]lacrosfixt.Option))
	if err != nil {
		s.Fatal("Failed to migrate profile: ", err)
	}
	defer cr.Close(ctx)
	verifyLacrosProfile(ctx, s, kb, cr)
}

const (
	bookmarkName         = "MyBookmark12345"                  // Arbitrary.
	extensionName        = "User-Agent Switcher for Chrome"   // Arbitrary extension from Chrome Store.
	extensionID          = "djflhoibgkdhkhhcedjiklpkjnoahfmg" // ID of the above extension.
	shortcutName         = "MyShortcut12345"                  // Arbitrary.
	titleOfAlphabetPage  = "Alphabet"                         // https://abc.xyz page title.
	titleOfDownloadsPage = "Downloads"                        // chrome://downloads page title.
	titleOfNewTabPage    = "New Tab"                          // chrome://newtab page title.
	cookie               = "MyCookie1234=abcd"                // Arbitrary cookie.
	localStorageKey      = "myCat"                            // Arbitrary localStorage key.
	localStorageValue    = "Meow"                             // Arbitrary localStorage value.
	indexedDBUserID      = 123                                // Arbitrary user id.
	indexedDBUserEmail   = "test@gmail.com"                   // Arbitrary user email.
	// Create an arbitrary indexedDB store and add a value.
	insertIndexedDBDataJS = `
    (userId, userEmail) => {
        return new Promise((resolve, reject) => {
            const req = window.indexedDB.open("someDataBase", 1);
            req.onerror = () => {
                console.error("Opening a database failed.");
                reject();
            }
            req.onupgradeneeded = e => {
                const db = e.target.result;
                const objectStore = db.createObjectStore("users", { keyPath: "id" });
                objectStore.transaction.onerror = () => {
                    console.error("Creating an object store failed.");
                    reject();
                }
                objectStore.transaction.oncomplete = () => {
                    const userObjectStore = db.transaction("users", 'readwrite')
                        .objectStore("users");
                    const req = userObjectStore.add({ id: userId, email: userEmail });
                    req.error = () => {
                        console.error("Adding an entry to database failed.");
                        reject();
                    };
                    req.onsuccess = () => resolve();
                }
            }
        })
    }`
	// Check that the value stored with insertIndexedDBDataJS is present.
	getIndexedDBDataJS = `
    (userId, userEmail) => {
        return new Promise((resolve, reject) => {
            const req = window.indexedDB.open("someDataBase");
            req.onerror = () => {
                console.log("Opening database 'someDataBase' failed.");
                reject();
            }
            req.onsuccess = e => {
                const db = e.target.result;
                const req = db.transaction("users").objectStore("users").get(userId);
                req.onsuccess = () => {
                    if (req.result.email == userEmail) {
                        resolve();
                    } else {
                        console.error("userEmail != " + req.result.email);
                        reject();
                    }
                }
                req.onerror = () => {
                    console.error("Failed to get user with id: " + userId);
                    reject();
                }
            }
        })
    }`
)

func waitForHistoryEntry(ctx context.Context, ui *uiauto.Context, br *browser.Browser, allowReload bool) error {
	conn, err := br.NewConn(ctx, "chrome://history")
	if err != nil {
		return errors.Wrap(err, "failed to open history page")
	}
	defer conn.Close()
	alphabetLink := nodewith.Name(titleOfAlphabetPage).Role(role.Link)
	err = ui.WaitUntilExists(alphabetLink)(ctx)
	if err != nil && allowReload {
		// If the page in question has just been visited, sometimes the
		// history page needs to be reloaded before the entry shows up
		// there. So reload and try again with a longer timeout.
		err = uiauto.Combine("find Alphabet history entry", br.ReloadActiveTab, ui.WithTimeout(30*time.Second).WaitUntilExists(alphabetLink))(ctx)
	}
	if err != nil {
		return err
	}
	if err := conn.CloseTarget(ctx); err != nil {
		return errors.Wrap(err, "failed to close target")
	}
	return nil
}

// prepareAshProfile resets profile migration, installs an extension, and
// creates two tabs, browsing history, a bookmark, a download, a shortcut, a
// cookie, an indexedDB entry and a localStorage value.
func prepareAshProfile(ctx context.Context, s *testing.State, kb *input.KeyboardEventWriter) {
	// First restart Chrome with Lacros disabled in order to reset profile migration.
	cr, err := chrome.New(ctx, chrome.DisableFeatures("LacrosSupport"))
	if err != nil {
		s.Fatal("Failed to start Chrome: ", err)
	}
	defer cr.Close(ctx)

	if err := testing.Poll(ctx, func(ctx context.Context) error {
		if _, err := os.Stat(migrate.LacrosFirstRunPath); !os.IsNotExist(err) {
			return errors.Wrap(err, "'First Run' file exists or cannot be read")
		}
		return nil
	}, nil); err != nil {
		s.Fatal("'First Run' file exists or cannot be read: ", err)
	}

	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to create Test API connection: ", err)
	}
	ui := uiauto.New(tconn)

	// Install an extension.
	if err := policyutil.EnsureGoogleCookiesAccepted(ctx, cr.Browser()); err != nil {
		s.Fatal("Failed to accept cookies: ", err)
	}
	extensionURL := "https://chrome.google.com/webstore/detail/" + extensionID + "?hl=en"
	conn, err := cr.NewConn(ctx, extensionURL)
	if err != nil {
		s.Fatal("Failed to open extension page: ", err)
	}
	defer conn.Close()
	addButton1 := nodewith.Name("Add to Chrome").Role(role.Button).First()
	addButton2 := nodewith.Name("Add extension").Role(role.Button)
	removeButton := nodewith.Name("Remove from Chrome").Role(role.Button).First()
	if err := uiauto.Combine("Install extension",
		ui.LeftClick(addButton1),
		// The "Add extension" button may not immediately be clickable.
		ui.LeftClickUntil(addButton2, ui.Gone(addButton2)),
		// TODO(crbug.com/1326398): Remove tab reload when this bug is fixed.
		ui.RetryUntil(cr.Browser().ReloadActiveTab, ui.WithTimeout(7*time.Second).WaitUntilExists(removeButton)),
	)(ctx); err != nil {
		s.Fatal("Failed to install: ", err)
	}

	// Visit the Alphabet page, creating a history entry.
	if err := conn.Navigate(ctx, "https://abc.xyz"); err != nil {
		s.Fatal("Failed to open Alphabet page: ", err)
	}
	if err := waitForHistoryEntry(ctx, ui, cr.Browser(), true); err != nil {
		s.Fatal("Failed to find Alphabet history entry: ", err)
	}

	// Set a cookie on that page.
	if err := conn.Call(ctx, nil, `(cookie) => document.cookie = cookie`, cookie); err != nil {
		s.Fatal("Failed to set cookie: ", err)
	}
	// Set localStorage on Alphabet page.
	if err := conn.Call(ctx, nil, `(key, value) => localStorage.setItem(key, value)`, localStorageKey, localStorageValue); err != nil {
		s.Fatal("Failed to set localStorage value: ", err)
	}
	// Create an indexedDB store and add a user in.
	if err := conn.Call(ctx, nil, insertIndexedDBDataJS, indexedDBUserID, indexedDBUserEmail); err != nil {
		s.Fatal("insertIndexedDBDataJS failed: ", err)
	}

	// Bookmark the chrome://downloads page.
	if err := conn.Navigate(ctx, "chrome://downloads"); err != nil {
		s.Fatal("Failed to open downloads page: ", err)
	}
	if err := kb.Accel(ctx, "Ctrl+d"); err != nil {
		s.Fatal("Failed to open bookmark creation popup: ", err)
	}
	if err := kb.Type(ctx, bookmarkName); err != nil {
		s.Fatal("Failed to type bookmark name: ", err)
	}
	doneButton := nodewith.Name("Done").Role(role.Button)
	if err := uiauto.Combine("Click 'Done' button",
		ui.LeftClick(doneButton),
		ui.WaitUntilGone(doneButton),
	)(ctx); err != nil {
		s.Fatal("Failed to click: ", err)
	}

	// Also download that page.
	if err := kb.Accel(ctx, "Ctrl+s"); err != nil {
		s.Fatal("Failed to open download popup: ", err)
	}
	saveButton := nodewith.Name("Save").Role(role.Button)
	if err := uiauto.Combine("Click 'Save' button",
		ui.WaitUntilExists(saveButton),
		ui.WaitUntilEnabled(saveButton),
		ui.LeftClick(saveButton),
		ui.WaitUntilGone(saveButton),
	)(ctx); err != nil {
		s.Fatal("Failed to click: ", err)
	}

	// Create a shortcut on the newtab page.
	if err := kb.Accel(ctx, "Ctrl+t"); err != nil {
		s.Fatal("Failed to open new tab: ", err)
	}
	addShortcutButton := nodewith.Name("Add shortcut").Role(role.Button)
	if err := uiauto.Combine("Click 'Add shortcut' button",
		ui.LeftClick(addShortcutButton),
		ui.WaitUntilGone(addShortcutButton),
	)(ctx); err != nil {
		s.Fatal("Failed to click: ", err)
	}
	if err := kb.Type(ctx, shortcutName+"\tfoobar"); err != nil {
		s.Fatal("Failed to type shortcut data: ", err)
	}
	if err := uiauto.Combine("Click 'Done' button",
		ui.LeftClick(doneButton),
		ui.WaitUntilGone(doneButton),
	)(ctx); err != nil {
		s.Fatal("Failed to click: ", err)
	}
}

// verifyLacrosProfile checks that the edits done by prepareAshProfile were carried over to Lacros.
func verifyLacrosProfile(ctx context.Context, s *testing.State, kb *input.KeyboardEventWriter, cr *chrome.Chrome) {
	if _, err := os.Stat(migrate.LacrosFirstRunPath); err != nil {
		s.Fatal("Error reading 'First Run' file: ", err)
	}

	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to create Test API connection: ", err)
	}

	l, err := lacros.Launch(ctx, tconn)
	if err != nil {
		s.Fatal("Failed to launch lacros: ", err)
	}

	// Check that the bookmark is present.
	ui := uiauto.New(tconn)
	bookmarkedButton := nodewith.Name(bookmarkName).Role(role.Button)
	if err = ui.WaitUntilExists(bookmarkedButton)(ctx); err != nil {
		s.Fatal("Failed to find bookmark: ", err)
	}

	// Check that the shortcut is present.
	shortcutLink := nodewith.Name(shortcutName).Role(role.Link)
	if err := ui.WaitUntilExists(shortcutLink)(ctx); err != nil {
		s.Fatal("Failed to find shortcut: ", err)
	}

	// Check that the browsing history contains the Alphabet page.
	if err := waitForHistoryEntry(ctx, ui, l.Browser(), false); err != nil {
		s.Fatal("Failed to find Alphabet history entry: ", err)
	}

	// Check that there is another tab showing the downloads page.
	if err := lacros.WaitForLacrosWindow(ctx, tconn, titleOfNewTabPage); err != nil {
		s.Fatal("Failed to find appropriate window: ", err)
	}
	if err := kb.Accel(ctx, "Ctrl+w"); err != nil {
		s.Fatal("Failed to close tab: ", err)
	}
	if err := lacros.WaitForLacrosWindow(ctx, tconn, titleOfDownloadsPage); err != nil {
		s.Fatal("Failed to find appropriate window: ", err)
	}

	// Check that the download page shows the previous download (of itself).
	downloadedFile := nodewith.Name(titleOfDownloadsPage + ".mhtml").Role(role.Link)
	if err = ui.WaitUntilExists(downloadedFile)(ctx); err != nil {
		s.Fatal("Failed to find download: ", err)
	}

	// Check that going back in history once brings us to the Alphabet page.
	if err := kb.Accel(ctx, "Alt+Left"); err != nil {
		s.Fatal("Failed to go to previous page: ", err)
	}
	if err := lacros.WaitForLacrosWindow(ctx, tconn, titleOfAlphabetPage); err != nil {
		s.Fatal("Failed to find appropriate window: ", err)
	}

	// Check if the cookie and localStorage values set in Ash are carried over to Lacros.
	func() {
		conn, err := l.NewConn(ctx, "https://abc.xyz")
		if err != nil {
			s.Fatal("Failed to open abc.xyz: ", err)
		}
		defer conn.Close()
		contained := false

		if err := conn.Call(ctx,
			&contained,
			`(cookie) => { return document.cookie.split('; ').includes(cookie); }`, cookie); err != nil {
			s.Fatal("Failed to get cookie: ", err)
		}
		if !contained {
			s.Fatal("Cookie set in Ash could not be found in Lacros")
		}

		contained = false
		if err := conn.Call(ctx, &contained,
			`(key, value) => { return localStorage.getItem(key) == value; }`, localStorageKey, localStorageValue); err != nil {
			s.Fatal("Failed to get localStorage value: ", err)
		}
		if !contained {
			s.Fatal("localStorage value set in Ash could not be found in Lacros")
		}
		if err := conn.Call(ctx, nil, getIndexedDBDataJS, indexedDBUserID, indexedDBUserEmail); err != nil {
			s.Fatal("getIndexedDBDataJS failed: ", err)
		}
	}()

	// Check that the extension is installed and enabled.
	func() {
		conn, err := l.NewConn(ctx, "chrome://extensions/?id="+extensionID)
		if err != nil {
			s.Fatal("Failed to open extension page: ", err)
		}
		defer conn.Close()
		extensionText := nodewith.Name(extensionName).Role(role.StaticText)
		onText := nodewith.Name("On").Role(role.StaticText)
		if err := uiauto.Combine("Verify extension status",
			ui.WaitUntilExists(extensionText),
			ui.Exists(onText),
		)(ctx); err != nil {
			s.Fatal("Failed: ", err)
		}
	}()
}
