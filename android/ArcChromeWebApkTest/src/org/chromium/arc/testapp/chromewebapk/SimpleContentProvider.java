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

import java.io.BufferedOutputStream;
import java.io.File;
import java.io.FileNotFoundException;
import java.io.FileOutputStream;
import java.io.IOException;

/**
 * A {@link ContentProvider} which serves simple files with fixed content.
 *
 * <p>This is a basic alternative to {@code FileProvider} which does not require AndroidX
 * dependencies.
 */
public final class SimpleContentProvider extends ContentProvider {

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
    public ParcelFileDescriptor openFile(Uri uri, String mode) throws FileNotFoundException {
        // Write the content to a file in our private data directly. Performing blocking file IO
        // like this isn't ideal, but should be fast enough to avoid triggering an App Not
        // Responding error.
        File f = new File(getContext().getDataDir(), uri.getLastPathSegment());
        try (BufferedOutputStream bos = new BufferedOutputStream(new FileOutputStream(f))) {
            bos.write(getFileBytes(uri.getLastPathSegment()));
        } catch (IOException e) {
            throw new RuntimeException(e);
        }

        return ParcelFileDescriptor.open(f, ParcelFileDescriptor.MODE_READ_ONLY);
    }

    private byte[] getFileBytes(String fileName) {
        if (fileName.equals("file2.json")) {
            return FILE_CONTENTS_2.getBytes();
        }

        return FILE_CONTENTS_1.getBytes();
    }
}
