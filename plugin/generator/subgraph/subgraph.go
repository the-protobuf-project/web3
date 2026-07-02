// Package subgraph generates a The Graph subgraph from the protokit IR — the
// off-chain read path for the solidity target. On-chain a CRUD contract can
// store records but cannot scale list/filter/secondary-index queries; a subgraph
// indexes the contracts' Created/Updated/Deleted events into a queryable
// GraphQL store, so the gRPC adapter's read methods hit the indexer, not the
// chain.
//
// Output layout — one buildable subgraph project per database:
//
//	<db>/subgraph.yaml          manifest: one dataSource per resource contract
//	<db>/schema.graphql         one @entity per resource (+ FK references)
//	<db>/abis/<Contract>.json   the events + get() each mapping binds to
//	<db>/src/<schemapkg>.ts      AssemblyScript event handlers
//	<db>/package.json           graph-cli scaffold (run graph codegen && graph build)
//
// Build it with the Graph CLI: `graph codegen && graph build` from the database
// directory (the AssemblyScript is verified there, the way `forge build` checks
// the solidity output).
package subgraph

import (
	"fmt"

	"google.golang.org/protobuf/compiler/protogen"

	"github.com/the-protobuf-project/protokit/docs"
	"github.com/the-protobuf-project/protokit/naming"
	"github.com/the-protobuf-project/protokit/schema"
	"github.com/the-protobuf-project/protokit/templates"
	"github.com/the-protobuf-project/web3/plugin/generator/types"
)

// Generator implements schema.Target for The Graph subgraph output.
type Generator struct{}

// Name returns the target identifier used in buf.gen.yaml opt: [target=subgraph].
func (g *Generator) Name() string { return "subgraph" }

// Generate writes one subgraph project per database.
func (g *Generator) Generate(p *protogen.Plugin, dbs []*schema.Database) error {
	for _, db := range dbs {
		sf := p.NewGeneratedFile(db.Name+"/schema.graphql", "")
		if err := templates.Render(tmpl, sf, "schema.graphql.tpl", schemaView(db)); err != nil {
			return fmt.Errorf("subgraph: %s/schema.graphql: %w", db.Name, err)
		}

		mf := p.NewGeneratedFile(db.Name+"/subgraph.yaml", "")
		if err := templates.Render(tmpl, mf, "subgraph.yaml.tpl", manifestView(db)); err != nil {
			return fmt.Errorf("subgraph: %s/subgraph.yaml: %w", db.Name, err)
		}

		for _, s := range db.Schemas {
			pkg := naming.GoPackage(s.Name)
			hf := p.NewGeneratedFile(fmt.Sprintf("%s/src/%s.ts", db.Name, pkg), "")
			if err := templates.Render(tmpl, hf, "mapping.ts.tpl", mappingView(db, s)); err != nil {
				return fmt.Errorf("subgraph: %s/src/%s.ts: %w", db.Name, pkg, err)
			}
			for _, t := range s.Tables {
				abiPath := fmt.Sprintf("%s/abis/%s.json", db.Name, contractName(t))
				af := p.NewGeneratedFile(abiPath, "")
				body, err := contractABIJSON(t)
				if err != nil {
					return fmt.Errorf("subgraph: %s: %w", abiPath, err)
				}
				if _, err := af.Write([]byte(body)); err != nil {
					return fmt.Errorf("subgraph: %s: %w", abiPath, err)
				}
			}
		}

		pf := p.NewGeneratedFile(db.Name+"/package.json", "")
		if err := templates.Render(tmpl, pf, "subgraph.package.json.tpl", map[string]any{"Database": db.Name}); err != nil {
			return fmt.Errorf("subgraph: %s/package.json: %w", db.Name, err)
		}

		rf := p.NewGeneratedFile(db.Name+"/README.md", "")
		md := docs.Render(db, docs.Meta{
			Title:   "The Graph subgraph",
			Tagline: "An off-chain index of the on-chain storage contracts — one @entity per resource.",
			Outputs: []string{
				"`schema.graphql` — one `@entity` per resource; foreign keys become references and the reverse side is a `@derivedFrom` list.",
				"`subgraph.yaml` — the manifest: one dataSource per resource contract, handling its `Created`/`Updated`/`Deleted` events. Fill in each contract `address` and `startBlock` after deployment.",
				"`abis/<Contract>.json` — the events and `get()` view each mapping binds to.",
				"`src/<schema>.ts` — AssemblyScript handlers that read a record via `get(key)` and upsert the entity; a Deleted event removes it.",
				"Build with the Graph CLI: `npm install` then `graph codegen && graph build` from this directory.",
			},
			Naming: docs.Local(db),
			TypeOf: types.SolidityForColumn,
		})
		if _, err := rf.Write([]byte(md)); err != nil {
			return fmt.Errorf("subgraph: %s/README.md: %w", db.Name, err)
		}
	}
	return nil
}
