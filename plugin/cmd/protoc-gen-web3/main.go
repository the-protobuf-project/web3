// Command protoc-gen-web3 is a protoc plugin that generates on-chain artifacts
// from proto descriptors annotated with google.api.* and web3.v1 options: Solidity
// storage contracts (target=solidity) and a Graph subgraph that indexes their
// events (target=subgraph). The generic IR engine lives in the protokit module;
// the database (gorm/sql/prisma) targets ship as the separate protoc-gen-orm
// plugin.
//
// # Install
//
//	go install github.com/the-protobuf-project/web3/cmd/protoc-gen-web3@latest
//
// # Usage via buf.gen.yaml
//
//	plugins:
//	  - local: protoc-gen-web3
//	    out: generated/
//	    opt:
//	      - target=solidity   # solidity | subgraph
package main

import (
	"flag"
	"fmt"
	"os"

	core "github.com/the-protobuf-project/protokit"
	"github.com/the-protobuf-project/protokit/header"
	"github.com/the-protobuf-project/web3/plugin/generator"
	"github.com/the-protobuf-project/web3/plugin/generator/backend"
	"google.golang.org/protobuf/compiler/protogen"
	"google.golang.org/protobuf/types/pluginpb"
)

// version is the build version, injected at release time via
// -ldflags "-X main.version=...".
var version = "dev"

func main() {
	// When invoked directly with -version (not by protoc), print and exit before
	// protogen tries to read a CodeGeneratorRequest from stdin.
	if len(os.Args) == 2 && (os.Args[1] == "-version" || os.Args[1] == "--version") {
		fmt.Printf("protoc-gen-web3 %s\n", version)
		return
	}

	// Every generated file's banner names the tool that produced it.
	header.SetTool("protoc-gen-web3")

	// flags are populated by protogen (ParamFunc maps each buf.gen.yaml opt:
	// "key=value" to flags.Set) before the Run closure reads them.
	var flags flag.FlagSet
	target := flags.String("target", "", "output backend: solidity | subgraph | solana | chaincode")
	strict := flags.String("strict", "",
		"per-rule severity for schema problems: \"\"=all warn, \"true\"=all error, "+
			"or \"ref:error,collision:warn,index:error,lint:warn\"")
	config := flags.String("config", "",
		"path to a web3.yaml mapping proto packages to databases/schemas")
	goModule := flags.String("go_module", "",
		"chaincode target only: Go import path of the output directory (e.g. "+
			"github.com/me/gen). When set, the generated Fabric chaincode imports its "+
			"packages by this base path and omits a standalone go.mod, so the tree "+
			"compiles inside an enclosing Go module")

	protogen.Options{ParamFunc: flags.Set}.Run(func(p *protogen.Plugin) error {
		// Proto3 `optional` is fully supported (presence is read via field_behavior,
		// not synthetic oneofs); declare it so buf/protoc don't warn.
		p.SupportedFeatures = uint64(pluginpb.CodeGeneratorResponse_FEATURE_PROTO3_OPTIONAL)
		// web3 owns its layout config (protokit has none): load it here and hand the
		// backend a fully-resolved view of grouping.
		cfg, err := backend.LoadConfig(*config)
		if err != nil {
			return err
		}
		return core.Run(p, core.Options{
			Target:  *target,
			Strict:  *strict,
			Version: version,
		}, generator.Targets(), backend.New(cfg, *goModule))
	})
}
