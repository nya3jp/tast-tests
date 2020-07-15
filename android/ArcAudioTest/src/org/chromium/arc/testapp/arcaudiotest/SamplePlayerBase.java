// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package org.chromium.arc.testapp.arcaudiotest;

import android.media.AudioManager;
import android.media.AudioTrack;
import android.util.Log;

public abstract class SamplePlayerBase {
    private final int mSampleRate;
    private final int mEncoding;
    private final int mChannelConfig;
    protected int mOffset = 0;
    protected AudioTrack mTrack;
    private int mBlockSize = 512;

    SamplePlayerBase(int sampleRate, int encoding, int channelConfig) {
        mSampleRate = sampleRate;
        mEncoding = encoding;
        mChannelConfig = channelConfig;
    }

    // Use abstract write to handle byte[] or short[] data.
    protected abstract int writeBlock(int numSamples);

    private AudioTrack createAudioTrack(int sampleRate, int encoding, int channelConfig) {
        int minBufferSize = AudioTrack.getMinBufferSize(sampleRate, channelConfig, encoding);
        Log.i(Constant.TAG, String.format("getMinBufferSize = %d", minBufferSize));
        int bufferSize = minBufferSize * 3; // plenty big
        AudioTrack track =
                new AudioTrack(
                        AudioManager.STREAM_MUSIC,
                        sampleRate,
                        channelConfig,
                        encoding,
                        bufferSize,
                        AudioTrack.MODE_STREAM);
        Log.i(
            Constant.TAG,
                String.format(
                        "track created using rate = %d, encoding = 0x%08x",
                        mSampleRate, mEncoding));
        return track;
    }

    public void play() throws Exception {
        final long TEST_DURATION_MILLIS = 5000;
        mTrack = createAudioTrack(mSampleRate, mEncoding, mChannelConfig);
        try {
            mTrack.play();
            long elapsedMillis = 0;
            long startTime = System.currentTimeMillis();
            while (elapsedMillis < TEST_DURATION_MILLIS) {
                writeBlock(mBlockSize);
                elapsedMillis = System.currentTimeMillis() - startTime;
            }
        } finally {
            mOffset = 0;
            mTrack.release();
        }
    }
}
