package types

// fieldopt.go reads the web3.v1.field option off a column's Source descriptor.
// The per-chain type projections consult it for their type override; a nil
// descriptor (a synthesized column) or an unannotated field yields the empty
// option, so the neutral projection applies.

import (
	"github.com/the-protobuf-project/protokit/schema"
	"github.com/the-protobuf-project/web3/plugin/pb/web3v1"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protoreflect"
)

func fieldOptions(d protoreflect.FieldDescriptor) *web3v1.FieldOptions {
	if d == nil || !proto.HasExtension(d.Options(), web3v1.E_Field) {
		return &web3v1.FieldOptions{}
	}
	return proto.GetExtension(d.Options(), web3v1.E_Field).(*web3v1.FieldOptions)
}

// Indexed reports whether a column is marked as an indexed event parameter via
// web3.v1.field.indexed (EVM only; nil/unannotated → false).
func Indexed(col *schema.Column) bool {
	return fieldOptions(col.Source).GetIndexed()
}
