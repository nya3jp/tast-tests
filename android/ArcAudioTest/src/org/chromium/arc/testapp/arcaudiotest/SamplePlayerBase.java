// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package org.chromium.arc.testapp.arcaudiotest;

import android.media.AudioAttributes;
import android.media.AudioFormat;
import android.media.AudioTrack;

public abstract class SamplePlayerBase {

    protected int mOffset = 0;
    protected AudioTrack mTrack;
    private int mBlockSize = 512;
    protected AudioTrack.Builder mTrackBuilder;
    private boolean mStop = false;

    public synchronized void setStop(boolean stop) {
        mStop = stop;
    }

    public synchronized boolean getStop() {
        return mStop;
    }

    SamplePlayerBase(int sampleRate, int encoding, int channelConfig) {
        int bufferSize =
            AudioTrack.getMinBufferSize(sampleRate, channelConfig, encoding) * 3; // plenty big
        mTrackBuilder = new AudioTrack.Builder();
        mTrackBuilder.setAudioFormat(new AudioFormat.Builder()
            .setEncoding(encoding)
            .setSampleRate(sampleRate)
            .setChannelMask(channelConfig)
            .build());
        mTrackBuilder.setBufferSizeInBytes(bufferSize);
    }

    // Use abstract write to handle byte[] or short[] data.
    protected abstract int writeBlock(int numSamples);

    public void setPerformanceMode(int performanceMode) {
        mTrackBuilder.setPerformanceMode(performanceMode);
    }

    public void setAudioAttributes(AudioAttributes attributes) {
        mTrackBuilder.setAudioAttributes(attributes);
    }

    public void play(long duration_millis) throws Exception {
        mTrack = mTrackBuilder.build();
        try {
            mTrack.play();
            long elapsedMillis = 0;
            long startTime = System.currentTimeMillis();
            while (elapsedMillis < duration_millis && !getStop()) {
                writeBlock(mBlockSize);
                elapsedMillis = System.currentTimeMillis() - startTime;
            }
        } finally {
            mOffset = 0;
            mTrack.release();
        }
    }
}
