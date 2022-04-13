Name:              3DHC_D1_C
Specification:     The bitstream is coded with random access configuration as well as 3-view configuration.
                   In addition to inter-view sample prediction for depth, SDC is enabled for depth coding.
Functional stage:  Decoding of three views with SDC enabled for depth.
Purpose:           To conform the SDC in random access configurations.

Software:          HTM-15.1
Configuration:     (based on random access default configuration)
                   #========== depth coding tools ==========
                   IntraWedgeFlag                      : 0
                   IntraContourFlag                    : 0
                   IntraSdcFlag                        : 1
                   DLT                                 : 0
                   QTL                                 : 0
                   QtPredFlag                          : 0
                   InterSdcFlag                        : 0
                   MpiFlag                             : 0
                   DepthIntraSkip                      : 0

Picture size:      1024x768
Frame rate:        30 frames/s
Sequence:          Kendo
Num. frames:       50
