// Copyright 2022 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package tape

import (
	"crypto/md5"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"strings"

	"chromiumos/tast/errors"
)

// LeasedAccount returns the username and password of a leased account.
// When an account is leased, the username is written to a location on the DUT. This
// function reads that username, matches it with the credentials,
// which are structured as a list and stored in tast-tests-private, and returns the
// corresponding username and password. This prevents passwords from being leaked
// on the DUT.
func LeasedAccount(creds, poolID string) (username, password string, err error) {
	// Read the account data.
	data, err := ioutil.ReadFile(LocalDUTAccountFileLocation(poolID))
	if err != nil {
		return "", "", errors.Wrap(err, "failed to read account file")
	}

	// Parse the account data.
	var parsedData LeasedAccountFileData
	if err := json.Unmarshal(data, &parsedData); err != nil {
		return "", "", errors.Wrap(err, "failed to unmarshal data")
	}

	return findAccount(creds, parsedData.Username)
}

// findAccount parses through the provided credentials for a given username,
// and returns the corresponding username, and password if it is found.
func findAccount(creds, leasedUsername string) (username, password string, err error) {
	for _, line := range strings.Split(creds, "\n") {
		line = strings.TrimSpace(line)
		if len(line) == 0 || strings.HasPrefix(line, "#") {
			continue
		}

		ps := strings.SplitN(line, ":", 2)
		if len(ps) != 2 {
			return "", "", errors.New("failed to parse credential list")
		}

		if ps[0] == leasedUsername {
			return ps[0], ps[1], nil
		}
	}

	return "", "", errors.New("failed to find leased account in list of credentials")
}

// LocalDUTAccountFileLocation returns the location on the DUT that the
// account file will be written to. This is unique per poolID i.e.
// poolID=`roblox` will always return the same path location, which is required
// since both local, and remote function calls rely on a consistent path.
func LocalDUTAccountFileLocation(poolID string) string {
	// Create a file-safe name that is unique per pool ID.
	hash := md5.Sum([]byte(poolID))
	hashVal := hex.EncodeToString(hash[:])

	// Return the path to the file.
	return fmt.Sprintf("/tmp/tape_%s.json", hashVal)
}
