// Package types is web3's projection of the neutral schema.FieldType onto a
// Solidity type, used by the solidity/subgraph targets. It is the chain-specific
// half of the type system that used to live in protokit (types.SolidityType);
// protokit now carries only the neutral FieldType.
package types

import "github.com/the-protobuf-project/protokit/schema"

// solForType maps a neutral FieldType to a Solidity type, matching what
// SolidityType(PostgresType(...)) produced before the split: amounts/counters and
// timestamps widen to uint256, floats/decimals-without-lossless-form fall back to
// string, bytes → bytes. TypeEnum is handled by the caller via schema.Column.Enum.
var solForType = map[schema.FieldType]string{
	schema.TypeString:    "string",
	schema.TypeBool:      "bool",
	schema.TypeInt32:     "int32",
	schema.TypeUint32:    "int64",
	schema.TypeInt64:     "int64",
	schema.TypeUint64:    "uint256",
	schema.TypeFloat:     "string",
	schema.TypeDouble:    "string",
	schema.TypeBytes:     "bytes",
	schema.TypeTimestamp: "uint256",
	schema.TypeDuration:  "string",
	schema.TypeDate:      "uint256",
	schema.TypeTimeOfDay: "uint256",
	schema.TypeDecimal:   "uint256",
	schema.TypeLatLng:    "string",
	schema.TypeInterval:  "string",
	schema.TypeText:      "string",
	schema.TypeJSON:      "string",
	schema.TypeULID:      "string",
	schema.TypeUUID:      "string",
}

// Solidity returns the Solidity type for a neutral FieldType, appending the
// dynamic-array suffix for a repeated field (a JSON aggregate stays a single
// string, matching the JSONB single-document mapping).
func Solidity(t schema.FieldType, list bool) string {
	base := solForType[t]
	if base == "" {
		base = "string"
	}
	if list && t != schema.TypeJSON {
		return base + "[]"
	}
	return base
}

// SolidityForColumn returns the effective Solidity type of a column: an explicit
// web3.v1.field.solidity_type override (e.g. "address", "bytes32", "uint128") read
// off the field's Source descriptor, else the neutral FieldType projection —
// proto-classified, or set by protokit's synthesis and foreign-key alignment.
func SolidityForColumn(col *schema.Column) string {
	if t := fieldOptions(col.Source).GetSolidityType(); t != "" {
		return t
	}
	return Solidity(col.Type, col.List)
}

// SolidityIsReference reports whether a Solidity type is a reference type, i.e.
// one that requires an explicit data location (calldata/memory) in function
// parameters and return values. string, bytes, dynamic arrays, and structs are
// reference types; the fixed-size value types (bool, intN, uintN) are not.
func SolidityIsReference(solType string) bool {
	switch solType {
	case "string", "bytes":
		return true
	}
	// Dynamic arrays (T[]) are reference types; structs are handled by the caller
	// which knows the struct names.
	if len(solType) >= 2 && solType[len(solType)-2:] == "[]" {
		return true
	}
	return false
}
