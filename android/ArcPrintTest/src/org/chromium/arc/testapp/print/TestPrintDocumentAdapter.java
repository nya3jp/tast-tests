/*
 * Copyright 2020 The Chromium OS Authors. All rights reserved.
 * Use of this source code is governed by a BSD-style license that can be
 * found in the LICENSE file.
 */

package org.chromium.arc.testapp.print;

import android.content.Context;
import android.graphics.Canvas;
import android.graphics.Color;
import android.graphics.Paint;
import android.graphics.pdf.PdfDocument;
import android.os.Bundle;
import android.os.CancellationSignal;
import android.os.ParcelFileDescriptor;
import android.print.PageRange;
import android.print.PrintAttributes;
import android.print.PrintDocumentAdapter;
import android.print.PrintDocumentInfo;
import android.print.pdf.PrintedPdfDocument;
import android.util.Log;

import java.io.FileOutputStream;
import java.io.IOException;

class TestPrintDocumentAdapter extends PrintDocumentAdapter {
    private static final String LOG_TAG = TestPrintDocumentAdapter.class.getSimpleName();

    private final int PAGE_COUNT = 10;

    private final Context mContext;
    private PrintAttributes mPrintAttributes;

    public TestPrintDocumentAdapter(Context context) {
        mContext = context;
    }

    @Override
    public void onLayout(
            PrintAttributes oldAttributes,
            PrintAttributes newAttributes,
            CancellationSignal cancellationSignal,
            LayoutResultCallback callback,
            Bundle extras) {
        Log.i(LOG_TAG, "onLayout() called.");

        boolean completed = false;
        try {
            if (cancellationSignal.isCanceled()) {
                Log.i(LOG_TAG, "Layout cancelled.");
                callback.onLayoutCancelled();
                completed = true;
                return;
            }

            PrintDocumentInfo info =
                    new PrintDocumentInfo.Builder("arc_print_test.pdf")
                            .setContentType(PrintDocumentInfo.CONTENT_TYPE_DOCUMENT)
                            .setPageCount(PAGE_COUNT)
                            .build();
            mPrintAttributes = newAttributes;
            callback.onLayoutFinished(info, !oldAttributes.equals(newAttributes));
            completed = true;
        } finally {
            if (!completed) {
                callback.onLayoutFailed(null);
            }
        }
    }

    @Override
    public void onWrite(
            PageRange[] pages,
            ParcelFileDescriptor fd,
            CancellationSignal cancellationSignal,
            WriteResultCallback callback) {
        Log.i(LOG_TAG, "onWrite() called.");

        boolean completed = false;
        try {
            if (cancellationSignal.isCanceled()) {
                Log.i(LOG_TAG, "Write cancelled.");
                try {
                    fd.close();
                } catch (IOException e) {
                    Log.e(LOG_TAG, "Failed to close FD:", e);
                }
                callback.onWriteCancelled();
                completed = true;
                return;
            }

            try {
                // Write all pages for simplicity. Unneeded pages will be removed by the
                // ArcPrintActiviy.
                writePagesToDocument(mContext, mPrintAttributes, fd, PAGE_COUNT);
                callback.onWriteFinished(new PageRange[] {PageRange.ALL_PAGES});
                completed = true;
            } catch (IOException e) {
                Log.e(LOG_TAG, "Failed to write pages.", e);
            }
        } finally {
            if (!completed) {
                callback.onWriteFailed(null);
            }
        }
    }

    private static void drawPage(PdfDocument.Page page, int pageNumber) {
        Canvas canvas = page.getCanvas();
        Paint textPaint = new Paint();
        textPaint.setColor(Color.RED);
        textPaint.setTextAlign(Paint.Align.CENTER);
        textPaint.setTextSize(200);
        int xPos = canvas.getWidth() / 2;
        int yPos =
                (int) ((canvas.getHeight() / 2) - ((textPaint.descent() + textPaint.ascent()) / 2));
        canvas.drawText(String.valueOf(pageNumber), xPos, yPos, textPaint);
    }

    private static void writePagesToDocument(
            Context context, PrintAttributes attributes, ParcelFileDescriptor output, int numPages)
            throws IOException {
        try (FileOutputStream fos = new FileOutputStream(output.getFileDescriptor())) {
            PrintedPdfDocument document = new PrintedPdfDocument(context, attributes);
            for (int i = 0; i < numPages; i++) {
                PdfDocument.Page page = document.startPage(i);
                drawPage(page, i + 1);
                document.finishPage(page);
            }
            document.writeTo(fos);
            fos.flush();
            document.close();
        }
    }
}
