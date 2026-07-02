// Package solidity generates Solidity smart contracts from the protokit IR — one
// self-contained CRUD storage contract per resource, mirroring the gorm target's
// per-resource store but emitting on-chain Solidity instead of Go.
//
// Output layout — one directory per schema (mirroring the gorm tree), plus a
// Foundry project scaffold at the database root so the tree compiles as-is:
//
//	<db>/foundry.toml          Foundry project config (compiles the whole tree)
//	<db>/.gitignore            ignores Foundry build artifacts
//	<db>/README.md             ER diagram + per-model reference
//	<db>/<schemapkg>/types.sol file-level enums shared by the schema's contracts
//	<db>/<schemapkg>/<Model>.sol  one storage contract per resource
//
// Each contract holds a struct mirroring the table columns, a key→record
// mapping, an enumerable key list, and create/get/update/remove/list/count/
// exists functions with Created/Updated/Deleted events. Foreign keys are stored
// as plain key fields (no cross-contract constraint is enforced on-chain).
package solidity

import (
	"fmt"

	"google.golang.org/protobuf/compiler/protogen"

	"github.com/the-protobuf-project/protokit/docs"
	"github.com/the-protobuf-project/protokit/naming"
	"github.com/the-protobuf-project/protokit/schema"
	"github.com/the-protobuf-project/protokit/templates"
	"github.com/the-protobuf-project/web3/plugin/generator/types"
)

// Generator implements schema.Target for Solidity smart-contract output.
type Generator struct{}

// Name returns the target identifier used in buf.gen.yaml opt: [target=solidity].
func (g *Generator) Name() string { return "solidity" }

// Generate writes one storage contract per resource (plus a per-schema enums
// file when the schema declares enums) and a Foundry project scaffold per
// database.
func (g *Generator) Generate(p *protogen.Plugin, dbs []*schema.Database) error {
	for _, db := range dbs {
		for _, s := range db.Schemas {
			pkg := naming.GoPackage(s.Name)
			// A schema's enums are emitted once as file-level Solidity enums that the
			// schema's contracts import; skip the file when there are none.
			if ev := enumViews(s); len(ev) > 0 {
				ef := p.NewGeneratedFile(fmt.Sprintf("%s/%s/types.sol", db.Name, pkg), "")
				if err := templates.Render(tmpl, ef, "types.sol.tpl", typesView(db, s, ev)); err != nil {
					return fmt.Errorf("solidity: %s/%s/types.sol: %w", db.Name, pkg, err)
				}
			}
			for _, t := range s.Tables {
				path := fmt.Sprintf("%s/%s/%s.sol", db.Name, pkg, t.LocalName)
				cf := p.NewGeneratedFile(path, "")
				if err := templates.Render(tmpl, cf, "contract.sol.tpl", contractView(db, s, t)); err != nil {
					return fmt.Errorf("solidity: %s: %w", path, err)
				}
			}
		}
		if err := writeScaffold(p, db); err != nil {
			return err
		}
	}
	return nil
}

// writeScaffold emits the Foundry project files and the README that turn the
// database folder into a buildable Solidity project.
func writeScaffold(p *protogen.Plugin, db *schema.Database) error {
	ff := p.NewGeneratedFile(db.Name+"/foundry.toml", "")
	if err := templates.Render(tmpl, ff, "foundry.toml.tpl", scaffoldView(db)); err != nil {
		return fmt.Errorf("solidity: %s/foundry.toml: %w", db.Name, err)
	}
	gf := p.NewGeneratedFile(db.Name+"/.gitignore", "")
	if err := templates.Render(tmpl, gf, "solidity.gitignore.tpl", nil); err != nil {
		return fmt.Errorf("solidity: %s/.gitignore: %w", db.Name, err)
	}
	rf := p.NewGeneratedFile(db.Name+"/README.md", "")
	md := docs.Render(db, docs.Meta{
		Title:   "Solidity contracts",
		Tagline: "A self-contained CRUD storage contract per resource — one Solidity file per model.",
		Outputs: []string{
			"`<schema>/<Model>.sol` — one storage contract per resource: a struct mirroring the columns, a key→record mapping, and `create`/`get`/`update`/`remove`/`list`/`count`/`exists` functions with `Created`/`Updated`/`Deleted` events. Writes are gated by the `access` model (open, `Ownable`, or per-record owner).",
			"`<schema>/types.sol` — file-level Solidity enums shared by the schema's contracts; emitted when the schema declares enums.",
			"`foundry.toml` — Foundry project config; run `forge build` from this directory to compile the whole tree.",
			"Solidity has no NULL and no float: nullable columns use the type's zero value, and REAL/DOUBLE/UUID-style columns are stored as `string`. Foreign keys are stored as plain key fields (no cross-contract constraint is enforced on-chain).",
			"`SCHEMA_VERSION` — each contract exposes a `bytes32` fingerprint of its storage layout. The generated runtime client compares it to the deployed contract and refuses a mismatch, so an out-of-date contract is caught rather than silently mis-decoded.",
			"Schema evolution: these contracts are immutable, so regenerating after a `.proto` change means deploying a new contract (its `SCHEMA_VERSION` changes) and migrating data — replay the old contract's `Created`/`Updated` events into the new one, or backfill from the subgraph. For in-place upgrades keep the address stable behind an ERC-1967/UUPS proxy and only ever *append* struct fields (never reorder or retype), preserving the storage layout the proxy delegates into.",
		},
		Naming: docs.Local(db),
		TypeOf: types.SolidityForColumn,
	})
	if _, err := rf.Write([]byte(md)); err != nil {
		return fmt.Errorf("solidity: %s/README.md: %w", db.Name, err)
	}
	return nil
}
