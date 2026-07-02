// Package chaincode generates Hyperledger Fabric (Go) chaincode from the protokit
// IR — one contractapi.Contract per resource with world-state CRUD transactions,
// mirroring the solidity target's per-resource storage contract but emitting Go
// chaincode for a Fabric network.
//
// Output layout — one Go chaincode module per database:
//
//	<db>/go.mod                        chaincode module + fabric-contract-api dep
//	<db>/main.go                       entrypoint registering every contract
//	<db>/README.md                     ER diagram + per-model reference
//	<db>/<schemapkg>/contract.go       asset structs + contractapi.Contract per resource
//
// Each resource becomes an asset struct (JSON-serialized into the world state)
// plus Create/Read/Update/Delete/List/Exists transactions keyed by the record
// key. Writes are gated by the access model (open, an owner client identity, or
// per-record owner).
package chaincode

import (
	"fmt"

	"google.golang.org/protobuf/compiler/protogen"

	"github.com/the-protobuf-project/protokit/docs"
	"github.com/the-protobuf-project/protokit/naming"
	"github.com/the-protobuf-project/protokit/schema"
	"github.com/the-protobuf-project/protokit/templates"
	"github.com/the-protobuf-project/web3/plugin/generator/types"
)

// Generator implements schema.Target for Hyperledger Fabric Go chaincode.
type Generator struct{}

// Name returns the target identifier used in buf.gen.yaml opt: [target=chaincode].
func (g *Generator) Name() string { return "chaincode" }

// Generate writes one contract.go per schema plus a chaincode module scaffold per
// database.
func (g *Generator) Generate(p *protogen.Plugin, dbs []*schema.Database) error {
	for _, db := range dbs {
		for _, s := range db.Schemas {
			pkg := naming.GoPackage(s.Name)
			cf := p.NewGeneratedFile(fmt.Sprintf("%s/%s/contract.go", db.Name, pkg), "")
			if err := templates.Render(tmpl, cf, "contract.go.tpl", contractView(db, s)); err != nil {
				return fmt.Errorf("chaincode: %s/%s/contract.go: %w", db.Name, pkg, err)
			}
		}
		if err := writeScaffold(p, db); err != nil {
			return err
		}
	}
	return nil
}

// writeScaffold emits the chaincode module files and README.
func writeScaffold(p *protogen.Plugin, db *schema.Database) error {
	mf := p.NewGeneratedFile(db.Name+"/main.go", "")
	if err := templates.Render(tmpl, mf, "main.go.tpl", scaffoldView(db)); err != nil {
		return fmt.Errorf("chaincode: %s/main.go: %w", db.Name, err)
	}
	// A standalone chaincode is its own Go module; when go_module places the tree
	// inside an enclosing module, that module owns the go.mod instead.
	if dbGoModule(db) == "" {
		gm := p.NewGeneratedFile(db.Name+"/go.mod", "")
		if err := templates.Render(tmpl, gm, "chaincode.go.mod.tpl", scaffoldView(db)); err != nil {
			return fmt.Errorf("chaincode: %s/go.mod: %w", db.Name, err)
		}
	}
	rf := p.NewGeneratedFile(db.Name+"/README.md", "")
	md := docs.Render(db, docs.Meta{
		Title:   "Fabric chaincode",
		Tagline: "A contractapi.Contract per resource with world-state CRUD — one Go chaincode module per database.",
		Outputs: []string{
			"`<schema>/contract.go` — an asset struct per resource plus a `contractapi.Contract` with `Create`/`Read`/`Update`/`Delete`/`List`/`Exists` transactions keyed by the record key. Writes are gated by the `access` model (open, an owner client identity, or per-record owner).",
			"`main.go` — the chaincode entrypoint that registers every generated contract; `go.mod` pins the fabric-contract-api-go dependency. Run `go build` from the database directory.",
			"Assets are JSON-serialized into the Fabric world state under a `<Asset>:<key>` composite key. Foreign keys are stored as plain key fields (no cross-asset constraint is enforced on-chain).",
		},
		Naming: docs.Local(db),
		TypeOf: types.FabricGoForColumn,
	})
	if _, err := rf.Write([]byte(md)); err != nil {
		return fmt.Errorf("chaincode: %s/README.md: %w", db.Name, err)
	}
	return nil
}
