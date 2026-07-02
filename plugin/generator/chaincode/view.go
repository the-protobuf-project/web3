package chaincode

// view.go prepares the Hyperledger Fabric (Go) chaincode template views: one
// asset struct + a contractapi.Contract with Create/Read/Update/Delete/List/
// Exists transactions per resource, plus the module scaffold. Templates are
// presentation only — Go naming, type projection, and key resolution live here.

import (
	"strings"

	"github.com/the-protobuf-project/protokit/header"
	"github.com/the-protobuf-project/protokit/naming"
	"github.com/the-protobuf-project/protokit/schema"
	"github.com/the-protobuf-project/web3/plugin/generator/types"
)

// fabricContractAPI pins the Fabric contract-api-go the scaffold targets.
const fabricContractAPI = "v1.2.2"

type fieldView struct {
	Comment, Name, JSONTag, Type string
}

type enumValueView struct{ Comment, Const, MapName string }

type enumView struct {
	Comment, Name string
	Values        []enumValueView
}

// assetView is one contract rendered as a Fabric asset struct plus its
// contractapi.Contract with world-state CRUD transactions.
type assetView struct {
	Comment, Struct, Contract string
	Fields                    []fieldView
	KeyName, KeyType          string // Go field name + type of the world-state key
	Access                    string // "" open, "ownable", "per_record_owner"
}

// goFieldName converts a snake_case column name to an exported Go field name.
func goFieldName(col string) string { return naming.PascalGo(col) }

// goType returns the Go type for a column: an enum column uses its generated Go
// enum type, everything else projects the neutral type.
func goType(col *schema.Column) string {
	if col.Enum != nil && len(col.Enum.Values) > 0 {
		return col.Enum.LocalName
	}
	return types.FabricGoForColumn(col)
}

// contractView assembles the contract.go data for one schema.
func contractView(db *schema.Database, s *schema.Schema) map[string]any {
	var assets []assetView
	for _, t := range s.Tables {
		assets = append(assets, assetViewOf(t))
	}
	return map[string]any{
		"Header": header.Render("//", header.Info{
			PluginVersion: db.PluginVersion,
			ProtocVersion: db.ProtocVersion,
			Source:        strings.Join(s.SourceProtos(), ", "),
			Database:      db.Name,
			Schema:        s.Name,
		}),
		"Package": naming.GoPackage(s.Name),
		"Enums":   enumViews(s),
		"Assets":  assets,
	}
}

// assetViewOf builds the per-contract asset + transactions view.
func assetViewOf(t *schema.Table) assetView {
	var fields []fieldView
	keyName, keyType := "", ""
	for _, col := range t.Columns {
		gt := goType(col)
		gn := goFieldName(col.Name)
		fields = append(fields, fieldView{
			Comment: commentOr(col.Comment, ""),
			Name:    gn,
			JSONTag: col.Name,
			Type:    gt,
		})
		if col.Name == t.PKColumn {
			keyName, keyType = gn, gt
		}
	}
	if keyName == "" && len(fields) > 0 {
		keyName, keyType = fields[0].Name, fields[0].Type
	}
	return assetView{
		Comment:  commentOr(t.Comment, t.LocalName+" asset."),
		Struct:   t.LocalName,
		Contract: t.LocalName + "Contract",
		Fields:   fields,
		KeyName:  keyName,
		KeyType:  keyType,
		Access:   t.Access,
	}
}

// enumViews renders each schema enum carrying values as a Go string type with one
// const per value, matching the gorm target's enum rendering.
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
				Const:   e.LocalName + naming.PascalGo(strings.ToLower(v.Name)),
				MapName: v.MapName,
			})
		}
		out = append(out, ev)
	}
	return out
}

// scaffoldView prepares the go.mod / main.go data for a database.
func scaffoldView(db *schema.Database) map[string]any {
	type contractRef struct{ Package, Type string }
	var refs []contractRef
	for _, s := range db.Schemas {
		pkg := naming.GoPackage(s.Name)
		for _, t := range s.Tables {
			refs = append(refs, contractRef{Package: pkg, Type: t.LocalName + "Contract"})
		}
	}
	// With go_module set, the database's packages live under that import path and
	// share the enclosing module; otherwise the database is its own standalone
	// module (its own go.mod), keyed by a generic base path.
	module := "github.com/the-protobuf-project/generated/" + db.Name
	standalone := true
	if gm := dbGoModule(db); gm != "" {
		module = gm + "/" + db.Name
		standalone = false
	}
	return map[string]any{
		"Database":          db.Name,
		"Module":            module,
		"Standalone":        standalone,
		"FabricContractAPI": fabricContractAPI,
		"Contracts":         refs,
		"Schemas":           schemaPkgs(db),
	}
}

func schemaPkgs(db *schema.Database) []string {
	var out []string
	for _, s := range db.Schemas {
		out = append(out, naming.GoPackage(s.Name))
	}
	return out
}

func commentOr(comment, fallback string) string {
	if comment != "" {
		return comment
	}
	return fallback
}
