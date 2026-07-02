// Package solana generates Anchor (Rust) programs from the protokit IR — one
// program per schema, each holding a PDA-backed account and create/update/close
// instructions per resource, mirroring the solidity target's per-resource storage
// contract but emitting on-chain Rust for Solana.
//
// Output layout — one Anchor workspace per database:
//
//	<db>/Anchor.toml                 workspace + program id manifest
//	<db>/Cargo.toml                  workspace member list
//	<db>/README.md                   ER diagram + per-model reference
//	<db>/programs/<schema>/Cargo.toml  program crate manifest
//	<db>/programs/<schema>/src/lib.rs  the #[program] with accounts + instructions
//
// Each resource becomes an #[account] struct plus create/update/close
// instructions keyed by a PDA seeded on the record key. Writes are gated by the
// access model (open, a program-owner authority, or per-record owner).
package solana

import (
	"fmt"

	"google.golang.org/protobuf/compiler/protogen"

	"github.com/the-protobuf-project/protokit/docs"
	"github.com/the-protobuf-project/protokit/naming"
	"github.com/the-protobuf-project/protokit/schema"
	"github.com/the-protobuf-project/protokit/templates"
	"github.com/the-protobuf-project/web3/plugin/generator/types"
)

// Generator implements schema.Target for Anchor (Rust) program output.
type Generator struct{}

// Name returns the target identifier used in buf.gen.yaml opt: [target=solana].
func (g *Generator) Name() string { return "solana" }

// Generate writes one Anchor program per schema plus a workspace scaffold per
// database.
func (g *Generator) Generate(p *protogen.Plugin, dbs []*schema.Database) error {
	for _, db := range dbs {
		for _, s := range db.Schemas {
			prog := naming.SnakeCase(s.Name)
			lib := p.NewGeneratedFile(fmt.Sprintf("%s/programs/%s/src/lib.rs", db.Name, prog), "")
			if err := templates.Render(tmpl, lib, "lib.rs.tpl", programView(db, s)); err != nil {
				return fmt.Errorf("solana: %s/programs/%s/src/lib.rs: %w", db.Name, prog, err)
			}
			cargo := p.NewGeneratedFile(fmt.Sprintf("%s/programs/%s/Cargo.toml", db.Name, prog), "")
			if err := templates.Render(tmpl, cargo, "program.cargo.toml.tpl", map[string]any{
				"Program": prog, "AnchorVersion": anchorVersion,
			}); err != nil {
				return fmt.Errorf("solana: %s/programs/%s/Cargo.toml: %w", db.Name, prog, err)
			}
		}
		if err := writeScaffold(p, db); err != nil {
			return err
		}
	}
	return nil
}

// writeScaffold emits the Anchor workspace files and README.
func writeScaffold(p *protogen.Plugin, db *schema.Database) error {
	at := p.NewGeneratedFile(db.Name+"/Anchor.toml", "")
	if err := templates.Render(tmpl, at, "anchor.toml.tpl", scaffoldView(db)); err != nil {
		return fmt.Errorf("solana: %s/Anchor.toml: %w", db.Name, err)
	}
	ct := p.NewGeneratedFile(db.Name+"/Cargo.toml", "")
	if err := templates.Render(tmpl, ct, "workspace.cargo.toml.tpl", scaffoldView(db)); err != nil {
		return fmt.Errorf("solana: %s/Cargo.toml: %w", db.Name, err)
	}
	rf := p.NewGeneratedFile(db.Name+"/README.md", "")
	md := docs.Render(db, docs.Meta{
		Title:   "Solana programs",
		Tagline: "An Anchor program per schema — a PDA-backed account and create/update/close instructions per resource.",
		Outputs: []string{
			"`programs/<schema>/src/lib.rs` — the `#[program]` module: an `#[account]` struct per resource plus `create`/`update`/`close` instructions keyed by a PDA seeded on the record key. Writes are gated by the `access` model (open, program authority, or per-record owner).",
			"`programs/<schema>/Cargo.toml` — the program crate; `Anchor.toml` + `Cargo.toml` at the root form the workspace. Run `anchor build` from the database directory.",
			"Dynamic fields (String, Vec) carry `#[max_len(..)]` so `#[derive(InitSpace)]` can size the rent-exempt account; adjust the caps to your data.",
			"`declare_id!` carries an all-ones placeholder — run `anchor keys sync` to bind real program ids before deploying.",
		},
		Naming: docs.Local(db),
		TypeOf: types.SolanaForColumn,
	})
	if _, err := rf.Write([]byte(md)); err != nil {
		return fmt.Errorf("solana: %s/README.md: %w", db.Name, err)
	}
	return nil
}
