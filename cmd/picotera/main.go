package main

import (
	"context"
	"fmt"
	"log"
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

	cli.Run()
}
