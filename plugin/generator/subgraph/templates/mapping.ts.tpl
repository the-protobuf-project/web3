{{.Header}}

import { Address, store } from "@graphprotocol/graph-ts";
import { {{.Entities}} } from "../generated/schema";
{{- range .Handlers}}
import { {{.Contract}}, {{.Entity}}Created, {{.Entity}}Updated, {{.Entity}}Deleted } from "../generated/{{.Entity}}/{{.Contract}}";
{{- end}}
{{range .Handlers}}
export function handle{{.Entity}}Created(event: {{.Entity}}Created): void {
  upsert{{.Entity}}(event.params.key, event.address);
}

export function handle{{.Entity}}Updated(event: {{.Entity}}Updated): void {
  upsert{{.Entity}}(event.params.key, event.address);
}

export function handle{{.Entity}}Deleted(event: {{.Entity}}Deleted): void {
  store.remove("{{.Entity}}", event.params.key);
}

// upsert{{.Entity}} reads the record from the contract and writes the entity.
function upsert{{.Entity}}(key: string, address: Address): void {
  let record = {{.Contract}}.bind(address).get(key);
  let entity = new {{.Entity}}(key);
{{- range .Assigns}}
  entity.{{.EntityField}} = record.{{.RecordField}};
{{- end}}
  entity.save();
}
{{end}}