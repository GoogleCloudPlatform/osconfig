package pretty

import (
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"
)

// Format uses jsonpb to marshal a proto for pretty printing.
func Format(pb proto.Message) string {
	m := &protojson.MarshalOptions{Indent: "  ", AllowPartial: true, UseProtoNames: true, EmitUnpopulated: true, UseEnumNumbers: false}
	return m.Format(pb)
}
