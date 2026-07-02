package subgraph

// view.go prepares the template view models for the subgraph backend: the
// GraphQL entities (with forward FK references and @derivedFrom reverse
// relations), the manifest dataSources (one per resource contract), and the
// AssemblyScript mapping handlers that materialize entities from the contracts'
// lifecycle events. Naming and type logic live here; templates are presentation.

import (
	"strings"

	"github.com/the-protobuf-project/protokit/header"
	"github.com/the-protobuf-project/protokit/naming"
	"github.com/the-protobuf-project/protokit/schema"
	"github.com/the-protobuf-project/web3/plugin/generator/types"
)

// graphApiVersion / graphSpecVersion pin the subgraph manifest versions.
const (
	graphSpecVersion = "1.0.0"
	graphAPIVersion  = "0.0.9"
)

// contractName mirrors the solidity target's contract name for a resource.
func contractName(t *schema.Table) string { return t.LocalName + "Store" }

// gqlField is one GraphQL entity field.
type gqlField struct{ Name, Type, Comment string }

// derivedField is a reverse relation rendered with @derivedFrom.
type derivedField struct{ Name, Type, Via string }

// entityView is one @entity type.
type entityView struct {
	Name, Comment string
	Fields        []gqlField
	Derived       []derivedField
}

// entityViews builds the GraphQL entities for a schema's tables.
func entityViews(s *schema.Schema) []entityView {
	var out []entityView
	for _, t := range s.Tables {
		e := entityView{
			Name:    t.LocalName,
			Comment: commentOr(t.Comment, t.LocalName+" — indexed from on-chain events."),
		}
		// The entity id is the on-chain record key.
		e.Fields = append(e.Fields, gqlField{Name: "id", Type: "ID!", Comment: "On-chain record key."})
		for _, col := range t.Columns {
			if col.Name == t.PKColumn {
				continue // the PK is the entity id
			}
			if col.FKModel != "" {
				// A foreign key becomes a forward reference to the related entity; the
				// referenced id string is stored in the field.
				e.Fields = append(e.Fields, gqlField{
					Name:    refFieldName(col.Name),
					Type:    col.FKModel + nn(col.NotNull),
					Comment: col.Comment,
				})
				continue
			}
			e.Fields = append(e.Fields, gqlField{
				Name:    naming.Camel(col.Name),
				Type:    graphqlType(col) + nn(col.NotNull),
				Comment: col.Comment,
			})
		}
		for _, hm := range t.HasMany {
			e.Derived = append(e.Derived, derivedField{
				Name: naming.Camel(hm.Field),
				Type: "[" + hm.Model + "!]!",
				Via:  refFieldName(hm.ViaFK),
			})
		}
		out = append(out, e)
	}
	return out
}

// graphqlType maps a column to its GraphQL scalar (without the nullability mark).
// Enums project to Int (the on-chain ordinal); integers split into Int (≤32-bit
// signed) and BigInt (everything wider, matching graph-ts).
func graphqlType(col *schema.Column) string {
	if col.Enum != nil {
		return "Int"
	}
	sol := types.SolidityForColumn(col)
	isArray := strings.HasSuffix(sol, "[]")
	base := strings.TrimSuffix(sol, "[]")
	var g string
	switch base {
	case "bool":
		g = "Boolean"
	case "string":
		g = "String"
	case "bytes":
		g = "Bytes"
	case "int16", "int32":
		g = "Int"
	default:
		if strings.HasPrefix(base, "int") || strings.HasPrefix(base, "uint") {
			g = "BigInt"
		} else {
			g = "String"
		}
	}
	if isArray {
		return "[" + g + "!]"
	}
	return g
}

// nn returns the GraphQL non-null mark for a NOT NULL column.
func nn(notNull bool) string {
	if notNull {
		return "!"
	}
	return ""
}

// refFieldName turns a foreign-key column name into the relation field name by
// dropping a trailing _id and camelCasing ("author_id" → "author").
func refFieldName(col string) string {
	return naming.Camel(strings.TrimSuffix(col, "_id"))
}

// --- manifest ---------------------------------------------------------------

// dataSourceView is one manifest dataSource (one resource contract).
type dataSourceView struct {
	Name       string // entity / dataSource name
	Contract   string // ABI name
	KeySig     string // event arg ABI type, e.g. "string"
	MappingPkg string // schema package owning the handler file
}

// manifestView assembles the subgraph.yaml data for a database.
func manifestView(db *schema.Database) map[string]any {
	var sources []dataSourceView
	for _, s := range db.Schemas {
		for _, t := range s.Tables {
			sources = append(sources, dataSourceView{
				Name:       t.LocalName,
				Contract:   contractName(t),
				KeySig:     keyABIType(t),
				MappingPkg: naming.GoPackage(s.Name),
			})
		}
	}
	return map[string]any{
		"SpecVersion": graphSpecVersion,
		"APIVersion":  graphAPIVersion,
		"Database":    db.Name,
		"Sources":     sources,
	}
}

// --- mapping ----------------------------------------------------------------

// assignView is one entity-field ← record-field assignment in a handler.
type assignView struct{ EntityField, RecordField string }

// handlerView is the mapping for one resource's three lifecycle events.
type handlerView struct {
	Entity   string
	Contract string
	Assigns  []assignView
}

// mappingView assembles one schema's AssemblyScript handler file.
func mappingView(db *schema.Database, s *schema.Schema) map[string]any {
	var handlers []handlerView
	var entityNames []string
	for _, t := range s.Tables {
		entityNames = append(entityNames, t.LocalName)
		h := handlerView{Entity: t.LocalName, Contract: contractName(t)}
		for _, col := range t.Columns {
			if col.Name == t.PKColumn {
				continue
			}
			field := naming.Camel(col.Name)
			if col.FKModel != "" {
				field = refFieldName(col.Name)
			}
			h.Assigns = append(h.Assigns, assignView{EntityField: field, RecordField: naming.Camel(col.Name)})
		}
		handlers = append(handlers, h)
	}
	return map[string]any{
		"Header": header.Render("//", header.Info{
			PluginVersion: db.PluginVersion,
			ProtocVersion: db.ProtocVersion,
			Source:        strings.Join(s.SourceProtos(), ", "),
			Database:      db.Name,
			Schema:        s.Name,
		}),
		"Entities": strings.Join(entityNames, ", "),
		"Handlers": handlers,
	}
}

// --- schema.graphql ---------------------------------------------------------

// schemaView assembles the schema.graphql data for a database.
func schemaView(db *schema.Database) map[string]any {
	var entities []entityView
	for _, s := range db.Schemas {
		entities = append(entities, entityViews(s)...)
	}
	return map[string]any{
		"Database": db.Name,
		"Entities": entities,
	}
}

// commentOr returns comment when non-empty, otherwise the fallback.
func commentOr(comment, fallback string) string {
	if comment != "" {
		return comment
	}
	return fallback
}
