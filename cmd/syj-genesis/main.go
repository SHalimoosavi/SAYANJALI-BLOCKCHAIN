package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"

	"github.com/SHalimoosavi/SAYANJALI-BLOCKCHAIN/internal/genesisconfig"
)

func main() {
	if len(os.Args) < 2 {
		usage()
		os.Exit(2)
	}

	switch os.Args[1] {
	case "generate":
		generate(os.Args[2:])
	case "validate":
		validate(os.Args[2:])
	case "verify":
		verify(os.Args[2:])
	case "print-summary":
		summary(os.Args[2:])
	case "help":
		usage()
	default:
		usage()
		os.Exit(2)
	}
}

func usage() {
	fmt.Fprintln(os.Stderr, "SYJ production genesis operator tool")
	fmt.Fprintln(os.Stderr, "commands:")
	fmt.Fprintln(os.Stderr, "  generate     --config <config.json> --out <artifact.json>")
	fmt.Fprintln(os.Stderr, "  validate     --config <config.json>")
	fmt.Fprintln(os.Stderr, "  verify       --artifact <artifact.json>")
	fmt.Fprintln(os.Stderr, "  print-summary --artifact <artifact.json>")
}

func generate(args []string) {
	fs := flag.NewFlagSet("generate", flag.ExitOnError)
	configPath := fs.String("config", "", "genesis configuration JSON")
	outPath := fs.String("out", "", "artifact output JSON")
	_ = fs.Parse(args)

	if *configPath == "" || *outPath == "" {
		fail(fmt.Errorf("--config and --out are required"))
	}

	cfg, err := genesisconfig.LoadConfig(*configPath)
	if err != nil {
		fail(err)
	}

	artifact, data, err := genesisconfig.BuildArtifact(cfg)
	if err != nil {
		fail(err)
	}

	if err := os.WriteFile(*outPath, data, 0600); err != nil {
		fail(err)
	}

	fmt.Printf("GENESIS ARTIFACT GENERATED\ncommitment=%s\noutput=%s\n",
		artifact.Commitment, *outPath)
}

func validate(args []string) {
	fs := flag.NewFlagSet("validate", flag.ExitOnError)
	configPath := fs.String("config", "", "genesis configuration JSON")
	_ = fs.Parse(args)

	if *configPath == "" {
		fail(fmt.Errorf("--config is required"))
	}

	cfg, err := genesisconfig.LoadConfig(*configPath)
	if err != nil {
		fail(err)
	}

	if err := genesisconfig.Validate(cfg); err != nil {
		fail(err)
	}

	fmt.Println("VALID")
}

func verify(args []string) {
	fs := flag.NewFlagSet("verify", flag.ExitOnError)
	artifactPath := fs.String("artifact", "", "genesis artifact JSON")
	_ = fs.Parse(args)

	if *artifactPath == "" {
		fail(fmt.Errorf("--artifact is required"))
	}

	artifact, err := genesisconfig.LoadArtifact(*artifactPath)
	if err != nil {
		fail(err)
	}

	if err := genesisconfig.VerifyArtifact(artifact); err != nil {
		fail(err)
	}

	fmt.Printf("VERIFIED\ncommitment=%s\n", artifact.Commitment)
}

func summary(args []string) {
	fs := flag.NewFlagSet("print-summary", flag.ExitOnError)
	artifactPath := fs.String("artifact", "", "genesis artifact JSON")
	_ = fs.Parse(args)

	if *artifactPath == "" {
		fail(fmt.Errorf("--artifact is required"))
	}

	artifact, err := genesisconfig.LoadArtifact(*artifactPath)
	if err != nil {
		fail(err)
	}

	data, err := json.MarshalIndent(artifact, "", "  ")
	if err != nil {
		fail(err)
	}

	fmt.Println(string(data))
}

func fail(err error) {
	fmt.Fprintln(os.Stderr, "ERROR:", err)
	os.Exit(1)
}
