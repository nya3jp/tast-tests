/*
 * Copyright 2021 The Chromium OS Authors. All rights reserved.
 * Use of this source code is governed by a BSD-style license that can be
 * found in the LICENSE file.
 */

package org.chromium.arc.testapp.chromewebapk;

import android.content.ContentProvider;
import android.content.ContentValues;
import android.database.Cursor;
import android.database.MatrixCursor;
import android.net.Uri;
import android.os.ParcelFileDescriptor;
import android.provider.OpenableColumns;

import java.io.IOException;
import java.io.OutputStream;

/**
 * A {@link ContentProvider} which serves simple files directly from memory.
 *
 * <p>This is a simple alternative to {@code FileProvider} which does not require AndroidX
 * dependencies and does not perform disk access.
 */
public final class InMemoryContentProvider extends ContentProvider {

    private static final String FILE_CONTENTS_1 = "{\"text\": \"foobar\"}";
    private static final String FILE_CONTENTS_2 = "{\"text\": \"lorem ipsum\"}";

    static Uri getContentUri(String fileName) {
        return Uri.parse("content://org.chromium.arc.testapp.chromewebapk.content/" + fileName);
    }

    @Override
    public boolean onCreate() {
        return true;
    }

    @Override
    public Cursor query(
            Uri uri,
            String[] projection,
            String selection,
            String[] selectionArgs,
            String sortOrder) {
        final MatrixCursor cursor = new MatrixCursor(projection);
        final MatrixCursor.RowBuilder row = cursor.newRow();
        String fileName = uri.getLastPathSegment();
        for (int i = 0; i < projection.length; i++) {
            switch (projection[i]) {
                case OpenableColumns.DISPLAY_NAME:
                    row.add(projection[i], fileName);
                    break;
                case OpenableColumns.SIZE:
                    row.add(projection[i], getFileBytes(fileName).length);
                    break;
            }
        }
        return cursor;
    }

    @Override
    public String getType(Uri uri) {
        return "application/json";
    }

    @Override
    public Uri insert(Uri uri, ContentValues values) {
        return null;
    }

    @Override
    public int delete(Uri uri, String selection, String[] selectionArgs) {
        return 0;
    }

    @Override
    public int update(Uri uri, ContentValues values, String selection, String[] selectionArgs) {
        return 0;
    }

    @Override
    public ParcelFileDescriptor openFile(Uri uri, String mode) {
        ParcelFileDescriptor readFd;
        ParcelFileDescriptor writeFd;
        try {
            ParcelFileDescriptor[] pipe = ParcelFileDescriptor.createPipe();
            readFd = pipe[0];
            writeFd = pipe[1];

            OutputStream stream = new ParcelFileDescriptor.AutoCloseOutputStream(writeFd);
            stream.write(getFileBytes(uri.getLastPathSegment()));
            stream.close();
        } catch (IOException e) {
            throw new RuntimeException(e);
        }

        return readFd;
    }

    private byte[] getFileBytes(String fileName) {
        if (fileName.equals("file2.json")) {
            return FILE_CONTENTS_2.getBytes();
        }

        return FILE_CONTENTS_1.getBytes();
    }
}
