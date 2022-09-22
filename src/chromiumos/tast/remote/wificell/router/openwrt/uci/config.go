// Copyright 2022 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package uci

import (
	"context"
	"path"

	"chromiumos/tast/common/utils"
	"chromiumos/tast/errors"
	"chromiumos/tast/testing"
)

// Constants for relevant config names as specified in the OpenWrt
// documentation here: https://openwrt.org/docs/guide-user/base-system/uci#configuration_files.
const (
	// ConfigDHCP is the name of config for dnsmasq and odhcpd
	// settings: DNS, DHCP, DHCPv6.
	ConfigDHCP = "dhcp"

	// ConfigNetwork is the name of the config for switch, interface and
	// route configuration: Basics, IPv4, IPv6, Routes, Rules, WAN, Aliases,
	// Switches, VLAN, IPv4/IPv6 transitioning, Tunneling.
	ConfigNetwork = "network"

	// ConfigWireless is the name of the config for wireless settings and
	// Wi-Fi network definition.
	ConfigWireless = "wireless"
)

// Relevant system directories
const (
	configDir       = "/etc/config"
	configBackupDir = "/etc/config/backup/"
)

// BackupConfig copies the config file, located at configDir/config, to the
// backupPath. If no backupFilePath is specified, it defaults to using
// configBackupDir/config. Returns the path to the newly created backup
// file. If the backupFilePath does not contain a directory, it is assumed
// that the target directory is configBackupDir.
//
// If a file exists at backupFilePath already, it is overwritten.
//
// Note: A "uci commit" is not necessary after running this, as it manually
// changes the config files and does not go through the uci CLI. However, to
// make the config changes used, any corresponding services that use it must
// be reloaded.
func BackupConfig(ctx context.Context, uci *Runner, config, backupFilePath string) (string, error) {
	// Resolve src and dst file paths.
	if backupFilePath == "" {
		backupFilePath = path.Join(configBackupDir, config)
	}
	srcConfigFile := path.Join(configDir, config)
	// Ensure backup dir exists.
	dstBackupDir, _ := path.Split(backupFilePath)
	if dstBackupDir == "" {
		dstBackupDir = configBackupDir
		backupFilePath = path.Join(configBackupDir, backupFilePath)
	}
	testing.ContextLogf(ctx, "Backing up current OpenWrt router UCI config for %q to backup config file %q", config, backupFilePath)
	if err := uci.cmd.Run(ctx, "mkdir", "-p", dstBackupDir); err != nil {
		return "", errors.Wrapf(err, "failed to create backup directory %q for destination backup file %q", dstBackupDir, backupFilePath)
	}
	// Backup config
	if err := uci.cmd.Run(ctx, "cp", srcConfigFile, backupFilePath); err != nil {
		return "", errors.Wrapf(err, "failed to backup file with 'cp %q %q'", srcConfigFile, backupFilePath)
	}
	return backupFilePath, nil
}

// RestoreConfig restores the config by replacing the existing config file,
// located at configDir/config, with a previously backed up copy located
// at backupFilePath. If no backupFilePath is specified, it defaults to using
// configBackupDir/config. If backupFilePath does not exist and
// ignoreMissingBackup is true, the config will not be changed and the function
// will return false. Returns true if the target config was updated from an
// existing backup.
//
// Note: A "uci commit" is not necessary after running this, as it manually
// changes the config files and does not go through the uci CLI. However, to
// make the config changes used, any corresponding services that use it must
// be reloaded.
func RestoreConfig(ctx context.Context, uci *Runner, config, backupFilePath string, ignoreMissingBackup bool) (bool, error) {
	// Resolve src and dst file paths.
	if backupFilePath == "" {
		backupFilePath = path.Join(configBackupDir, config)
	}
	dstConfigFile := path.Join(configDir, config)

	var err error
	updated := false
	_, err = uci.cmd.Output(ctx, "test", "-f", backupFilePath)
	if err == nil {
		testing.ContextLogf(ctx, "Restoring OpenWrt router UCI config %q with backup config file %q", config, backupFilePath)
		err = uci.cmd.Run(ctx, "cp", backupFilePath, dstConfigFile)
		updated = true
	} else if !ignoreMissingBackup {
		err = errors.Wrapf(err, "backup file existence test failed for file %q", backupFilePath)
	} else {
		testing.ContextLogf(ctx, "Skipping OpenWrt router UCI config %q restoration as backup config file %q does not exist", config, backupFilePath)
		return false, nil
	}
	if err != nil {
		return updated, errors.Wrapf(err, "failed to restore %q from backup file %q", dstConfigFile, backupFilePath)
	}
	return updated, nil
}

// ResetConfigs restores the given configs to their backed up states. If
// expectBackupsToExist is false, configs are backed up again after they are
// restored to ensure a backup exists.
//
// Note: A "uci commit" is not necessary after running this, as it manually
// changes the config files and does not go through the uci CLI. However, to
// make the config changes used, any corresponding services that use them must
// be reloaded.
func ResetConfigs(ctx context.Context, uci *Runner, expectBackupsToExist bool, configs ...string) error {
	var firstRestoreError error
	for _, config := range configs {
		// Restore from backup
		if updated, err := RestoreConfig(ctx, uci, config, "", !expectBackupsToExist); err != nil {
			utils.CollectFirstErr(ctx, &firstRestoreError, errors.Wrapf(err, "failed to restore config %q from backup", config))
		} else if !updated && !expectBackupsToExist {
			// Backup what is there since a backup does not exist yet
			if _, err := BackupConfig(ctx, uci, config, ""); err != nil {
				return errors.Wrapf(err, "failed to backup config %q", config)
			}
		}
	}
	return firstRestoreError
}

// ReloadConfigServices reloads any service that uses any of the specified
// configs with the current committed configuration settings.
func ReloadConfigServices(ctx context.Context, uci *Runner, configs ...string) error {
	for _, config := range configs {
		var err error
		switch config {
		case ConfigWireless:
			err = ReloadWifi(ctx, uci)
		case ConfigDHCP:
			err = RestartDnsmasq(ctx, uci)
		case ConfigNetwork:
			err = ReloadNetwork(ctx, uci)
		default:
			err = errors.New("unknown config or reloading its services not yet supported")
		}
		if err != nil {
			return errors.Wrapf(err, "failed to reload config %q", config)
		}
	}
	return nil
}

// ReloadWifi reloads Wi-Fi, which uses the current committed settings in
// that are in ConfigWireless.
func ReloadWifi(ctx context.Context, uci *Runner) error {
	testing.ContextLog(ctx, "Reloading OpenWrt router wifi settings")
	if err := uci.cmd.Run(ctx, "wifi", "reload"); err != nil {
		return errors.Wrap(err, "failed to reload wifi settings")
	}
	return nil
}

// RestartDnsmasq restarts the dnsmasq service, which reloads the current
// committed settings that are in ConfigDHCP. See the OpenWrt documentation
// for more info at https://openwrt.org/docs/guide-user/base-system/dhcp.
func RestartDnsmasq(ctx context.Context, uci *Runner) error {
	testing.ContextLog(ctx, "Restarting OpenWrt router dnsmasq service")
	if err := uci.cmd.Run(ctx, "/etc/init.d/dnsmasq", "restart"); err != nil {
		return errors.Wrap(err, "failed to restart dnsmasq service")
	}
	return nil
}

// ReloadNetwork reloads the network service with the current committed settings
// in ConfigNetwork. See the OpenWrt documentation for more info at https://openwrt.org/docs/guide-user/base-system/basic-networking#network_configuration_management.
func ReloadNetwork(ctx context.Context, uci *Runner) error {
	testing.ContextLog(ctx, "Reloading OpenWrt router network service")
	if err := uci.cmd.Run(ctx, "/etc/init.d/network", "reload"); err != nil {
		return errors.Wrap(err, "failed to reload network service")
	}
	return nil
}

// CommitAndReloadConfig first commits any pending config changes and then
// reloads its dependent service to put the changes into effect.
func CommitAndReloadConfig(ctx context.Context, uci *Runner, config string) error {
	testing.ContextLogf(ctx, "Committing and reloading changes to UCI config %q", config)
	if err := uci.Commit(ctx, config); err != nil {
		return errors.Wrapf(err, "failed to commit changes to config %q", config)
	}
	if err := ReloadConfigServices(ctx, uci, config); err != nil {
		return err
	}
	return nil
}
