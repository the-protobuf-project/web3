package types

// solana.go projects the neutral schema.FieldType onto an Anchor (Rust) type for
// the Solana target: account fields and instruction arguments. A repeated field
// becomes a Vec<T>; a JSON aggregate stays a single String (matching the EVM
// single-document mapping). An explicit web3.v1.field.solana_type override wins.

import "github.com/the-protobuf-project/protokit/schema"

// rustForType maps a neutral FieldType to an Anchor/Rust type. Amounts/counters
// widen to u64, decimals to u128, timestamps to i64 (Solana Clock unix time);
// floats fall back to their Rust equivalents but are rare on-chain. TypeEnum is
// handled by the caller via schema.Column.Enum.
var rustForType = map[schema.FieldType]string{
	schema.TypeString:    "String",
	schema.TypeBool:      "bool",
	schema.TypeInt32:     "i32",
	schema.TypeUint32:    "u32",
	schema.TypeInt64:     "i64",
	schema.TypeUint64:    "u64",
	schema.TypeFloat:     "f32",
	schema.TypeDouble:    "f64",
	schema.TypeBytes:     "Vec<u8>",
	schema.TypeTimestamp: "i64",
	schema.TypeDuration:  "i64",
	schema.TypeDate:      "i64",
	schema.TypeTimeOfDay: "i64",
	schema.TypeDecimal:   "u128",
	schema.TypeLatLng:    "String",
	schema.TypeInterval:  "String",
	schema.TypeText:      "String",
	schema.TypeJSON:      "String",
	schema.TypeULID:      "String",
	schema.TypeUUID:      "String",
}

// Solana returns the Anchor/Rust type for a neutral FieldType, wrapping a
// repeated field in Vec<..> (a JSON aggregate stays a single String).
func Solana(t schema.FieldType, list bool) string {
	base := rustForType[t]
	if base == "" {
		base = "String"
	}
	if list && t != schema.TypeJSON {
		return "Vec<" + base + ">"
	}
	return base
}

// SolanaForColumn returns the effective Anchor/Rust type of a column: an explicit
// web3.v1.field.solana_type override (e.g. "Pubkey", "[u8; 32]") read off the
// field's Source descriptor, else the neutral FieldType projection.
func SolanaForColumn(col *schema.Column) string {
	if t := fieldOptions(col.Source).GetSolanaType(); t != "" {
		return t
	}
	return Solana(col.Type, col.List)
}

// SolanaMaxLen returns the Anchor #[max_len(..)] a dynamic field needs so the
// account's rent-exempt space can be computed at init, or 0 for fixed-size types
// that need no annotation. Strings and byte vectors get a generous default cap;
// other Vec<T> fields cap their element count.
func SolanaMaxLen(col *schema.Column) int {
	rust := SolanaForColumn(col)
	switch {
	case rust == "String":
		return 64
	case rust == "Vec<u8>":
		return 128
	case len(rust) >= 4 && rust[:4] == "Vec<":
		return 16
	default:
		return 0
	}
}
