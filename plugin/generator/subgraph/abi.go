package subgraph

// abi.go emits the minimal contract ABI a subgraph needs: the three lifecycle
// events it indexes and the get(key) view its mapping calls to read a record's
// fields. The ABI mirrors the solidity target's generated contract, so
// `graph codegen` produces a binding whose getters match the struct fields.

import (
	"encoding/json"

	"github.com/the-protobuf-project/protokit/naming"
	"github.com/the-protobuf-project/protokit/schema"
	"github.com/the-protobuf-project/web3/plugin/generator/types"
)

// abiParam is one ABI input/output (or tuple component).
type abiParam struct {
	Name       string     `json:"name"`
	Type       string     `json:"type"`
	Indexed    *bool      `json:"indexed,omitempty"`
	Components []abiParam `json:"components,omitempty"`
}

// abiEntry is one ABI element (an event or a function).
type abiEntry struct {
	Type            string     `json:"type"`
	Name            string     `json:"name"`
	Inputs          []abiParam `json:"inputs"`
	Outputs         []abiParam `json:"outputs,omitempty"`
	StateMutability string     `json:"stateMutability,omitempty"`
	Anonymous       *bool      `json:"anonymous,omitempty"`
}

// abiComponentType returns the ABI type for a column: an enum is encoded as the
// uint8 ordinal the contract stores; everything else uses the solidity type
// (which is already a valid elementary ABI type).
func abiComponentType(c *schema.Column) string {
	if c.Enum != nil {
		return "uint8"
	}
	return types.SolidityForColumn(c)
}

// keyABIType returns the ABI type of the table's primary-key column (the event
// and get() key), defaulting to string when unresolved.
func keyABIType(t *schema.Table) string {
	for _, c := range t.Columns {
		if c.Name == t.PKColumn {
			return abiComponentType(c)
		}
	}
	return "string"
}

// contractABIJSON renders the complete ABI JSON for a table's storage contract:
// the CRUD functions and the three lifecycle events. The subgraph only binds
// get() and the events, but emitting the full surface lets the runtime evm
// driver call create/update/remove from the same generated artifact.
func contractABIJSON(t *schema.Table) (string, error) {
	record := func(name string) abiParam {
		components := make([]abiParam, 0, len(t.Columns))
		for _, c := range t.Columns {
			components = append(components, abiParam{Name: naming.Camel(c.Name), Type: abiComponentType(c)})
		}
		return abiParam{Name: name, Type: "tuple", Components: components}
	}
	recordArray := func(name string) abiParam {
		p := record(name)
		p.Type = "tuple[]"
		return p
	}
	key := keyABIType(t)
	no := false
	event := func(suffix string) abiEntry {
		return abiEntry{
			Type:      "event",
			Name:      t.LocalName + suffix,
			Inputs:    []abiParam{{Name: "key", Type: key, Indexed: &no}},
			Anonymous: &no,
		}
	}
	fn := func(name, mut string, inputs, outputs []abiParam) abiEntry {
		return abiEntry{Type: "function", Name: name, StateMutability: mut, Inputs: inputs, Outputs: outputs}
	}
	keyParam := []abiParam{{Name: "key", Type: key}}
	entries := []abiEntry{
		fn("SCHEMA_VERSION", "pure", nil, []abiParam{{Name: "", Type: "bytes32"}}),
		fn("create", "nonpayable", []abiParam{record("record")}, nil),
		fn("get", "view", keyParam, []abiParam{record("")}),
		fn("update", "nonpayable", []abiParam{record("record")}, nil),
		fn("remove", "nonpayable", keyParam, nil),
		fn("exists", "view", keyParam, []abiParam{{Name: "", Type: "bool"}}),
		fn("count", "view", nil, []abiParam{{Name: "", Type: "uint256"}}),
		fn("list", "view", nil, []abiParam{recordArray("items")}),
		event("Created"),
		event("Updated"),
		event("Deleted"),
	}
	b, err := json.MarshalIndent(entries, "", "  ")
	if err != nil {
		return "", err
	}
	return string(b) + "\n", nil
}
