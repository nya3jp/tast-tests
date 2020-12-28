# Test app for testing the NaCl technology

This directory contains the source files of the Chrome App that is used by the
Tast test for exercising the Native Client technology.

## NaCl module preparation

The NaCl module used by this Chrome App is compiled from a simple test program
from the Chromium repository:

  src/chrome/test/data/nacl/simple.cc

The NaCl module is precompiled and injected into the test via data files, since
compiling it requires setting up the NaCl toolchain (which would be cumbersome
and very time-consuming to perform during the test).

In accordance with the Tast guidelines, only small textual files are stored in
this repository, meanwhile the actual PNaCl module binary (nacl_module.pexe) is
stored as an external data file. If a new version of it needs to be generated,
the following command could be used (which assumes that you have the Chromium
checkout and the NaCl toolchain set up):

${NACL_SDK_ROOT}/toolchain/linux_pnacl/bin/pnacl-clang++ \
  -I${NACL_SDK_ROOT}/include \
  -L${NACL_SDK_ROOT}/lib/pnacl/Release \
  ${CHROME_SRC_ROOT}/chrome/test/data/nacl/simple.cc \
  -O2 -lppapi -lppapi_cpp \
  -o nacl_module.non_finalized.pexe \
  && \
  ${NACL_SDK_ROOT}/toolchain/linux_pnacl/bin/pnacl-finalize \
  nacl_module.non_finalized.pexe \
  -o nacl_module.pexe
