-	Bitstream file name
-	-	log2_min_transform_block_size_minus2_2_Samsung_1.bin
-	Explanation of bitstream features
-	-	Test exercises CTU size 64 with depth 2 and log2_min_transform_block_size_minus2 = 2.
-	-	So minimum CU size is 32x32 and miminun transform size for Luma is 16x16 (Chroma 8x8).
-	-	Bit streams consist of I and P slices.
-	Minimum level of this bitstream (or picture size)
-	-	5
-	Frame rate
-	-	30

Specification:
All slices are coded as I, or P slices. Each picture contains only one slice. log2_min_transform_block_size_minus2 is set equal to 2.
For current bit stream Maximum CU size is 64x64, miminun CU size is 32x32, miminun transform size for Luma is 16x16 (for Chroma 8x8).
Functional stage:
Test the reconstruction process of slices with limited minimum transfrom size.
Purpose:
Check if decoder properly decodes slices with RQT with minimum transfrom size different from 4x4 (default).

