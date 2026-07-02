// SPDX-License-Identifier: Apache-2.0
{{.Header}}
pragma solidity ^{{.Solc}};
{{if .NeedEnums}}
import "./types.sol";
{{end}}
/// @notice {{.Comment}}
contract {{.Contract}} {
	/// @notice Fingerprint of the generated storage layout. A client refuses a
	/// deployed contract whose SCHEMA_VERSION differs from the one it was
	/// generated against, catching layout drift on an immutable contract.
	bytes32 public constant SCHEMA_VERSION = {{.SchemaVersion}};

	struct {{.Struct}} {
{{- range .Fields}}
{{- if .Comment}}
		// {{.Comment}}
{{- end}}
		{{.Type}} {{.Name}};
{{- end}}
	}

	mapping({{.Key.Type}} => {{.Struct}}) private _records;
	{{.Key.Type}}[] private _keys;
	// 1-based position of each key in _keys; 0 means absent.
	mapping({{.Key.Type}} => uint256) private _keyIndex;
{{- if eq .Access "ownable"}}

	// Ownable: only the deployer may write; reads stay open.
	address private immutable _owner;
{{- else if eq .Access "per_record_owner"}}

	// Per-record ownership: the creator of a record is the only account that may
	// update or remove it.
	mapping({{.Key.Type}} => address) private _recordOwner;
{{- end}}

	// The key is emitted non-indexed when it is a dynamic type (string/bytes) so
	// off-chain indexers can recover its value, not just match its topic hash.
	// Fields marked (web3.v1.field).indexed become extra indexed topics so a
	// subgraph or log filter can query records by them.
	event {{.Struct}}Created({{.Key.Type}}{{if .Key.Indexed}} indexed{{end}} key{{range .IndexedFields}}, {{.Type}} indexed {{.Name}}{{end}});
	event {{.Struct}}Updated({{.Key.Type}}{{if .Key.Indexed}} indexed{{end}} key{{range .IndexedFields}}, {{.Type}} indexed {{.Name}}{{end}});
	event {{.Struct}}Deleted({{.Key.Type}}{{if .Key.Indexed}} indexed{{end}} key);
{{- if eq .Access "ownable"}}

	/// @notice Sets the contract owner to the deployer.
	constructor() {
		_owner = msg.sender;
	}

	/// @notice Restricts a write to the contract owner.
	modifier onlyOwner() {
		require(msg.sender == _owner, "{{.Struct}}: not owner");
		_;
	}
{{- end}}

	/// @notice Store a new record. Reverts if a record with the same key exists.
	function create({{.Struct}} calldata record) external{{if eq .Access "ownable"}} onlyOwner{{end}} {
		require(_keyIndex[record.{{.Key.Field}}] == 0, "{{.Struct}}: already exists");
		_records[record.{{.Key.Field}}] = record;
		_keys.push(record.{{.Key.Field}});
		_keyIndex[record.{{.Key.Field}}] = _keys.length;
{{- if eq .Access "per_record_owner"}}
		_recordOwner[record.{{.Key.Field}}] = msg.sender;
{{- end}}
		emit {{.Struct}}Created(record.{{.Key.Field}}{{range .IndexedFields}}, record.{{.Name}}{{end}});
	}

	/// @notice Fetch a record by key. Reverts if absent.
	function get({{.Key.Type}}{{.Key.Loc}} key) external view returns ({{.Struct}} memory) {
		require(_keyIndex[key] != 0, "{{.Struct}}: not found");
		return _records[key];
	}

	/// @notice Overwrite an existing record. Reverts if absent.
	function update({{.Struct}} calldata record) external{{if eq .Access "ownable"}} onlyOwner{{end}} {
		require(_keyIndex[record.{{.Key.Field}}] != 0, "{{.Struct}}: not found");
{{- if eq .Access "per_record_owner"}}
		require(_recordOwner[record.{{.Key.Field}}] == msg.sender, "{{.Struct}}: not record owner");
{{- end}}
		_records[record.{{.Key.Field}}] = record;
		emit {{.Struct}}Updated(record.{{.Key.Field}}{{range .IndexedFields}}, record.{{.Name}}{{end}});
	}

	/// @notice Delete a record by key. Reverts if absent.
	function remove({{.Key.Type}}{{.Key.Loc}} key) external{{if eq .Access "ownable"}} onlyOwner{{end}} {
		uint256 idx = _keyIndex[key];
		require(idx != 0, "{{.Struct}}: not found");
{{- if eq .Access "per_record_owner"}}
		require(_recordOwner[key] == msg.sender, "{{.Struct}}: not record owner");
{{- end}}
		uint256 i = idx - 1;
		uint256 last = _keys.length - 1;
		if (i != last) {
			{{.Key.Type}}{{if .Key.Loc}} memory{{end}} moved = _keys[last];
			_keys[i] = moved;
			_keyIndex[moved] = idx;
		}
		_keys.pop();
		delete _keyIndex[key];
		delete _records[key];
{{- if eq .Access "per_record_owner"}}
		delete _recordOwner[key];
{{- end}}
		emit {{.Struct}}Deleted(key);
	}

	/// @notice True when a record with the given key exists.
	function exists({{.Key.Type}}{{.Key.Loc}} key) external view returns (bool) {
		return _keyIndex[key] != 0;
	}

	/// @notice Number of stored records.
	function count() external view returns (uint256) {
		return _keys.length;
	}

	/// @notice Every stored record.
	function list() external view returns ({{.Struct}}[] memory items) {
		items = new {{.Struct}}[](_keys.length);
		for (uint256 i = 0; i < _keys.length; i++) {
			items[i] = _records[_keys[i]];
		}
	}
}
