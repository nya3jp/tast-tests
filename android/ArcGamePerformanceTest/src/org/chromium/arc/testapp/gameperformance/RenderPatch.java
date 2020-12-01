/*
 * Copyright 2020 The Chromium OS Authors. All rights reserved.
 * Use of this source code is governed by a BSD-style license that can be
 * found in the LICENSE file.
 */

package org.chromium.arc.testapp.gameperformance;

import java.nio.Buffer;
import java.nio.ByteBuffer;
import java.nio.ByteOrder;

/**
 * Helper class that generates patch to render. Patch is a regular polygon with the center in 0.
 * Regular polygon fits in circle with requested radius.
 */
public class RenderPatch {
    public static final int FLOAT_SIZE = 4;
    public static final int SHORT_SIZE = 2;
    public static final int VERTEX_COORD_COUNT = 3;
    public static final int VERTEX_STRIDE = VERTEX_COORD_COUNT * FLOAT_SIZE;
    public static final int TEXTURE_COORD_COUNT = 2;
    public static final int TEXTURE_STRIDE = TEXTURE_COORD_COUNT * FLOAT_SIZE;

    // Tessellation is done using points on circle.
    public static final int TESSELLATION_BASE = 0;
    // Tessellation is done using extra point in 0.
    public static final int TESSELLATION_TO_CENTER = 1;

    // Radius of circle that fits polygon.
    private final float mDimension;

    private final ByteBuffer mVertexBuffer;
    private final ByteBuffer mTextureBuffer;
    private final ByteBuffer mIndexBuffer;

    public RenderPatch(int triangleCount, float dimension, int tessellation) {
        mDimension = dimension;

        int pointCount;
        int externalPointCount;

        if (triangleCount < 1) {
            throw new IllegalArgumentException("Too few triangles to perform tessellation");
        }

        switch (tessellation) {
            case TESSELLATION_BASE:
                externalPointCount = triangleCount + 2;
                pointCount = externalPointCount;
                break;
            case TESSELLATION_TO_CENTER:
                if (triangleCount < 3) {
                    throw new IllegalArgumentException(
                            "Too few triangles to perform tessellation to center");
                }
                externalPointCount = triangleCount;
                pointCount = triangleCount + 1;
                break;
            default:
                throw new IllegalArgumentException("Wrong tesselation requested");
        }

        if (pointCount > Short.MAX_VALUE) {
            throw new IllegalArgumentException("Number of requested triangles is too big");
        }

        mVertexBuffer = ByteBuffer.allocateDirect(pointCount * VERTEX_STRIDE);
        mVertexBuffer.order(ByteOrder.nativeOrder());

        mTextureBuffer = ByteBuffer.allocateDirect(pointCount * TEXTURE_STRIDE);
        mTextureBuffer.order(ByteOrder.nativeOrder());

        for (int i = 0; i < externalPointCount; ++i) {
            // Use 45 degree rotation to make quad aligned along axes in case
            // triangleCount is four.
            final double angle = Math.PI * 0.25 + (Math.PI * 2.0 * i) / (externalPointCount);
            // Positions
            mVertexBuffer.putFloat((float) (dimension * Math.sin(angle)));
            mVertexBuffer.putFloat((float) (dimension * Math.cos(angle)));
            mVertexBuffer.putFloat(0.0f);
            // Texture coordinates.
            mTextureBuffer.putFloat((float) (0.5 + 0.5 * Math.sin(angle)));
            mTextureBuffer.putFloat((float) (0.5 - 0.5 * Math.cos(angle)));
        }

        if (tessellation == TESSELLATION_TO_CENTER) {
            // Add center point.
            mVertexBuffer.putFloat(0.0f);
            mVertexBuffer.putFloat(0.0f);
            mVertexBuffer.putFloat(0.0f);
            mTextureBuffer.putFloat(0.5f);
            mTextureBuffer.putFloat(0.5f);
        }

        mIndexBuffer =
                ByteBuffer.allocateDirect(
                        triangleCount * 3 /* indices per triangle */ * SHORT_SIZE);
        mIndexBuffer.order(ByteOrder.nativeOrder());

        switch (tessellation) {
            case TESSELLATION_BASE:
                for (int i = 0; i < triangleCount; ++i) {
                    mIndexBuffer.putShort((short) 0);
                    mIndexBuffer.putShort((short) (i + 1));
                    mIndexBuffer.putShort((short) (i + 2));
                }
                break;
            case TESSELLATION_TO_CENTER:
                for (int i = 0; i < triangleCount; ++i) {
                    mIndexBuffer.putShort((short) i);
                    mIndexBuffer.putShort((short) ((i + 1) % externalPointCount));
                    mIndexBuffer.putShort((short) externalPointCount);
                }
                break;
        }

        if (mVertexBuffer.remaining() != 0
                || mTextureBuffer.remaining() != 0
                || mIndexBuffer.remaining() != 0) {
            throw new RuntimeException("Failed to fill buffers");
        }

        // Upcast needed, see:
        // https://github.com/robolectric/robolectric/pull/4894/commits/70abf0475100e78b4984cab40a07ba4c2fe52184
        ((Buffer)mVertexBuffer).position(0);
        ((Buffer)mTextureBuffer).position(0);
        ((Buffer)mIndexBuffer).position(0);
    }

    public float getDimension() {
        return mDimension;
    }

    public ByteBuffer getVertexBuffer() {
        return mVertexBuffer;
    }

    public ByteBuffer getTextureBuffer() {
        return mTextureBuffer;
    }

    public ByteBuffer getIndexBuffer() {
        return mIndexBuffer;
    }
}
