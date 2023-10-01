package water

type Feature uint64

// Feature is a bit mask of experimental features of WATER.
//
// TODO: implement Feature.
const (
	FEATURE_DUMMY    Feature = 1 << iota // a dummy feature that does nothing.
	FEATURE_RESERVED                     // reserved for future use
	// ...
	FEATURE_CWAL Feature = 0xFFFFFFFFFFFFFFFF // CWAL = Can't Wait Any Longer
	FEATURE_NONE Feature = 0                  // NONE = No Experimental Features
)
