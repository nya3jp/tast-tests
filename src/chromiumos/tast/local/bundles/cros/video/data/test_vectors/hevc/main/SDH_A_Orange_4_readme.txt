
SDH_A_Orange_4:

conformance: HM-10.0
category: Entropy coding
sub category: Sign Data Hiding

resolution: 1920x1080
# of frames: 2

Specification:
A. sign_data_hiding_flag is on:
	At least 100 CGs have lastSigScanPos-firstSigScanPos = 3.
	At least 2000 CGs have lastSigScanPos-firstSigScanPos < 3.
	At least 1000 CGs have lastSigScanPos-firstSigScanPos > 3 and infer the sign bit to be positive.
	At least 1000 CGS have lastSigScanPos-firstSigScanPos > 3 and infer the sign bit to be negative.
o	Testing position of CG with SDH:
	    Sign inference happens for all possible combinations of (TU size, CG position in TU) at least once in the frame.
o	Testing significance positions:
	    All possible combinations for (lastSigScanPos, firstSigScanPos ) in a CG appear at least once in the frame.
o	Testing the correctness of the inference of the sign (odd/even) and testing if the calculation of the sum creates
	overflows (e.g. implementations with 8, 16 or 32 bits adder):
    *	4x4 TU
     1.	lastSigScanPos-firstSigScanPos = 3, 4 combinations of (first sign bit, sum parity): (negative, even), (positive, odd).
     2.	lastSigScanPos-firstSigScanPos < 3, 4 combinations of (first sign bit, sum parity) : (negative, even), (positive, odd).
     3.	lastSigScanPos-firstSigScanPos > 3, first sign bit even, sum of signed values, not including initial sign-inferred coeff,
        include at least
	    (0, ±2, ±4, ±6, ±8, ±10, ±12, ±14, ±16, ±32, ±64, ±128, ±256, ±512, ±1024, ±2048, ±4096, ±8192, ±16384)
     4.	lastSigScanPos-firstSigScanPos  > 3, first sign bit odd, sum of signed values, not including initial sign-inferred coeff,
        include at least
	    (±1, ±3, ±5, ±7, ±9, ±11, ±13, ±15, ±17, ±33, ±129, ±257, ±513, ±1025, ±2049, ±4097, ±8193, ±16385)
     5.	One CG with all coefficients to maximum absolute value and sign inference.
     6.	One CG with all coefficients to minimum absolute value and sign inference.
    *	8x8 TU
     1.	Repeat the test cases for 4x4 TU for all 4 CGs.
    *	16x16 TU
     1.	Repeat the test cases for 4x4 TU for all 16 CGs.
    *	32x32 TU
     1.	Repeat the test cases for 4x4 TU for all 64 CGs.

B. sign_data_hiding_flag is on:
	At least 500 CGs have lastSigScanPos-firstSigScanPos > 3  and the CU they belong to has cu_transquant_bypass_flag=1.
	At least 500 CGs have lastSigScanPos-firstSigScanPos > 3  and the CU they belong to has cu_transquant_bypass_flag=0.



Purpose:
Test SDH sign inference, and test SDH in conjunction with cu_transquant_bypass_flag.

MD5: SEI message in the bitstream
