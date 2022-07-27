// Copyright 2022 The ChromiumOS Authors.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package org.chromium.arc.testapp.arcaudiotest;

import android.content.res.AssetFileDescriptor;
import java.io.BufferedInputStream;
import java.io.InputStream;
import java.lang.Thread;
import android.util.Log;

class SamplePlayerSineGenerator extends SamplePlayerBase {

    private final byte[] mData;

    SamplePlayerSineGenerator(int sampleRate, int encoding, int channelConfig, int[] freqs) {
        super(sampleRate, encoding, channelConfig);
        mData = new byte[sampleRate * freqs.length * 2]; // For 1 second of data

        double A = 3200; // Amplitude, set to about 0.1 of max value (max = 32768)

        // generate 1 second sine wave
        int curByte = 0;
        for(int t = 0;t < sampleRate;t++) {
            for(int channel = 0;channel < freqs.length;channel++) {
                short y = 0;
                if(freqs[channel] != 0) {
                    y = (short)(A*(Math.sin(2 * Math.PI * freqs[channel] * t/(double)sampleRate)));
                }

                // Write to mData as 16 bit format
                mData[curByte++] = (byte)(y      & 0xFF);
                mData[curByte++] = (byte)((y>>8) & 0xFF);
            }
        }
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
