package main

import (
	"context"
	"fmt"
	"log"
	"picotera/pkg/configx"
	"picotera/pkg/llmbridge"
	"picotera/pkg/server"

	"github.com/danielgtaylor/huma/v2/humacli"
	"github.com/spf13/cobra"
)

type Options struct{}

func main() {
	cli := humacli.New(func(h humacli.Hooks, o *Options) {
		h.OnStart(func() {
			ctx := context.Background()
			server, err := server.NewServer(ctx)
			if err != nil {
				log.Fatalf("failed to create server: %v", err)
			}

			err = server.Serve()
			if err != nil {
				log.Fatalf("failed to serve: %v", err)
			}
		})
	})

	cli.Root().AddCommand(&cobra.Command{
		Use:   "openapi",
		Short: "Generate OpenAPI specification",
		Run: func(cmd *cobra.Command, args []string) {
			api := server.NewHuma()
			b, _ := api.OpenAPI().DowngradeYAML()
			fmt.Println(string(b))
		},
	})

	precompileCmd := &cobra.Command{
		Use:   "precompile-llmbridge-wasm",
		Short: "Precompile the llmbridge WASM module into the wazero cache",
		Run: func(cmd *cobra.Command, args []string) {
			ctx := context.Background()
			config, err := configx.Parse()
			if err != nil {
				log.Fatalf("failed to parse config: %v", err)
			}
			if err := llmbridge.Precompile(ctx, llmbridge.Config{
				WASMPath:    config.LLMBridgeWASMPath,
				CacheDir:    config.LLMBridgeWASMCacheDir,
				RuntimeMode: config.LLMBridgeWASMRuntime,
			}); err != nil {
				log.Fatalf("failed to precompile llmbridge wasm: %v", err)
			}
			cacheDir := config.LLMBridgeWASMCacheDir
			if cacheDir == "" {
				cacheDir = llmbridge.DefaultCacheDir(config.LLMBridgeWASMPath)
			}
			fmt.Printf("precompiled %s into %s\n", config.LLMBridgeWASMPath, cacheDir)
		},
	}
	cli.Root().AddCommand(precompileCmd)

	cli.Run()
}
