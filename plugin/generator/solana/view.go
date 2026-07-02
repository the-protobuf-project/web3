package solana

// view.go prepares the Anchor (Rust) template views: per-program account structs,
// the create/update/close instructions keyed by a record's PDA, the file-level
// enums, and the Cargo/Anchor scaffold. Templates are presentation only — naming,
// Rust-type projection, and key resolution live here.

import (
	"strings"

	"github.com/the-protobuf-project/protokit/header"
	"github.com/the-protobuf-project/protokit/naming"
	"github.com/the-protobuf-project/protokit/schema"
	"github.com/the-protobuf-project/web3/plugin/generator/types"
)

// anchorVersion pins the Anchor + Solana toolchain the scaffold targets.
const anchorVersion = "0.30.1"

// placeholderProgramID is a valid all-ones base58 program id the generated
// declare_id! carries until the author runs `anchor keys sync`.
const placeholderProgramID = "11111111111111111111111111111111"

type fieldView struct {
	Comment, Name, Type string
	MaxLen              int // Anchor #[max_len(..)] for a dynamic field; 0 omits it
}

type enumValueView struct{ Comment, Name string }

type enumView struct {
	Comment, Name string
	Values        []enumValueView
}

// accountView is one contract rendered as an Anchor #[account] plus its
// create/update/close instructions, keyed by a PDA derived from the record key.
type accountView struct {
	Comment, Struct, Snake string
	Fields                 []fieldView
	KeyName, KeyType       string
	KeySeed                string // Rust expr yielding the PDA seed bytes from `data`
	Access                 string // "" open, "ownable", "per_record_owner"
}

// keySeed returns the Rust expression that turns a record key into PDA seed bytes,
// picked by key type: a String hashes its bytes, a Pubkey its ref, an integer its
// little-endian bytes.
func keySeed(keyName, keyType string) string {
	switch keyType {
	case "String":
		return "data." + keyName + ".as_bytes()"
	case "Pubkey":
		return "data." + keyName + ".as_ref()"
	default:
		return "&data." + keyName + ".to_le_bytes()"
	}
}

// rustReserved holds Rust keywords a struct field name must not be; a colliding
// name is prefixed with the raw-identifier escape r#.
var rustReserved = map[string]bool{
	"as": true, "break": true, "const": true, "continue": true, "crate": true,
	"else": true, "enum": true, "extern": true, "false": true, "fn": true,
	"for": true, "if": true, "impl": true, "in": true, "let": true, "loop": true,
	"match": true, "mod": true, "move": true, "mut": true, "pub": true, "ref": true,
	"return": true, "self": true, "Self": true, "static": true, "struct": true,
	"super": true, "trait": true, "true": true, "type": true, "unsafe": true,
	"use": true, "where": true, "while": true, "async": true, "await": true,
	"dyn": true, "box": true, "final": true, "macro": true, "override": true,
	"priv": true, "typeof": true, "virtual": true, "yield": true,
}

// rustFieldName keeps the snake_case column name (already Rust convention),
// escaping a reserved word as a raw identifier (r#type).
func rustFieldName(col string) string {
	if rustReserved[col] {
		return "r#" + col
	}
	return col
}

// rustType returns the Anchor/Rust type for a column: an enum column uses its
// generated enum type, an enum with no values falls back to u8, everything else
// projects the neutral type (honoring a web3.v1.field.solana_type override).
func rustType(col *schema.Column) string {
	if col.Enum != nil {
		if len(col.Enum.Values) > 0 {
			return col.Enum.LocalName
		}
		return "u8"
	}
	return types.SolanaForColumn(col)
}

// programView assembles the lib.rs data for one schema: its program name, the
// account structs + instructions, and the enums the accounts reference.
func programView(db *schema.Database, s *schema.Schema) map[string]any {
	prog := naming.SnakeCase(s.Name)
	var accounts []accountView
	for _, t := range s.Tables {
		accounts = append(accounts, accountViewOf(t))
	}
	return map[string]any{
		"Header":    header.Render("//", header.Info{
			PluginVersion: db.PluginVersion,
			ProtocVersion: db.ProtocVersion,
			Source:        strings.Join(s.SourceProtos(), ", "),
			Database:      db.Name,
			Schema:        s.Name,
		}),
		"Program":   prog,
		"ProgramID": placeholderProgramID,
		"Enums":     enumViews(s),
		"Accounts":  accounts,
	}
}

// accountViewOf builds the per-contract account + instruction view.
func accountViewOf(t *schema.Table) accountView {
	var fields []fieldView
	keyName, keyType := "", ""
	for _, col := range t.Columns {
		rt := rustType(col)
		fn := rustFieldName(col.Name)
		// #[max_len] belongs only on a dynamic field (String/Vec); an enum column
		// renders as its fixed-size enum type, so it never carries one.
		maxLen := 0
		if col.Enum == nil {
			maxLen = types.SolanaMaxLen(col)
		}
		fields = append(fields, fieldView{
			Comment: commentOr(col.Comment, ""),
			Name:    fn,
			Type:    rt,
			MaxLen:  maxLen,
		})
		if col.Name == t.PKColumn {
			keyName, keyType = fn, rt
		}
	}
	if keyName == "" && len(fields) > 0 {
		keyName, keyType = fields[0].Name, fields[0].Type
	}
	return accountView{
		Comment: commentOr(t.Comment, t.LocalName+" account."),
		Struct:  t.LocalName,
		Snake:   naming.SnakeCase(t.LocalName),
		Fields:  fields,
		KeyName: keyName,
		KeyType: keyType,
		KeySeed: keySeed(keyName, keyType),
		Access:  t.Access,
	}
}

// enumViews renders each schema enum carrying at least one value as a Rust enum
// (Anchor derives AnchorSerialize/Deserialize on it). An empty enum is skipped
// and its columns fall back to u8.
func enumViews(s *schema.Schema) []enumView {
	var out []enumView
	for _, e := range s.Enums {
		if len(e.Values) == 0 {
			continue
		}
		ev := enumView{
			Comment: commentOr(e.Comment, e.LocalName+" values."),
			Name:    e.LocalName,
		}
		for _, v := range e.Values {
			ev.Values = append(ev.Values, enumValueView{
				Comment: v.Comment,
				Name:    naming.PascalGo(strings.ToLower(v.Name)),
			})
		}
		out = append(out, ev)
	}
	return out
}

// scaffoldView prepares the Anchor.toml / Cargo.toml data for a database.
func scaffoldView(db *schema.Database) map[string]any {
	var programs []string
	for _, s := range db.Schemas {
		programs = append(programs, naming.SnakeCase(s.Name))
	}
	return map[string]any{
		"Database":      db.Name,
		"Programs":      programs,
		"AnchorVersion": anchorVersion,
		"ProgramID":     placeholderProgramID,
	}
}

func commentOr(comment, fallback string) string {
	if comment != "" {
		return comment
	}
	return fallback
}
