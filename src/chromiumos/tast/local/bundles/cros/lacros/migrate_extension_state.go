// Copyright 2022 The ChromiumOS Authors.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package lacros

import (
	"context"
	"io/ioutil"
	"os"
	"path/filepath"

	"chromiumos/tast/errors"
	"chromiumos/tast/fsutil"
	"chromiumos/tast/local/bundles/cros/lacros/migrate"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/lacros"
	"chromiumos/tast/local/chrome/lacros/lacrosfixt"
	"chromiumos/tast/testing"
)

var extensionFiles = []string{
	"manifest.json",
	"index.html",
}

const (
	simpleExtensionID  = "jjbombdcioakijidkhacpmocpojelebc"
	simpleExtensionURL = "chrome-extension://" + simpleExtensionID + "/index.html"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         MigrateExtensionState,
		LacrosStatus: testing.LacrosVariantExists,
		Desc:         "Test Ash-to-Lacros profile migration handles extension states",
		Contacts: []string{
			"ythjkt@google.com", // Test author
			"neis@google.com",
			"hidehiko@google.com",
			"lacros-team@google.com",
		},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"chrome", "lacros"},
		Data:         extensionFiles,
		Params: []testing.Param{{
			Name: "primary",
			Val:  []lacrosfixt.Option{lacrosfixt.Mode(lacros.LacrosPrimary)},
		}, {
			Name: "only",
			Val:  []lacrosfixt.Option{lacrosfixt.Mode(lacros.LacrosOnly)},
		}},
	})
}

// MigrateExtensionState tests that the migration successfully migrates extension state by:
// 1. Store extension state by calling relevant APIs on the extension page on Ash.
// 2. Run profile migration from Ash to Lacros.
// 3. Check if the states created in step 1 can be found on Lacros.
func MigrateExtensionState(ctx context.Context, s *testing.State) {
	extDir, err := ioutil.TempDir("", "simple_extension.")
	if err != nil {
		s.Fatal("Failed to create simple_extension dir: ", err)
	}
	defer os.RemoveAll(extDir)

	for _, name := range extensionFiles {
		if err := fsutil.CopyFile(s.DataPath(name), filepath.Join(extDir, name)); err != nil {
			s.Fatal("Failed to copy file: ", err)
		}
	}

	if err := prepareExtensionState(ctx, extDir); err != nil {
		s.Fatal("Failed to prepare extension state: ", err)
	}

	opts := s.Param().([]lacrosfixt.Option)
	opts = append(opts, lacrosfixt.UnpackedExtension(extDir))
	cr, err := migrate.Run(ctx, opts)
	if err != nil {
		s.Fatal("Failed to migrate profile: ", err)
	}
	defer cr.Close(ctx)

	verifyExtensionStateOnLacros(ctx, s, cr)
}

func prepareExtensionState(ctx context.Context, extDir string) error {
	// TODO(ythjkt): Call more extension api which store values on disk from https://developer.chrome.com/docs/extensions/reference/.
	cr, err := chrome.New(ctx, chrome.DisableFeatures("LacrosSupport"), chrome.UnpackedExtension(extDir))
	if err != nil {
		return errors.Wrap(err, "failed to start chrome")
	}
	defer cr.Close(ctx)

	if err := testing.Poll(ctx, func(ctx context.Context) error {
		if _, err := os.Stat(migrate.LacrosFirstRunPath); !os.IsNotExist(err) {
			return errors.Wrap(err, "'First Run' file exists or cannot be read")
		}
		return nil
	}, nil); err != nil {
		return errors.Wrap(err, "'First Run' file exists or cannot be read")
	}

	conn, err := cr.NewConn(ctx, simpleExtensionURL)
	if err != nil {
		return errors.Wrap(err, "failed to open extension page")
	}

	if err := conn.Call(ctx, nil, `() => {
		return new Promise((resolve, reject) => {
			chrome.storage.sync.set({key: 'value'}, () => {
				if (chrome.runtime.lastError) {
					reject(new Error(chrome.runtime.lastError.message));
				}
				resolve();
			});
		});
	}`); err != nil {
		return errors.Wrap(err, "failed to call chrome.storage.sync.set()")
	}

	return nil
}

func verifyExtensionStateOnLacros(ctx context.Context, s *testing.State, cr *chrome.Chrome) {
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
	defer l.Close(ctx)

	conn, err := l.NewConn(ctx, simpleExtensionURL)
	if err != nil {
		s.Fatal("Failed to open extension page: ", err)
	}
	defer conn.Close()

	if err := conn.Call(ctx, nil, `() => {
		return new Promise((resolve, reject) => {
			chrome.storage.sync.get(['key'], (result) => {
				if (result.key == 'value') resolve();
				else reject();
			});
		});
	}`); err != nil {
		s.Fatal("Failed to get the expected value with chrome.storage.sync.get: ", err)
	}
}
