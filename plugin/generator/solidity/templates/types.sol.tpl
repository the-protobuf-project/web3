// SPDX-License-Identifier: Apache-2.0
{{.Header}}
pragma solidity ^{{.Solc}};
{{range .Enums}}
/// @notice {{.Comment}}
enum {{.Name}} {
{{- range .Values}}
{{- if .Comment}}
	// {{.Comment}}
{{- end}}
	{{.Name}}{{if not .Last}},{{end}}
{{- end}}
}
{{end}}