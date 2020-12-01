/*
 * Copyright 2020 The Chromium OS Authors. All rights reserved.
 * Use of this source code is governed by a BSD-style license that can be
 * found in the LICENSE file.
 */

package org.chromium.arc.testapp.gameperformance;

import java.util.Random;

import android.opengl.Matrix;

/** Class that performs bouncing animation for RenderPatch on the screen. */
public class RenderPatchAnimation {
    private static final Random RANDOM = new Random();

    private final RenderPatch mRenderPatch;
    // Bounds of animation
    private final float mAvailableX;
    private final float mAvailableY;

    // Current position.
    private float mPosX;
    private float mPosY;
    // Direction of movement.
    private float mDirX;
    private float mDirY;

    private float[] mMatrix;

    public RenderPatchAnimation(RenderPatch renderPatch, float ratio) {
        mRenderPatch = renderPatch;

        mAvailableX = ratio - mRenderPatch.getDimension();
        mAvailableY = 1.0f - mRenderPatch.getDimension();

        mPosX = 2.0f * mAvailableX * RANDOM.nextFloat() - mAvailableX;
        mPosY = 2.0f * mAvailableY * RANDOM.nextFloat() - mAvailableY;
        mMatrix = new float[16];

        // Evenly distributed in cycle, normalized.
        while (true) {
            mDirX = 2.0f * RANDOM.nextFloat() - 1.0f;
            mDirY = mRenderPatch.getDimension() < 1.0f ? 2.0f * RANDOM.nextFloat() - 1.0f : 0.0f;

            final float length = (float) Math.sqrt(mDirX * mDirX + mDirY * mDirY);
            if (length <= 1.0f && length > 0.0f) {
                mDirX /= length;
                mDirY /= length;
                break;
            }
        }
    }

    public RenderPatch getRenderPatch() {
        return mRenderPatch;
    }

    /**
     * Performs the next update. t specifies the distance to travel along the direction. This checks
     * if patch goes out of screen and invert axis direction if needed.
     */
    public void update(float t) {
        mPosX += mDirX * t;
        mPosY += mDirY * t;
        if (mPosX < -mAvailableX) {
            mDirX = Math.abs(mDirX);
        } else if (mPosX > mAvailableX) {
            mDirX = -Math.abs(mDirX);
        }
        if (mPosY < -mAvailableY) {
            mDirY = Math.abs(mDirY);
        } else if (mPosY > mAvailableY) {
            mDirY = -Math.abs(mDirY);
        }
    }

    /** Returns Model/View/Projection transform for the patch. */
    public float[] getTransform(float[] vpMatrix) {
        Matrix.setIdentityM(mMatrix, 0);
        mMatrix[12] = mPosX;
        mMatrix[13] = mPosY;
        Matrix.multiplyMM(mMatrix, 0, vpMatrix, 0, mMatrix, 0);
        return mMatrix;
    }
}
