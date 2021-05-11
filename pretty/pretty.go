package pretty

import (
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"
)

var marshalOptions protojson.MarshalOptions

func init() {
	marshalOptions = protojson.MarshalOptions{Indent: "  ", AllowPartial: true, UseProtoNames: true, EmitUnpopulated: true, UseEnumNumbers: false}
}

// MarshalOptions returns protojson options used in Format.
func MarshalOptions() protojson.MarshalOptions {
	return marshalOptions
}

// Format uses jsonpb to marshal a proto for pretty printing.
func Format(pb proto.Message) string {
	return marshalOptions.Format(pb)
}
