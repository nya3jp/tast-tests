Name:              3DHC_D1_D
Specification:     The bitstream is coded with AI configuration as well as 3-view configuration.
                   In addition, SDC is enabled for depth coding.
Functional stage:  Decoding of three views with SDC enabled for depth.
Purpose:           To conform SDC in All-Intra configuration.

Software:          HTM-15.1
Configuration:     (based on all-intra default configuration)
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