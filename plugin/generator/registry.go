// Package generator composes the blockchain backends protoc-gen-web3 ships: the
// EVM Solidity storage contracts and their Graph subgraph, Solana Anchor (Rust)
// programs, and Hyperledger Fabric (Go) chaincode. The IR frontend and shared
// infrastructure come from the protokit module.
package generator

import (
	"github.com/the-protobuf-project/protokit/schema"
	"github.com/the-protobuf-project/web3/plugin/generator/chaincode"
	"github.com/the-protobuf-project/web3/plugin/generator/solana"
	"github.com/the-protobuf-project/web3/plugin/generator/solidity"
	"github.com/the-protobuf-project/web3/plugin/generator/subgraph"
)

// Targets is the blockchain-backend registry, keyed by the value users write in
// buf.gen.yaml opt: [target=<key>]. EVM ships the Solidity storage contracts and
// their Graph subgraph; solana emits Anchor (Rust) programs; chaincode emits
// Hyperledger Fabric (Go) contracts.
func Targets() map[string]schema.Target {
	return map[string]schema.Target{
		"solidity":  &solidity.Generator{},
		"subgraph":  &subgraph.Generator{},
		"solana":    &solana.Generator{},
		"chaincode": &chaincode.Generator{},
	}
}
