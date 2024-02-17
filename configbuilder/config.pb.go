package configbuilder

import "github.com/refraction-networking/water/configbuilder/pb"

// ConfigProtoBuf defines the Protobuf format of the Config.
//
// This struct may fail to fully represent the Config struct, as it is
// non-trivial to represent a func or other non-serialized structures.
type ConfigProtoBuf = pb.Config
