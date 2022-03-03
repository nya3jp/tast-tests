// Copyright 2022 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package uci contains utility functions to wrap around the uci program.
//
// The UCI (United Configuration Interface) system is used to centralize the
// configuration of OpenWrt services.
//
// See https://openwrt.org/docs/guide-user/base-system/uci for documentation on
// the UCI system and https://openwrt.org/docs/guide-user/base-system/uci#command-line_utility
// for documentation on the uci CLI which this package uses.
package uci

import (
	"bytes"
	"context"
	"strings"

	"chromiumos/tast/common/network/cmd"
	"chromiumos/tast/errors"
	remoteCmd "chromiumos/tast/remote/network/cmd"
	"chromiumos/tast/ssh"
)

const (
	uciCmd = "uci"
)

// CLI flags
const (
	flagC          string = "-c" // <path>; set the search path for config files (default: /etc/config)
	FlagD                 = "-d" // set the delimiter for list values in uci show
	flagF                 = "-f" // <file>; use <file> as input instead of stdin
	FlagM                 = "-m" // when importing, merge data into an existing package
	FlagLowercaseN        = "-n" // name unnamed sections on export (default)
	FlagUppercaseN        = "-N" // don't name unnamed sections
	flagLowercaseP        = "-p" // <path>; add a search path for config change files
	flagUppercaseP        = "-P" // <path>; add a search path for config change files and use as default
	FlagQ                 = "-q" // quiet mode (don't print error messages)
	FlagLowercaseS        = "-s" // force strict mode (stop on parser errors, default)
	FlagUppercaseS        = "-S" // disable strict mode
	FlagX                 = "-X" // do not use extended syntax on 'show'
)

// FlagC builds the parameterized CLI flag to set the search path for config.
// files (default: /etc/config).
func FlagC(path string) string {
	return flagC + path
}

// FlagF builds the parameterized CLI flag to use <file> as input instead of stdin.
func FlagF(file string) string {
	return flagF + file
}

// FlagLowercaseP builds the parameterized CLI flag to add a search path for
// config change files.
func FlagLowercaseP(path string) string {
	return flagLowercaseP + path
}

// FlagUppercaseP builds the parameterized CLI flag to add a search path for
// config change files and use as default.
func FlagUppercaseP(path string) string {
	return flagUppercaseP + path
}

// Runner contains methods for running the uci command.
type Runner struct {
	cmd cmd.Runner
}

// NewRunner creates uci command runner.
func NewRunner(cmd cmd.Runner) *Runner {
	return &Runner{
		cmd: cmd,
	}
}

// NewRemoteRunner creates an uci runner for remote execution.
func NewRemoteRunner(host *ssh.Conn) *Runner {
	return NewRunner(&remoteCmd.RemoteCmdRunner{Host: host})
}

// Uci runs the uci command with the given arguments and CLI flags.
func (r *Runner) Uci(ctx context.Context, flags []string, args ...string) error {
	args = append(flags, args...)
	if err := r.cmd.Run(ctx, uciCmd, args...); err != nil {
		return errors.Wrapf(err, `failed to run "uci %s"`, strings.Join(args, " "))
	}
	return nil
}

// UciWithOutput runs the uci command with the given arguments and CLI flags and
// returns the output lines in a list.
func (r *Runner) UciWithOutput(ctx context.Context, flags []string, args ...string) ([]string, error) {
	args = append(flags, args...)
	output, err := r.cmd.Output(ctx, uciCmd, args...)
	if err != nil {
		return nil, errors.Wrapf(err, `failed to run "uci %s"`, strings.Join(args, " "))
	}
	return strings.Split(strings.TrimSpace(string(output)), "\n"), nil
}

// buildLocation builds a location string that follows the uci compressed notation.
//
// Each parameter is increasingly more specific, and should only be provided if
// the former is given (option is the exception).
//
// The format with all provided is "<config>.<section>.<option>=<value>".
func (r *Runner) buildLocation(config, section, option, value string) (string, error) {
	var location bytes.Buffer
	if config != "" {
		location.WriteString(config)
		if section != "" {
			location.WriteString(".")
			location.WriteString(section)
			if option != "" {
				location.WriteString(".")
				location.WriteString(option)
			}
			if value != "" {
				location.WriteString("=")
				location.WriteString(value)
			}
		} else if option != "" || value != "" {
			return "", errors.New("location section is required if an option or value is given")
		}
	} else if section != "" || option != "" || value != "" {
		return "", errors.New("location config is required if a section, option, or value is given")
	}
	return location.String(), nil
}

// Commit writes changes of the given configuration file, or if none is given,
// all configuration files, to the filesystem.
//
// All “uci set”, “uci add”, “uci rename” and “uci delete” commands are staged
// into a temporary location and written to flash at once with “uci commit”.
// This must be called after editing any of the mentioned commands to save their
// changes.
//
// CLI usage is "uci commit [<config>]".
func (r *Runner) Commit(ctx context.Context, config string, flags ...string) error {
	if config != "" {
		return r.Uci(ctx, flags, "commit", config)
	}
	return r.Uci(ctx, flags, "commit")
}

// Revert reverts the given option, section or configuration file.
//
// CLI usage is "uci revert <config>[.<section>[.<option>]]".
func (r *Runner) Revert(ctx context.Context, config, section, option string, flags ...string) error {
	if config == "" {
		return errors.Errorf("invalid location parameter with config=%q section=%q option=%q: config is required", config, section, option)
	}
	location, err := r.buildLocation(config, section, option, "")
	if err != nil {
		return errors.Errorf("invalid location parameter with config=%q section=%q option=%q", config, section, option)
	}
	return r.Uci(ctx, flags, "revert", location)
}

// Changes lists staged changes to the given configuration file or if none
// given, all configuration files.
//
// CLI usage is "uci changes [<config>]".
func (r *Runner) Changes(ctx context.Context, config string, flags ...string) ([]string, error) {
	if config != "" {
		return r.UciWithOutput(ctx, flags, "changes", config)
	}
	return r.UciWithOutput(ctx, flags, "changes")
}

// Show shows the given option, section or configuration in compressed notation.
//
// CLI usage is "uci show [<config>[.<section>[.<option>]]]".
func (r *Runner) Show(ctx context.Context, config, section, option string, flags ...string) ([]string, error) {
	location, err := r.buildLocation(config, section, option, "")
	if err != nil {
		return nil, errors.Wrapf(err, "invalid location parameter with config=%q section=%q option=%q", config, section, option)
	}
	if location != "" {
		return r.UciWithOutput(ctx, flags, "show", location)
	}
	return r.UciWithOutput(ctx, flags, "show")
}

// Set sets the value of the given option, or adds a new section with the type
// set to the given value.
//
// CLI usage is "uci set <config>.<section>[.<option>]=<value>".
func (r *Runner) Set(ctx context.Context, config, section, option, value string, flags ...string) error {
	if config == "" || section == "" || value == "" {
		return errors.Errorf("invalid location parameter with config=%q section=%q option=%q value=%q: config, section, and value are required", config, section, option, value)
	}
	location, err := r.buildLocation(config, section, option, value)
	if err != nil {
		return errors.Errorf("invalid location parameter with config=%q section=%q option=%q value=%q", config, section, option, value)
	}
	return r.Uci(ctx, flags, "set", location)
}

// Get gets the value of the given option or the type of the given section.
//
// CLI usage is "uci get <config>.<section>[.<option>]".
func (r *Runner) Get(ctx context.Context, config, section, option string, flags ...string) ([]string, error) {
	if config == "" || section == "" {
		return nil, errors.Errorf("invalid location parameter with config=%q section=%q option=%q: config and section are required", config, section, option)
	}
	location, err := r.buildLocation(config, section, option, "")
	if err != nil {
		return nil, errors.Errorf("invalid location parameter with config=%q section=%q option=%q", config, section, option)
	}
	return r.UciWithOutput(ctx, flags, "get", location)
}

// Delete deletes the given section or option.
//
// CLI usage is "uci delete <config>.<section>[.<option>]".
func (r *Runner) Delete(ctx context.Context, config, section, option string, flags ...string) ([]string, error) {
	if config == "" || section == "" {
		return nil, errors.Errorf("invalid location parameter with config=%q section=%q option=%q: config and section are required", config, section, option)
	}
	location, err := r.buildLocation(config, section, option, "")
	if err != nil {
		return nil, errors.Errorf("invalid location parameter with config=%q section=%q option=%q", config, section, option)
	}
	return r.UciWithOutput(ctx, flags, "delete", location)
}

// DelList removes the given string from an existing list option.
//
// CLI usage is "uci del_list <config>.<section>.<option>=<str>".
func (r *Runner) DelList(ctx context.Context, config, section, option, str string, flags ...string) error {
	if config == "" || section == "" || option == "" || str == "" {
		return errors.Errorf("invalid location parameter with config=%q section=%q option=%q str=%q: config, section, option, and str are required", config, section, option, str)
	}
	location, err := r.buildLocation(config, section, option, str)
	if err != nil {
		return errors.Errorf("invalid location parameter with config=%q section=%q option=%q str=%q", config, section, option, str)
	}
	return r.Uci(ctx, flags, "del_list", location)
}

// AddList adds the given string to an existing list option.
//
// CLI usage is "uci add_list <config>.<section>.<option>=<str>".
func (r *Runner) AddList(ctx context.Context, config, section, option, str string, flags ...string) error {
	if config == "" || section == "" || option == "" || str == "" {
		return errors.Errorf("invalid location parameter with config=%q section=%q option=%q str=%q: config, section, option, and str are required", config, section, option, str)
	}
	location, err := r.buildLocation(config, section, option, str)
	if err != nil {
		return errors.Errorf("invalid location parameter with config=%q section=%q option=%q str=%q", config, section, option, str)
	}
	return r.Uci(ctx, flags, "add_list", location)
}

// Add adds an anonymous section of type sectionType to the given configuration.
//
// CLI usage is "uci add <config> <sectionType>".
func (r *Runner) Add(ctx context.Context, config, sectionType string, flags ...string) error {
	if config == "" || sectionType == "" {
		return errors.New("config and sectionType are required")
	}
	return r.Uci(ctx, flags, "add", config, sectionType)
}

// Rename renames the given option or section to the given name.
//
// CLI usage is "uci rename <config>.<section>[.<option>]=<name>".
func (r *Runner) Rename(ctx context.Context, config, section, option, name string, flags ...string) error {
	if config == "" || section == "" {
		return errors.Errorf("invalid location parameter with config=%q section=%q option=%q name=%q: config and section are required", config, section, option, name)
	}
	location, err := r.buildLocation(config, section, option, name)
	if err != nil {
		return errors.Errorf("invalid location parameter with config=%q section=%q option=%q name=%q", config, section, option, name)
	}
	return r.Uci(ctx, flags, "rename", location)
}

// Reorder moves a section to another position.
//
// CLI usage is "uci reorder <config>.<section>=<position>".
func (r *Runner) Reorder(ctx context.Context, config, section, position string, flags ...string) error {
	if config == "" || section == "" || position == "" {
		return errors.Errorf("invalid location parameter with config=%q section=%q value=%q: config, section, and position are required", config, section, position)
	}
	location, err := r.buildLocation(config, section, "", position)
	if err != nil {
		return errors.Errorf("invalid location parameter with config=%q section=%q position=%q", config, section, position)
	}
	return r.Uci(ctx, flags, "reorder", location)
}

// Import imports configuration files in UCI syntax.
//
// CLI usage is "uci import [<config>]".
func (r *Runner) Import(ctx context.Context, config string, flags ...string) error {
	if config != "" {
		return r.Uci(ctx, flags, "import", config)
	}
	return r.Uci(ctx, flags, "import")
}

// Export exports the configuration in a machine-readable format.
//
// CLI usage is "uci export [<config>]".
func (r *Runner) Export(ctx context.Context, config string, flags ...string) ([]string, error) {
	if config != "" {
		return r.UciWithOutput(ctx, flags, "export", config)
	}
	return r.UciWithOutput(ctx, flags, "export")
}

// Batch executes a multi-line UCI script which is typically wrapped into a
// "here" document syntax. This command is not supported by this wrapper and
// an error will always be returned if this is called.
func (r *Runner) Batch() error {
	return errors.New(`the "uci batch" command is not supported by this wrapper`)
}
