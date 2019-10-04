// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package vimcompile provides functionality to compile vim package using crostini containers and outputs the time taken to compile it
package vimcompile

import (
	"context"
	"time"

	"chromiumos/tast/local/perf"
	"chromiumos/tast/local/testexec"
	"chromiumos/tast/local/vm"
	"chromiumos/tast/testing"
)

const(
	// numberOfIterations is set to the number of times vim is to be compiled
	numberOfIterations = 5
	// shaValue is from vim github where we are checking out
	shaValue = "fbbd10"
)

// RunTest compiles vim multiple times and captures the average amount of time taken to compile it.
func RunTest(ctx context.Context, s *testing.State, cont *vm.Container) {
	configureVim := "cd /home/testuser/vim/src && ./configure"
	makeVim := "cd /home/testuser/vim/src && make -j > /dev/null"
	removeVim := "cd /home/testuser && rm -rf vim"
	untarVim := "cd /home/testuser && tar -xvf vim.tar.gz"
	collectTime := make([]float64, numberOfIterations)
	i := 0

	setupTest(ctx, s, cont)

	s.Log("Compiling vim ",numberOfIterations," times")
	for i < numberOfIterations {
		//Configuring vim package
		s.Log("Configuring vim")
		if err := cont.Command(ctx, "sh","-c",configureVim).Run(testexec.DumpLogOnError); err != nil {
			s.Error("Failed to configure vim package: ", err)
		}

		//Compiling vim package
		s.Log("Iteration number ",i+1," to make vim package is in progress...")
		start := time.Now()
		if err := cont.Command(ctx, "sh","-c",makeVim).Run(testexec.DumpLogOnError); err != nil {
			s.Fatal("Failed to make vim package: ", err)
		}
		endTime := time.Since(start)
		s.Log("Amount of time taken to compile vim in ",i+1," iteration: ",endTime)
		collectTime[i] = float64(endTime)

		//Removing vim directory
		s.Log("Removing vim directory")
		if err := cont.Command(ctx, "sh","-c",removeVim).Run(testexec.DumpLogOnError); err != nil {
			s.Fatal("Failed to remove vim package: ", err)
		}

		//Untaring vim packge for subsequent iterations
		s.Log("Untaring vim package")
		if err := cont.Command(ctx, "sh","-c",untarVim).Run(testexec.DumpLogOnError); err != nil {
			s.Fatal("Failed to untar vim package: ", err)
		}
		i++
	}

	//Calculating average time to compile vim
	var sumTime float64
	for i := 0; i < len(collectTime); i++ {
		sumTime += collectTime[i]
	}
	avgTime := sumTime/float64(len(collectTime))
	avgTime = avgTime/float64(time.Millisecond)
	s.Log("Average time to compile vim: ", avgTime,"ms")

	//Saving data to Chrome Performance Dashboard - https://chromeperf.appspot.com/
	pv := perf.NewValues()
	pv.Set(perf.Metric{
		Name: "compile_time",
		Unit: "milliseconds",
		Direction: perf.SmallerIsBetter,
	}, avgTime)
	if err:= pv.Save(s.OutDir()); err != nil {
		s.Error("Failed to save average compile time due to: ",err)
	}
}

// setupTest sets up the device to compile vim
func setupTest(ctx context.Context, s *testing.State, cont *vm.Container) {
	installLibs := "sudo apt-get install -y gcc make libncurses5-dev libncursesw5-dev"
	fixMissingLibs := "sudo apt-get update --fix-missing"
	cloneVim := "git clone https://github.com/vim/vim.git"
	checkoutVimSha := "cd /home/testuser/vim/src && git checkout "+shaValue
	tarVim := "tar -czvf vim.tar.gz vim"

	s.Log("Installing required packages to compile vim")
	if err := cont.Command(ctx, "sh","-c",installLibs).Run(testexec.DumpLogOnError); err != nil {
		s.Fatal("Failed to install packages: ", err)
	}

	s.Log("Installing missing dependencies")
	if err := cont.Command(ctx, "sh","-c",fixMissingLibs).Run(testexec.DumpLogOnError); err != nil {
		s.Fatal("Failed to update dependencies: ", err)
	}

	s.Log("Cloning vim from github")
	if err := cont.Command(ctx, "sh","-c",cloneVim).Run(testexec.DumpLogOnError); err != nil {
		s.Fatal("Failed to clone vim package: ", err)
	}

	//This is required to ensure we measure the same content each time
	s.Log("Checking out a specific tag in vim")
	if err := cont.Command(ctx, "sh","-c",checkoutVimSha).Run(testexec.DumpLogOnError); err != nil {
		s.Error("Failed to check out a specific tag due to: ", err)
	}

	s.Log("Compressing and saving the vim folder")
	if err := cont.Command(ctx, "sh","-c",tarVim).Run(testexec.DumpLogOnError); err != nil {
		s.Error("Failed to compress vim folder: ", err)
	}
}
