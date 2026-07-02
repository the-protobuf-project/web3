package generator_test

// Golden-file tests for the blockchain backends. Each directory under
// testdata/cases/ is one case: its .proto files are compiled in-process (via the
// protokit golden harness) and the solidity/subgraph output is compared
// byte-for-byte against <case>/golden/<target>/. The bookstore example (which
// pulls the real example protos) is covered by the buf-generate acceptance test
// in the protorm repo instead.
//
// Regenerate goldens after an intentional output change:
//
//	go test . -run TestGolden -update

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/the-protobuf-project/protokit/golden"
	"github.com/the-protobuf-project/protokit/header"
	"github.com/the-protobuf-project/protokit/schema"
	"github.com/the-protobuf-project/web3/plugin/generator"
	"github.com/the-protobuf-project/web3/plugin/generator/backend"
)

// defaultTargets are the chain backends every golden case runs: EVM (solidity +
// subgraph), Solana (solana), and Fabric (chaincode).
var defaultTargets = []string{"solidity", "subgraph", "solana", "chaincode"}

// web3Backend builds the web3 Backend for one golden case, reading any web3.yaml
// the case ships (grouping config). protokit's harness stays generator-neutral —
// this generator-specific knowledge lives here, not in RunCase.
func web3Backend(dir string) schema.Backend {
	var cfg *backend.Config
	if path := filepath.Join(dir, "web3.yaml"); fileExists(path) {
		c, err := backend.LoadConfig(path)
		if err != nil {
			panic(err) // a malformed fixture config is a test-authoring bug
		}
		cfg = c
	}
	return backend.New(cfg, "")
}

func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

// TestMain stamps the web3 tool name into generated banners so goldens match
// what the protoc-gen-web3 binary produces (the shared harness defaults to
// protoc-gen-orm).
func TestMain(m *testing.M) {
	header.SetTool("protoc-gen-web3")
	os.Exit(m.Run())
}

func TestGolden(t *testing.T) {
	cases, err := os.ReadDir("testdata/cases")
	if err != nil {
		t.Fatalf("read cases: %v", err)
	}
	for _, c := range cases {
		if !c.IsDir() {
			continue
		}
		t.Run(c.Name(), func(t *testing.T) {
			golden.RunCase(t, filepath.Join("testdata", "cases", c.Name()), generator.Targets(), defaultTargets, web3Backend)
		})
	}
}
