// Copyright 2022 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package factory

const (
	// FactoryRootPath is the root path of installed factory toolkit.
	FactoryRootPath = "/usr/local/factory"
	// ToolkitEnabledPath is the path for a file that determines whether
	// toolkit is enabled.
	ToolkitEnabledPath = FactoryRootPath + "/enabled"
	// ToolkitVersionFilePath is the path for a file where it stores the
	// version of the toolkit installed.
	ToolkitVersionFilePath = FactoryRootPath + "/TOOLKIT_VERSION"
	// ActiveTestListFilePath is the path for a file where it stores the
	// name of test list that is currently active.
	ActiveTestListFilePath = FactoryRootPath + "/py/test/test_lists/active_test_list.json"
)
