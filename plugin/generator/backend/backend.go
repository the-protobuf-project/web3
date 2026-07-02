// Package backend is web3's schema.Backend: the bridge between protokit's
// generic IR builder and web3's own annotation package. protokit reads the
// generic google.api.* structure itself and calls this backend for the rest —
// the three Read* methods map web3.v1 options onto protokit's neutral structure
// during the build, and Enrich folds web3's on-chain concerns (access control and
// the storage-layout fingerprint) into the IR afterward. This is what lets a user
// annotate entirely with web3.v1 while protokit imports no web3 proto.
package backend

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"

	"github.com/the-protobuf-project/protokit/schema"
	"github.com/the-protobuf-project/web3/plugin/generator/types"
	"github.com/the-protobuf-project/web3/plugin/pb/web3v1"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protoreflect"
)

// Backend implements schema.Backend for web3's web3.v1 annotations. It also owns
// web3's grouping config (web3.yaml) — protokit holds none; the Backend resolves
// grouping here and hands protokit a fully-resolved Datasource per file.
type Backend struct {
	cfg      *Config // web3.yaml layout config; nil when none was supplied
	goModule string  // Go import path of the output dir; the chaincode target reads it
}

// New builds a web3 Backend from the resolved plugin options. goModule is the Go
// import path of the output directory (from the go_module opt); when set, the
// Fabric chaincode target imports its generated packages by that base path and
// omits a standalone go.mod, so the tree compiles inside an enclosing module.
// The zero value (Backend{}) is still valid — no config, package-path defaults.
func New(cfg *Config, goModule string) Backend {
	return Backend{cfg: cfg, goModule: goModule}
}

// ReadDatasource resolves the file's grouping and target chain from
// web3.v1.chain and web3.yaml, returning a fully-resolved protokit Datasource.
// The chain enum is carried in Provider ("evm"/"solana"/"fabric") so each target
// can read it off db.Provider. Precedence for namespace/group: annotation > config
// > protokit's package-path default. An explicit chain.group annotation is
// authoritative and never version-stripped; a config-derived (or
// resource-type-derived) group obeys strip_version via SchemaStrip.
func (b Backend) ReadDatasource(d protoreflect.FileDescriptor) schema.Datasource {
	o := chainOpts(d)
	cfgDB, cfgSchema, stripVer := b.cfg.resolve(string(d.Package()))

	namespace := o.GetNamespace()
	if namespace == "" {
		namespace = cfgDB
	}

	group := o.GetGroup()
	strip := false
	if group == "" {
		group = cfgSchema
		strip = stripVer
	}

	return schema.Datasource{
		Database:    namespace,
		Schema:      group,
		Provider:    chainProvider(o.GetChain()),
		SchemaStrip: strip,
	}
}

// DedupeSchemaTable reports web3.yaml's dedupe_schema_table policy (false when no
// config was supplied).
func (b Backend) DedupeSchemaTable() bool {
	return b.cfg != nil && b.cfg.DedupeSchemaTable
}

// ReadTable maps web3.v1.contract's structural fields onto protokit's neutral
// TableStructure. The rendering field (access) is applied later in Enrich.
func (Backend) ReadTable(d protoreflect.MessageDescriptor) schema.TableStructure {
	o := contractOpts(d)
	return schema.TableStructure{
		Table:      o.GetAlias(),
		Skip:       o.GetSkip(),
		ID:         idStrategy(o.GetId()),
		Timestamps: o.GetTimestamps(),
	}
}

// ReadColumn maps web3.v1.field's structural fields onto protokit's neutral
// ColumnStructure. Per-chain type overrides (solidity_type/solana_type) are not
// stored on the neutral IR — they are read at render time off the field's Source
// descriptor by the types package.
func (Backend) ReadColumn(d protoreflect.FieldDescriptor) schema.ColumnStructure {
	o := fieldOpts(d)
	return schema.ColumnStructure{
		Column: o.GetAlias(),
		Skip:   o.GetSkip(),
	}
}

// Enrich sets each contract's access model (from web3.v1.contract.access) and its
// storage-layout fingerprint (SCHEMA_VERSION). A synthesized contract (nil Source)
// or one without the annotation is open.
func (b Backend) Enrich(dbs []*schema.Database) error {
	for _, db := range dbs {
		for _, s := range db.Schemas {
			for _, t := range s.Tables {
				t.Access = accessModel(t.Source)
				t.SchemaVersion = schemaFingerprint(t)
			}
		}
		// Stamp the chaincode target's render-time knob onto the neutral IR;
		// protokit never interprets it, the target reads it back off db.Opts.
		if b.goModule != "" {
			if db.Opts == nil {
				db.Opts = map[string]string{}
			}
			db.Opts["go_module"] = b.goModule
		}
	}
	return nil
}

// accessModel maps web3.v1.contract.access to the short string the generators
// consume ("" open, "ownable", "per_record_owner").
func accessModel(d protoreflect.MessageDescriptor) string {
	switch contractOpts(d).GetAccess() {
	case web3v1.AccessControl_ACCESS_CONTROL_OWNABLE:
		return "ownable"
	case web3v1.AccessControl_ACCESS_CONTROL_PER_RECORD_OWNER:
		return "per_record_owner"
	default:
		return ""
	}
}

// schemaFingerprint hashes a contract's ordered fields (name + on-chain type) into
// a 0x-prefixed 32-byte hex string. Any change to the on-chain storage layout —
// an added/removed/retyped field — changes the fingerprint, which is how the evm
// driver detects a deployed contract that no longer matches the client. The type
// is each field's Solidity projection, so the fingerprint tracks exactly the
// struct the solidity target emits.
func schemaFingerprint(t *schema.Table) string {
	h := sha256.New()
	for _, c := range t.Columns {
		typ := types.SolidityForColumn(c)
		if c.Enum != nil {
			typ = "enum:" + c.Enum.LocalName
		}
		fmt.Fprintf(h, "%s:%s;", c.Name, typ)
	}
	return "0x" + hex.EncodeToString(h.Sum(nil))
}

// chainProvider maps web3.v1.Chain onto the provider token carried in
// schema.Database.Provider, which the targets read to pick their chain.
func chainProvider(c web3v1.Chain) string {
	switch c {
	case web3v1.Chain_CHAIN_SOLANA:
		return "solana"
	case web3v1.Chain_CHAIN_FABRIC:
		return "fabric"
	default:
		return "evm"
	}
}

// idStrategy maps web3.v1.IdStrategy onto protokit's neutral schema.IDStrategy.
func idStrategy(s web3v1.IdStrategy) schema.IDStrategy {
	switch s {
	case web3v1.IdStrategy_ID_STRATEGY_ULID:
		return schema.IDULID
	case web3v1.IdStrategy_ID_STRATEGY_UUID:
		return schema.IDUUID
	default:
		return schema.IDUnspecified
	}
}

// --- option accessors (safe empty value when the extension or descriptor is absent) ---

func chainOpts(d protoreflect.FileDescriptor) *web3v1.ChainOptions {
	if d == nil || !proto.HasExtension(d.Options(), web3v1.E_Chain) {
		return &web3v1.ChainOptions{}
	}
	return proto.GetExtension(d.Options(), web3v1.E_Chain).(*web3v1.ChainOptions)
}

func contractOpts(d protoreflect.MessageDescriptor) *web3v1.ContractOptions {
	if d == nil || !proto.HasExtension(d.Options(), web3v1.E_Contract) {
		return &web3v1.ContractOptions{}
	}
	return proto.GetExtension(d.Options(), web3v1.E_Contract).(*web3v1.ContractOptions)
}

func fieldOpts(d protoreflect.FieldDescriptor) *web3v1.FieldOptions {
	if d == nil || !proto.HasExtension(d.Options(), web3v1.E_Field) {
		return &web3v1.FieldOptions{}
	}
	return proto.GetExtension(d.Options(), web3v1.E_Field).(*web3v1.FieldOptions)
}
