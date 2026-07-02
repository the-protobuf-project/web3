package solidity

// view.go prepares the template view models for the solidity backend: the
// per-contract struct fields, the key (primary-key) descriptor that drives the
// mapping and CRUD signatures, the file-level enum list, and the scaffold data.
// Templates are presentation only — all naming and type logic lives here.

import (
	"strings"

	"github.com/the-protobuf-project/protokit/header"
	"github.com/the-protobuf-project/protokit/naming"
	"github.com/the-protobuf-project/protokit/schema"
	"github.com/the-protobuf-project/web3/plugin/generator/types"
)

// soliditySolc pins the compiler pragma and the foundry.toml solc version so the
// generated tree builds reproducibly.
const soliditySolc = "0.8.24"

type fieldView struct{ Comment, Name, Type string }

type enumValueView struct {
	Comment, Name string
	Last          bool
}

type enumView struct {
	Comment, Name string
	Values        []enumValueView
}

// keyView describes a contract's primary key — the mapping key type and the
// struct field carrying it, plus the data location (" calldata" for reference
// types, "" for value types) that the CRUD signatures need. Indexed is true when
// the key is a value type safe to mark `indexed` in events; a dynamic key
// (string/bytes) is emitted non-indexed so off-chain consumers can recover it
// from the event data rather than only its keccak hash in the topic.
type keyView struct {
	Type, Field, Loc string
	Indexed          bool
}

// reserved is the set of Solidity keywords and built-in type names a generated
// struct field must not shadow; a colliding name gets a trailing underscore.
var reserved = map[string]bool{
	"address": true, "bool": true, "string": true, "bytes": true, "byte": true,
	"int": true, "uint": true, "fixed": true, "ufixed": true,
	"mapping": true, "struct": true, "enum": true, "contract": true,
	"interface": true, "library": true, "function": true, "modifier": true,
	"event": true, "emit": true, "return": true, "returns": true, "if": true,
	"else": true, "for": true, "while": true, "do": true, "break": true,
	"continue": true, "new": true, "delete": true, "this": true, "super": true,
	"memory": true, "storage": true, "calldata": true, "public": true,
	"private": true, "internal": true, "external": true, "view": true,
	"pure": true, "payable": true, "constant": true, "immutable": true,
	"using": true, "import": true, "pragma": true, "is": true, "type": true,
	"from": true, "as": true, "now": true, "wei": true, "gwei": true,
	"ether": true, "seconds": true, "minutes": true, "hours": true,
	"days": true, "weeks": true, "years": true, "true": true, "false": true,
	"assembly": true, "try": true, "catch": true, "revert": true,
	"require": true, "assert": true, "abstract": true, "virtual": true,
	"override": true, "constructor": true, "receive": true, "fallback": true,
	"unchecked": true,
}

// solidityFieldName converts a snake_case column name to a camelCase Solidity
// struct field, appending "_" when the result would shadow a reserved word.
func solidityFieldName(col string) string {
	name := naming.Camel(col)
	if reserved[name] {
		return name + "_"
	}
	return name
}

// solidityFieldType returns the Solidity type for a column: enums use their
// file-level enum type, columns whose enum carries no values (only the dropped
// *_UNSPECIFIED sentinel) fall back to uint8, everything else projects the
// canonical PostgreSQL type.
func solidityFieldType(col *schema.Column) string {
	if col.Enum != nil {
		if len(col.Enum.Values) > 0 {
			return col.Enum.LocalName
		}
		return "uint8"
	}
	return types.SolidityForColumn(col)
}

// contractView assembles the template data for one resource's storage contract.
func contractView(db *schema.Database, s *schema.Schema, t *schema.Table) map[string]any {
	var fields []fieldView
	var indexed []fieldView // fields flagged web3.v1.field.indexed, emitted as event topics
	needEnums := false
	var key keyView
	for _, col := range t.Columns {
		ft := solidityFieldType(col)
		if col.Enum != nil && len(col.Enum.Values) > 0 {
			needEnums = true
		}
		fn := solidityFieldName(col.Name)
		fields = append(fields, fieldView{
			Comment: commentOr(col.Comment, ""),
			Name:    fn,
			Type:    ft,
		})
		if col.Name == t.PKColumn {
			key = keyView{Type: ft, Field: fn, Loc: loc(ft), Indexed: !types.SolidityIsReference(ft)}
		} else if types.Indexed(col) {
			indexed = append(indexed, fieldView{Name: fn, Type: ft})
		}
	}
	// Fall back to the first column when no PK was resolved, so the contract still
	// has a coherent mapping key rather than an empty one.
	if key.Field == "" && len(fields) > 0 {
		ft := fields[0].Type
		key = keyView{Type: ft, Field: fields[0].Name, Loc: loc(ft), Indexed: !types.SolidityIsReference(ft)}
	}
	// A Solidity event carries at most 3 indexed topics; the key takes one when it
	// is itself indexed, so cap the additional indexed fields at the remainder.
	maxIndexed := 3
	if key.Indexed {
		maxIndexed--
	}
	if len(indexed) > maxIndexed {
		indexed = indexed[:maxIndexed]
	}

	return map[string]any{
		"Header": header.Render("//", header.Info{
			PluginVersion: db.PluginVersion,
			ProtocVersion: db.ProtocVersion,
			Source:        strings.Join(s.SourceProtos(), ", "),
			Database:      db.Name,
			Schema:        s.Name,
		}),
		"Solc":      soliditySolc,
		"Contract":  t.LocalName + "Store",
		"Struct":    t.LocalName,
		"Comment":   commentOr(t.Comment, t.LocalName+" storage contract."),
		"NeedEnums":     needEnums,
		"Fields":        fields,
		"IndexedFields": indexed,
		"Key":           key,
		"Access":        t.Access,
		"SchemaVersion": t.SchemaVersion,
	}
}

// loc returns the data-location keyword a Solidity parameter of type t needs:
// " calldata" for reference types, "" for value types.
func loc(t string) string {
	if types.SolidityIsReference(t) {
		return " calldata"
	}
	return ""
}

// typesView assembles the per-schema enums file template data.
func typesView(db *schema.Database, s *schema.Schema, enums []enumView) map[string]any {
	return map[string]any{
		"Header": header.Render("//", header.Info{
			PluginVersion: db.PluginVersion,
			ProtocVersion: db.ProtocVersion,
			Source:        strings.Join(s.SourceProtos(), ", "),
			Database:      db.Name,
			Schema:        s.Name,
		}),
		"Solc":  soliditySolc,
		"Enums": enums,
	}
}

// enumViews renders each schema enum carrying at least one value as a Solidity
// enum. An enum with no values (one declaring only the dropped *_UNSPECIFIED
// sentinel) is skipped — Solidity has no empty enum — and its columns fall back
// to uint8 in solidityFieldType.
func enumViews(s *schema.Schema) []enumView {
	var out []enumView
	for _, e := range s.Enums {
		if len(e.Values) == 0 {
			continue
		}
		ev := enumView{
			Comment: commentOr(e.Comment, e.LocalName+" enumerates the "+e.LocalSQLName+" values."),
			Name:    e.LocalName,
		}
		for _, v := range e.Values {
			ev.Values = append(ev.Values, enumValueView{
				Comment: v.Comment,
				Name:    naming.SanitizeIdent(v.Name),
			})
		}
		if n := len(ev.Values); n > 0 {
			ev.Values[n-1].Last = true
		}
		out = append(out, ev)
	}
	return out
}

// scaffoldView prepares the Foundry project scaffold template data.
func scaffoldView(db *schema.Database) map[string]any {
	return map[string]any{
		"Database": db.Name,
		"Solc":     soliditySolc,
	}
}

// commentOr returns comment when non-empty, otherwise the fallback.
func commentOr(comment, fallback string) string {
	if comment != "" {
		return comment
	}
	return fallback
}
