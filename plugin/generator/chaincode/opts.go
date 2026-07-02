package chaincode

// opts.go reads the chaincode target's render-time knobs off the neutral IR. The
// web3 backend stamps them onto db.Opts during Enrich; protokit never interprets
// them and the chaincode target is the only consumer.

import "github.com/the-protobuf-project/protokit/schema"

// dbGoModule is the Go import path of the generated output directory (from the
// go_module opt). When set, the chaincode imports its per-schema packages by this
// base path and no standalone go.mod is emitted, so the tree compiles inside an
// enclosing module. Empty means each database is its own standalone module.
func dbGoModule(db *schema.Database) string { return db.Opt("go_module") }
