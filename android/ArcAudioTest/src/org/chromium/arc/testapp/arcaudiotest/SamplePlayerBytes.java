// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package org.chromium.arc.testapp.arcaudiotest;

import android.content.res.AssetFileDescriptor;
import java.io.BufferedInputStream;
import java.io.InputStream;

class SamplePlayerBytes extends SamplePlayerBase {

    private final byte[] mData;

    SamplePlayerBytes(int sampleRate, int encoding, int channelConfig) {
        super(sampleRate, encoding, channelConfig);
        mData = new byte[128 * 1024];
    }

    SamplePlayerBytes(int sampleRate, int encoding, int channelConfig, AssetFileDescriptor fd)
        throws Exception {
        super(sampleRate, encoding, channelConfig);
        mData = loadRawResourceBytes(fd);
    }

    private byte[] loadRawResourceBytes(AssetFileDescriptor fd) throws Exception {
        long length = fd.getLength();
        byte[] buffer = new byte[(int) length];

        try (InputStream is = fd.createInputStream();
            BufferedInputStream bis = new BufferedInputStream(is)) {
            bis.read(buffer);
        }
        return buffer;
    }

    @Override
    protected int writeBlock(int numBytes) {
        int result = 0;
        int bytesToWrite = Math.min(numBytes, mData.length - mOffset);
        if (bytesToWrite > 0) {
            result = mTrack.write(mData, mOffset, bytesToWrite);
            mOffset += result;
        } else {
            mOffset = 0; // rewind
        }
        return result;
    }
}
