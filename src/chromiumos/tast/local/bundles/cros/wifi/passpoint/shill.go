// Copyright 2022 The ChromiumOS Authors.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package passpoint

import (
	"context"
	"fmt"
	"math/rand"
	"os"
	"path/filepath"
	"strconv"
	"time"

	"github.com/godbus/dbus"

	"chromiumos/tast/common/crypto/certificate"
	"chromiumos/tast/common/shillconst"
	"chromiumos/tast/ctxutil"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/shill"
	"chromiumos/tast/local/sysutil"
	"chromiumos/tast/testing"
)

const (
	testUser        = "test-user"
	testPassword    = "test-password"
	testPackageName = "app.passpoint.example.com"
	fakeProfilesDir = "/run/shill/user_profiles"
)

// Credentials represents a set of Passpoint credentials with selection criteria
type Credentials struct {
	// Domain of the service provider
	Domain string
	// List of organisation identifiers (OI)
	HomeOIs []uint64
	// List of required organisation identifiers
	RequiredHomeOIs []uint64
	// List of roaming compatible OIs
	RoamingOIs []uint64
}

// ToProperties converts the set of credentials to a map for credentials D-Bus
// properties.
func (pc *Credentials) ToProperties() map[string]interface{} {
	certs := certificate.TestCert1()
	props := map[string]interface{}{
		shillconst.PasspointCredentialsPropertyDomains:            []string{pc.Domain},
		shillconst.PasspointCredentialsPropertyRealm:              pc.Domain,
		shillconst.PasspointCredentialsPropertyMeteredOverride:    false,
		shillconst.PasspointCredentialsPropertyAndroidPackageName: testPackageName,
		shillconst.ServicePropertyEAPMethod:                       "TTLS",
		shillconst.ServicePropertyEAPInnerEAP:                     "auth=MSCHAPV2",
		shillconst.ServicePropertyEAPIdentity:                     testUser,
		shillconst.ServicePropertyEAPPassword:                     testPassword,
		shillconst.ServicePropertyEAPCACertPEM:                    []string{certs.CACred.Cert},
	}

	if len(pc.HomeOIs) > 0 {
		var ois []string
		for _, oi := range pc.HomeOIs {
			ois = append(ois, strconv.FormatUint(oi, 10))
		}
		props[shillconst.PasspointCredentialsPropertyHomeOIs] = ois
	}

	if len(pc.RequiredHomeOIs) > 0 {
		var ois []string
		for _, oi := range pc.RequiredHomeOIs {
			ois = append(ois, strconv.FormatUint(oi, 10))
		}
		props[shillconst.PasspointCredentialsPropertyRequiredHomeOIs] = ois
	}

	if len(pc.RoamingOIs) > 0 {
		var ois []string
		for _, oi := range pc.RoamingOIs {
			ois = append(ois, strconv.FormatUint(oi, 10))
		}
		props[shillconst.PasspointCredentialsPropertyRoamingConsortia] = ois
	}

	return props
}

// SetInterworkingSelectEnabled sets the "interworking enabled" property to
// |enabled| for the Wi-Fi device |iface|.
func SetInterworkingSelectEnabled(ctx context.Context, m *shill.Manager, iface string, enabled bool) error {
	dev, err := m.DeviceByName(ctx, iface)
	if err != nil {
		return errors.Wrapf(err, "failed to obtain device for interface %s", iface)
	}
	err = dev.PropertyHolder.SetProperty(ctx, shillconst.DevicePropertyPasspointInterworkingSelectEnabled, enabled)
	if err != nil {
		return errors.Wrapf(err, "failed to set interworking selection on %s", iface)
	}
	return nil
}

// RandomProfileName returns a random name for Shill test profile.
func RandomProfileName() string {
	const letters = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"

	s := make([]byte, 8)
	for i := range s {
		s[i] = letters[rand.Intn(len(letters))]
	}
	return "passpoint" + string(s)
}

// CreateFakeUserProfile creates a fake user profile in Shill on top of the
// default profile for test purpose.
func CreateFakeUserProfile(ctx context.Context, m *shill.Manager, name string) (path dbus.ObjectPath, err error) {
	// Shorten deadline to leave time for cleanup.
	cleanupCtx := ctx
	ctx, cancel := ctxutil.Shorten(ctx, 10*time.Second)
	defer cancel()

	// To be a Shill user profile, the profile name must match the format "~user/identifier".
	profileName := fmt.Sprintf("~%s/shill", name)
	profilePath := filepath.Join(fakeProfilesDir, name)

	// Pop all user profiles to be sure there's no Passpoint configuration leftovers.
	// Note: Passpoint credentials can't be pushed to default profiles.
	if err := m.PopAllUserProfiles(ctx); err != nil {
		return "", errors.Wrap(err, "failed to pop user profiles")
	}

	// Remove the test profile if it still exists.
	p, err := m.ProfileByName(ctx, profileName)
	if !errors.Is(err, shill.ErrProfileNotFound) {
		return "", err
	} else if p != nil {
		if err := m.RemoveProfile(ctx, profileName); err != nil {
			return "", errors.Wrapf(err, "failed to remove profile %s", profileName)
		}
	}

	// Obtain Shill UID et GID
	uid, err := sysutil.GetUID("shill")
	if err != nil {
		return "", errors.Wrap(err, "failed to obtain Shill user id")
	}
	gid, err := sysutil.GetGID("shill")
	if err != nil {
		return "", errors.Wrap(err, "failed to obtain Shill group id")
	}

	// Create a directory for the test user profile
	if err := os.MkdirAll(profilePath, 0700); err != nil {
		return "", errors.Wrap(err, "failed to create profile dir")
	}
	defer func(ctx context.Context) {
		if err != nil {
			if err := os.RemoveAll(profilePath); err != nil {
				testing.ContextLogf(ctx, "Failed to remove %s: %v", profilePath, err)
			}
		}
	}(cleanupCtx)
	if err := os.Chown(fakeProfilesDir, int(uid), int(gid)); err != nil {
		return "", errors.Wrap(err, "failed to chown")
	}
	if err := os.Chown(profilePath, int(uid), int(gid)); err != nil {
		return "", errors.Wrap(err, "failed to chown")
	}

	// Create and push the profile
	if _, err = m.CreateProfile(ctx, profileName); err != nil {
		return "", errors.Wrap(err, "failed to create profile")
	}
	defer func(ctx context.Context) {
		if err != nil {
			if err := m.RemoveProfile(ctx, profileName); err != nil {
				testing.ContextLogf(ctx, "Failed to remove profile %s: %v", profileName, err)
			}
		}
	}(cleanupCtx)
	path, err = m.PushProfile(ctx, profileName)
	if err != nil {
		return "", errors.Wrap(err, "failed to push profile")
	}

	return path, nil
}

// RemoveFakeUserProfile removes the fake Shill user profile created for test purpose.
func RemoveFakeUserProfile(ctx context.Context, m *shill.Manager, name string) error {
	profileName := fmt.Sprintf("~%s/shill", name)
	profilePath := filepath.Join(fakeProfilesDir, name)

	if err := m.PopProfile(ctx, profileName); err != nil {
		return errors.Wrapf(err, "failed to pop profile %s", profileName)
	}
	if err := m.RemoveProfile(ctx, profileName); err != nil {
		return errors.Wrapf(err, "failed to remove profile %s", profileName)
	}
	if err := os.RemoveAll(profilePath); err != nil {
		return errors.Wrapf(err, "failed to remove profile directory %s", profilePath)
	}
	return nil
}
