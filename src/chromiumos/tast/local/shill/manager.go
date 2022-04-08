// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package shill

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/godbus/dbus"

	"chromiumos/tast/common/shillconst"
	"chromiumos/tast/ctxutil"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/dbusutil"
	"chromiumos/tast/local/sysutil"
	"chromiumos/tast/testing"
	"chromiumos/tast/timing"
)

const (
	dbusManagerPath      = "/" // crosbug.com/20135
	dbusManagerInterface = "org.chromium.flimflam.Manager"
	fakeProfilesDir      = "/run/shill/user_profiles"
)

// Exported constants
const (
	EnableWaitTime = 500 * time.Millisecond
)

// Manager wraps a Manager D-Bus object in shill.
type Manager struct {
	*PropertyHolder
}

// Technology is the type of a shill device's technology
type Technology string

// Device technologies
// Refer to Flimflam type options in
// https://chromium.googlesource.com/chromiumos/platform2/+/refs/heads/main/system_api/dbus/shill/dbus-constants.h#334
const (
	TechnologyCellular Technology = shillconst.TypeCellular
	TechnologyEthernet Technology = shillconst.TypeEthernet
	TechnologyPPPoE    Technology = shillconst.TypePPPoE
	TechnologyVPN      Technology = shillconst.TypeVPN
	TechnologyWifi     Technology = shillconst.TypeWifi
)

// ErrProfileNotFound is the error returned by ProfileByName when the requested
// profile does not exist in Shill.
var ErrProfileNotFound = errors.New("profile not found")

// NewManager connects to shill's Manager.
func NewManager(ctx context.Context) (*Manager, error) {
	ph, err := NewPropertyHolder(ctx, dbusService, dbusManagerInterface, dbusManagerPath)
	if err != nil {
		return nil, err
	}
	return &Manager{PropertyHolder: ph}, nil
}

// FindMatchingService returns the first Service that matches |expectProps|.
// If no matching Service is found, returns shillconst.ErrorMatchingServiceNotFound.
// Note that the complete list of Services is searched, including those with Visible=false.
// To find only visible services, please specify Visible=true in expectProps.
func (m *Manager) FindMatchingService(ctx context.Context, expectProps map[string]interface{}) (*Service, error) {
	ctx, st := timing.Start(ctx, "m.FindMatchingService")
	defer st.End()

	var servicePath dbus.ObjectPath
	if err := m.Call(ctx, "FindMatchingService", expectProps).Store(&servicePath); err != nil {
		return nil, err
	}
	service, err := NewService(ctx, servicePath)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to instantiate service %s", servicePath)
	}
	return service, nil
}

// WaitForServiceProperties returns the first matching Service who has the expected properties.
// If there's no matching service, it polls until timeout is reached.
// Noted that it searches all services including Visible=false ones. To focus on visible services,
// please specify Visible=true in expectProps.
func (m *Manager) WaitForServiceProperties(ctx context.Context, expectProps map[string]interface{}, timeout time.Duration) (*Service, error) {
	var service *Service
	if err := testing.Poll(ctx, func(ctx context.Context) (e error) {
		service, e = m.FindMatchingService(ctx, expectProps)
		return e
	}, &testing.PollOptions{Timeout: timeout}); err != nil {
		return nil, err
	}
	return service, nil
}

// ProfilePaths returns a list of profile paths.
func (m *Manager) ProfilePaths(ctx context.Context) ([]dbus.ObjectPath, error) {
	p, err := m.GetProperties(ctx)
	if err != nil {
		return nil, err
	}
	return p.GetObjectPaths(shillconst.ManagerPropertyProfiles)
}

// Profiles returns a list of profiles.
func (m *Manager) Profiles(ctx context.Context) ([]*Profile, error) {
	paths, err := m.ProfilePaths(ctx)
	if err != nil {
		return nil, err
	}

	profiles := make([]*Profile, len(paths))
	for i, path := range paths {
		profiles[i], err = NewProfile(ctx, path)
		if err != nil {
			return nil, err
		}
	}
	return profiles, nil
}

// ActiveProfile returns the active profile.
func (m *Manager) ActiveProfile(ctx context.Context) (*Profile, error) {
	props, err := m.GetProperties(ctx)
	if err != nil {
		return nil, err
	}
	path, err := props.GetObjectPath(shillconst.ManagerPropertyActiveProfile)
	if err != nil {
		return nil, err
	}
	return NewProfile(ctx, path)
}

// Devices returns a list of devices.
func (m *Manager) Devices(ctx context.Context) ([]*Device, error) {
	p, err := m.GetProperties(ctx)
	if err != nil {
		return nil, err
	}
	paths, err := p.GetObjectPaths(shillconst.ManagerPropertyDevices)
	if err != nil {
		return nil, err
	}
	devs := make([]*Device, 0, len(paths))
	for _, path := range paths {
		d, err := NewDevice(ctx, path)
		if err != nil {
			return nil, err
		}
		devs = append(devs, d)
	}
	return devs, nil
}

// DeviceByType returns a device matching |type| or a "Device not found" error.
func (m *Manager) DeviceByType(ctx context.Context, deviceType string) (*Device, error) {
	devices, err := m.Devices(ctx)
	if err != nil {
		return nil, err
	}
	for _, d := range devices {
		properties, err := d.GetProperties(ctx)
		if err != nil {
			return nil, err
		}
		t, err := properties.GetString(shillconst.DevicePropertyType)
		if err != nil {
			return nil, err
		}
		if t == deviceType {
			return d, nil
		}
	}
	return nil, errors.New("Device not found")
}

// ConfigureService configures a service with the given properties and returns its path.
func (m *Manager) ConfigureService(ctx context.Context, props map[string]interface{}) (dbus.ObjectPath, error) {
	var service dbus.ObjectPath
	if err := m.Call(ctx, "ConfigureService", props).Store(&service); err != nil {
		return "", errors.Wrap(err, "failed to configure service")
	}
	return service, nil
}

// ConfigureServiceForProfile configures a service at the given profile path.
func (m *Manager) ConfigureServiceForProfile(ctx context.Context, path dbus.ObjectPath, props map[string]interface{}) (dbus.ObjectPath, error) {
	var service dbus.ObjectPath
	if err := m.Call(ctx, "ConfigureServiceForProfile", path, props).Store(&service); err != nil {
		return "", errors.Wrap(err, "failed to configure service")
	}
	return service, nil
}

// CreateProfile creates a profile.
func (m *Manager) CreateProfile(ctx context.Context, name string) (dbus.ObjectPath, error) {
	var profile dbus.ObjectPath
	if err := m.Call(ctx, "CreateProfile", name).Store(&profile); err != nil {
		return "", errors.Wrap(err, "failed to create profile")
	}
	return profile, nil
}

// PushProfile pushes a profile.
func (m *Manager) PushProfile(ctx context.Context, name string) (dbus.ObjectPath, error) {
	var profile dbus.ObjectPath
	if err := m.Call(ctx, "PushProfile", name).Store(&profile); err != nil {
		return "", errors.Wrap(err, "failed to create profile")
	}
	return profile, nil
}

// RemoveProfile removes the profile with the given name.
func (m *Manager) RemoveProfile(ctx context.Context, name string) error {
	return m.Call(ctx, "RemoveProfile", name).Err
}

// PopProfile pops the profile with the given name if it is on top of the stack.
func (m *Manager) PopProfile(ctx context.Context, name string) error {
	return m.Call(ctx, "PopProfile", name).Err
}

// ProfileByName returns a profile matching |name| or an ErrProfileNotFound if
// the profile does not exist.
func (m *Manager) ProfileByName(ctx context.Context, name string) (*Profile,
	error) {
	profiles, err := m.Profiles(ctx)
	if err != nil {
		return nil, err
	}

	for _, p := range profiles {
		properties, err := p.GetProperties(ctx)
		if err != nil {
			return nil, err
		}

		n, err := properties.GetString(shillconst.ProfilePropertyName)
		if err != nil {
			return nil, err
		}

		if n == name {
			return p, nil
		}
	}
	return nil, ErrProfileNotFound
}

// PopAllUserProfiles removes all user profiles from the stack of managed profiles leaving only default profiles.
func (m *Manager) PopAllUserProfiles(ctx context.Context) error {
	return m.Call(ctx, "PopAllUserProfiles").Err
}

// RequestScan requests a scan for the specified technology.
func (m *Manager) RequestScan(ctx context.Context, technology Technology) error {
	return m.Call(ctx, "RequestScan", string(technology)).Err
}

// EnableTechnology enables a technology interface.
func (m *Manager) EnableTechnology(ctx context.Context, technology Technology) error {
	return m.Call(ctx, "EnableTechnology", string(technology)).Err
}

// DisableTechnology disables a technology interface.
func (m *Manager) DisableTechnology(ctx context.Context, technology Technology) error {
	return m.Call(ctx, "DisableTechnology", string(technology)).Err
}

// GetEnabledTechnologies returns a list of all enabled shill networking technologies.
func (m *Manager) GetEnabledTechnologies(ctx context.Context) ([]Technology, error) {
	var enabledTechnologies []Technology
	prop, err := m.GetProperties(ctx)
	if err != nil {
		return enabledTechnologies, errors.Wrap(err, "failed to get properties")
	}
	technologies, err := prop.GetStrings(shillconst.ManagerPropertyEnabledTechnologies)
	if err != nil {
		return enabledTechnologies, errors.Wrapf(err, "failed to get property: %s", shillconst.ManagerPropertyEnabledTechnologies)
	}
	for _, t := range technologies {
		enabledTechnologies = append(enabledTechnologies, Technology(t))
	}
	return enabledTechnologies, nil
}

func (m *Manager) hasTechnology(ctx context.Context, technologyProperty string, technology Technology) (bool, error) {
	prop, err := m.GetProperties(ctx)
	if err != nil {
		return false, errors.Wrap(err, "failed to get properties")
	}
	technologies, err := prop.GetStrings(technologyProperty)
	if err != nil {
		return false, errors.Wrapf(err, "failed to get property: %s", technologyProperty)
	}
	for _, t := range technologies {
		if t == string(technology) {
			return true, nil
		}
	}
	return false, nil
}

// IsAvailable returns true if a technology is available.
func (m *Manager) IsAvailable(ctx context.Context, technology Technology) (bool, error) {
	return m.hasTechnology(ctx, shillconst.ManagerPropertyAvailableTechnologies, technology)
}

// IsEnabled returns true if a technology is enabled.
func (m *Manager) IsEnabled(ctx context.Context, technology Technology) (bool, error) {
	return m.hasTechnology(ctx, shillconst.ManagerPropertyEnabledTechnologies, technology)
}

// DisableTechnologyForTesting first checks whether |technology| is enabled.
// If it is enabled, it disables the technology and returns a callback that can
// be deferred to re-enable the technology when the test completes.
// (The callback should take a contexted shortened by shill.EnableWaitTime).
// If the technology is already enabled, no work is done and a nil callback is returned
func (m *Manager) DisableTechnologyForTesting(ctx context.Context, technology Technology) (func(ctx context.Context), error) {
	enabled, err := m.IsEnabled(ctx, technology)
	if err != nil {
		return nil, errors.Wrapf(err, "unable to determine enabled state for %s", string(technology))
	}
	if !enabled {
		return nil, nil
	}
	if err := m.DisableTechnology(ctx, technology); err != nil {
		return nil, errors.Wrapf(err, "unable to disable technology: %s", string(technology))
	}
	return func(ctx context.Context) {
		m.EnableTechnology(ctx, technology)
		// Ensure the Enable completes before the test ends so that cleanup will succeed.
		const interval = 100 * time.Millisecond
		testing.Poll(ctx, func(ctx context.Context) error {
			enabled, err := m.IsEnabled(ctx, technology)
			if err != nil {
				return errors.Wrap(err, "failed to get enabled state")
			}
			if !enabled {
				return errors.New("not enabled")
			}
			return nil
		}, &testing.PollOptions{
			// Subtract |interval| seconds from EnableWaitTime for Timeout so that
			// this completes or throws an error before a context shortened by
			// EnableWaitTime times out.
			Timeout:  EnableWaitTime - interval,
			Interval: interval,
		})
	}, nil
}

// DevicesByTechnology returns list of Devices and their Properties snapshots of the specified technology.
func (m *Manager) DevicesByTechnology(ctx context.Context, technology Technology) ([]*Device, []*dbusutil.Properties, error) {
	var matches []*Device
	var props []*dbusutil.Properties

	devs, err := m.Devices(ctx)
	if err != nil {
		return nil, nil, err
	}

	for _, dev := range devs {
		p, err := dev.GetProperties(ctx)
		if err != nil {
			if dbusutil.IsDBusError(err, dbusutil.DBusErrorUnknownObject) {
				// This error is forgivable as a device may go down anytime.
				continue
			}
			return nil, nil, err
		}
		if devType, err := p.GetString(shillconst.DevicePropertyType); err != nil {
			testing.ContextLogf(ctx, "Error getting the type of the device %q: %v", dev, err)
			continue
		} else if devType != string(technology) {
			continue
		}
		matches = append(matches, dev)
		props = append(props, p)
	}
	return matches, props, nil
}

// DeviceByName returns the Device matching the given interface name.
func (m *Manager) DeviceByName(ctx context.Context, iface string) (*Device, error) {
	devs, err := m.Devices(ctx)
	if err != nil {
		return nil, err
	}

	for _, dev := range devs {
		p, err := dev.GetProperties(ctx)
		if err != nil {
			if dbusutil.IsDBusError(err, dbusutil.DBusErrorUnknownObject) {
				// This error is forgivable as a device may go down anytime.
				continue
			}
			return nil, err
		}
		if devIface, err := p.GetString(shillconst.DevicePropertyInterface); err != nil {
			testing.ContextLogf(ctx, "Error getting the device interface %q: %v", dev, err)
			continue
		} else if devIface == iface {
			return dev, nil
		}
	}
	return nil, errors.New("unable to find matching device")
}

// WaitForDeviceByName returns the Device matching the given interface name.
// If there's no match, it waits until one appears, or until timeout.
func (m *Manager) WaitForDeviceByName(ctx context.Context, iface string, timeout time.Duration) (*Device, error) {
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	pw, err := m.CreateWatcher(ctx)
	if err != nil {
		return nil, errors.Wrap(err, "failed to create a PropertiesWatcher")
	}
	defer pw.Close(ctx)

	for {
		if d, err := m.DeviceByName(ctx, iface); err == nil {
			return d, nil
		}

		if _, err := pw.WaitAll(ctx, shillconst.ManagerPropertyDevices); err != nil {
			return nil, err
		}
	}
}

// SetDebugTags sets the debug tags that are enabled for logging. "tags" is a list of valid tag names separated by "+".
// Shill silently ignores invalid flags.
func (m *Manager) SetDebugTags(ctx context.Context, tags []string) error {
	return m.Call(ctx, "SetDebugTags", strings.Join(tags, "+")).Err
}

// GetDebugTags gets the list of enabled debug tags. The list is represented as a string of tag names separated by "+".
func (m *Manager) GetDebugTags(ctx context.Context) ([]string, error) {
	var tags string
	if err := m.Call(ctx, "GetDebugTags").Store(&tags); err != nil {
		return nil, err
	}
	tagsArr := strings.Split(tags, "+")
	return tagsArr, nil
}

// SetDebugLevel sets the debugging level.
func (m *Manager) SetDebugLevel(ctx context.Context, level int) error {
	return m.Call(ctx, "SetDebugLevel", level).Err
}

// GetDebugLevel gets the enabled debug level.
func (m *Manager) GetDebugLevel(ctx context.Context) (int, error) {
	var level int
	if err := m.Call(ctx, "GetDebugLevel").Store(&level); err != nil {
		return 0, err
	}
	return level, nil
}

// RecheckPortal requests shill to rerun its captive portal detector.
func (m *Manager) RecheckPortal(ctx context.Context) error {
	return m.Call(ctx, "RecheckPortal").Err
}

// ClaimInterface assigns the ownership of "intf" to the specified "claimer".
// The claimer will prevent Shill to manage the device (see shill/doc/manager-api.txt).
func (m *Manager) ClaimInterface(ctx context.Context, claimer, intf string) error {
	return m.Call(ctx, "ClaimInterface", claimer, intf).Err
}

// ReleaseInterface takes the ownership of "intf" from the specified "claimer"
// to give it back to Shill.
func (m *Manager) ReleaseInterface(ctx context.Context, claimer, intf string) error {
	return m.Call(ctx, "ReleaseInterface", claimer, intf).Err
}

// AddPasspointCredentials adds a set of Passpoint credentials to "profile"
// using the method AddPasspointCredentials() from Shill Manager. "props" is a
// map of properties that describes the set of credentials, the keys are defined
// in tast/common/shillconst.
func (m *Manager) AddPasspointCredentials(ctx context.Context, profile dbus.ObjectPath, props map[string]interface{}) error {
	return m.Call(ctx, "AddPasspointCredentials", profile, props).Err
}

// SetInterworkingSelectEnabled sets the "interworking enabled" property to
// |enabled| for the Wi-Fi device |iface|.
func (m *Manager) SetInterworkingSelectEnabled(ctx context.Context, iface string, enabled bool) error {
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

// CreateFakeUserProfile creates a fake user profile in Shill on top of the
// default profile for test purpose.
func (m *Manager) CreateFakeUserProfile(ctx context.Context, name string) (path dbus.ObjectPath, err error) {
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
	if !errors.Is(err, ErrProfileNotFound) {
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
func (m *Manager) RemoveFakeUserProfile(ctx context.Context, name string) error {
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
