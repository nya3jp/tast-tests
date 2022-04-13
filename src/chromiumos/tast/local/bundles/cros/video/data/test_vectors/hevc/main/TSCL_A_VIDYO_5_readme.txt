Bitstream file name: TSCL_A_VIDYO_5.hevc
Conformance point: HM-10.0
Bitstream feature:
The bitstream is for testing temporal scalability.
The bitstream has 4 temporal layers, and was encoded with the following options as the prediction structure.

IntraPeriod                   : 32          # Period of I-Frame ( -1 = only first)
DecodingRefreshType           : 1           # Random Accesss 0:none, 1:CDR, 2:IDR
GOPSize                       : 8           # GOP Size (number of B slice = GOPSize-1)
#        Type POC QPoffset QPfactor tcOffsetDiv2 betaOffsetDiv2 temporal_id #ref_pics_active #ref_pics reference pictures     predict deltaRPS #ref_idcs reference idcs
Frame1:  B    8   1        0.442    0            0              0           2                2         -8 -16          0
Frame2:  B    4   2        0.3536   0            0              1           2                3         -4 -12 4        0
Frame3:  B    2   3        0.3536   0            0              2           2                4         -2 -10 2 6      0
Frame4:  B    1   4        0.68     0            0              3           2                5         -1 -9 1 3 7     0
Frame5:  B    3   4        0.68     0            0              3           2                5         -1 -3 -11 1 5   0
Frame6:  B    6   3        0.3536   0            0              2           2                4         -2 -4 -6 2      0
Frame7:  B    5   4        0.68     0            0              3           2                5         -1 -3 -5  1 3   0
Frame8:  B    7   4        0.68     0            0              3           2                5         -1 -3 -5 -7 1   0

picture size: 416x240 (BasketballPass_416x240_50.yuv)
frame rate: 50
number of encoded frames: 73

revision 2:
	fixes the illegal nal_unit_type in some of the reference frames
	
revision 3:
	fixes the illegal nal_unit_type in leading pictures for CRA pictures

revision 4:
	added proper profile and level
	
revision 5:
	fixes the issue with RPS in CRA pictures