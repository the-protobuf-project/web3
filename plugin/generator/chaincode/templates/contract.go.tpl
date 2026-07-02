{{.Header}}
package {{.Package}}

import (
	"encoding/json"
	"fmt"

	"github.com/hyperledger/fabric-contract-api-go/contractapi"
)
{{range .Enums}}
// {{.Comment}}
type {{.Name}} string

const (
{{- $t := .Name}}
{{- range .Values}}
{{- if .Comment}}
	// {{.Comment}}
{{- end}}
	{{.Const}} {{$t}} = "{{.MapName}}"
{{- end}}
)
{{end}}
{{- range .Assets}}
// {{.Comment}}
type {{.Struct}} struct {
{{- range .Fields}}
{{- if .Comment}}
	// {{.Comment}}
{{- end}}
	{{.Name}} {{.Type}} `json:"{{.JSONTag}}"`
{{- end}}
{{- if .Access}}
	// Owner is the client identity permitted to update or delete this asset.
	Owner string `json:"owner"`
{{- end}}
}

// {{.Contract}} provides world-state CRUD transactions for {{.Struct}} assets.
type {{.Contract}} struct {
	contractapi.Contract
}

// stateKey is the world-state key an asset is stored under.
func (c *{{.Contract}}) stateKey(key {{.KeyType}}) string {
	return fmt.Sprintf("{{.Struct}}:%v", key)
}

// Create stores a new {{.Struct}}. Fails if one with the same key already exists.
func (c *{{.Contract}}) Create(ctx contractapi.TransactionContextInterface, asset {{.Struct}}) error {
	sk := c.stateKey(asset.{{.KeyName}})
	existing, err := ctx.GetStub().GetState(sk)
	if err != nil {
		return fmt.Errorf("read state: %w", err)
	}
	if existing != nil {
		return fmt.Errorf("{{.Struct}} %v already exists", asset.{{.KeyName}})
	}
{{- if .Access}}
	id, err := ctx.GetClientIdentity().GetID()
	if err != nil {
		return fmt.Errorf("client identity: %w", err)
	}
	asset.Owner = id
{{- end}}
	data, err := json.Marshal(asset)
	if err != nil {
		return err
	}
	return ctx.GetStub().PutState(sk, data)
}

// Read returns the {{.Struct}} stored under key. Fails if absent.
func (c *{{.Contract}}) Read(ctx contractapi.TransactionContextInterface, key {{.KeyType}}) (*{{.Struct}}, error) {
	data, err := ctx.GetStub().GetState(c.stateKey(key))
	if err != nil {
		return nil, fmt.Errorf("read state: %w", err)
	}
	if data == nil {
		return nil, fmt.Errorf("{{.Struct}} %v not found", key)
	}
	var asset {{.Struct}}
	if err := json.Unmarshal(data, &asset); err != nil {
		return nil, err
	}
	return &asset, nil
}

// Update overwrites an existing {{.Struct}}. Fails if absent.
func (c *{{.Contract}}) Update(ctx contractapi.TransactionContextInterface, asset {{.Struct}}) error {
{{- if .Access}}
	current, err := c.Read(ctx, asset.{{.KeyName}})
	if err != nil {
		return err
	}
	id, err := ctx.GetClientIdentity().GetID()
	if err != nil {
		return fmt.Errorf("client identity: %w", err)
	}
	if current.Owner != id {
		return fmt.Errorf("{{.Struct}} %v: caller is not the owner", asset.{{.KeyName}})
	}
	asset.Owner = current.Owner
{{- else}}
	exists, err := c.Exists(ctx, asset.{{.KeyName}})
	if err != nil {
		return err
	}
	if !exists {
		return fmt.Errorf("{{.Struct}} %v not found", asset.{{.KeyName}})
	}
{{- end}}
	data, err := json.Marshal(asset)
	if err != nil {
		return err
	}
	return ctx.GetStub().PutState(c.stateKey(asset.{{.KeyName}}), data)
}

// Delete removes the {{.Struct}} stored under key. Fails if absent.
func (c *{{.Contract}}) Delete(ctx contractapi.TransactionContextInterface, key {{.KeyType}}) error {
{{- if .Access}}
	current, err := c.Read(ctx, key)
	if err != nil {
		return err
	}
	id, err := ctx.GetClientIdentity().GetID()
	if err != nil {
		return fmt.Errorf("client identity: %w", err)
	}
	if current.Owner != id {
		return fmt.Errorf("{{.Struct}} %v: caller is not the owner", key)
	}
{{- else}}
	exists, err := c.Exists(ctx, key)
	if err != nil {
		return err
	}
	if !exists {
		return fmt.Errorf("{{.Struct}} %v not found", key)
	}
{{- end}}
	return ctx.GetStub().DelState(c.stateKey(key))
}

// Exists reports whether a {{.Struct}} is stored under key.
func (c *{{.Contract}}) Exists(ctx contractapi.TransactionContextInterface, key {{.KeyType}}) (bool, error) {
	data, err := ctx.GetStub().GetState(c.stateKey(key))
	if err != nil {
		return false, fmt.Errorf("read state: %w", err)
	}
	return data != nil, nil
}

// List returns every stored {{.Struct}}.
func (c *{{.Contract}}) List(ctx contractapi.TransactionContextInterface) ([]*{{.Struct}}, error) {
	iter, err := ctx.GetStub().GetStateByRange("{{.Struct}}:", "{{.Struct}}:￿")
	if err != nil {
		return nil, fmt.Errorf("range state: %w", err)
	}
	defer iter.Close()
	var out []*{{.Struct}}
	for iter.HasNext() {
		kv, err := iter.Next()
		if err != nil {
			return nil, err
		}
		var asset {{.Struct}}
		if err := json.Unmarshal(kv.Value, &asset); err != nil {
			return nil, err
		}
		out = append(out, &asset)
	}
	return out, nil
}
{{end}}