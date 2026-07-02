package types

// fabric.go projects the neutral schema.FieldType onto a Go type for the
// Hyperledger Fabric chaincode target: asset struct fields and transaction
// arguments. A repeated field becomes a []T; a JSON aggregate stays a single
// string. Fabric derives Go types directly, so there is no per-field override.

import "github.com/the-protobuf-project/protokit/schema"

// goForType maps a neutral FieldType to the Go type a Fabric chaincode asset
// uses. Amounts/counters keep their width; timestamps are unix seconds (int64);
// decimals and free-form aggregates stay strings for lossless JSON round-trips.
var goForType = map[schema.FieldType]string{
	schema.TypeString:    "string",
	schema.TypeBool:      "bool",
	schema.TypeInt32:     "int32",
	schema.TypeUint32:    "uint32",
	schema.TypeInt64:     "int64",
	schema.TypeUint64:    "uint64",
	schema.TypeFloat:     "float32",
	schema.TypeDouble:    "float64",
	schema.TypeBytes:     "[]byte",
	schema.TypeTimestamp: "int64",
	schema.TypeDuration:  "int64",
	schema.TypeDate:      "int64",
	schema.TypeTimeOfDay: "int64",
	schema.TypeDecimal:   "string",
	schema.TypeLatLng:    "string",
	schema.TypeInterval:  "string",
	schema.TypeText:      "string",
	schema.TypeJSON:      "string",
	schema.TypeULID:      "string",
	schema.TypeUUID:      "string",
}

// FabricGo returns the Go type for a neutral FieldType, wrapping a repeated field
// in []T (a JSON aggregate stays a single string).
func FabricGo(t schema.FieldType, list bool) string {
	base := goForType[t]
	if base == "" {
		base = "string"
	}
	if list && t != schema.TypeJSON {
		return "[]" + base
	}
	return base
}

// FabricGoForColumn returns the effective Go type of a column for the chaincode
// target: the neutral FieldType projection (enum columns are handled by the caller
// via schema.Column.Enum).
func FabricGoForColumn(col *schema.Column) string {
	return FabricGo(col.Type, col.List)
}
