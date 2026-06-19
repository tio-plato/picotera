package main

import (
	"context"
	"errors"
	"fmt"
	"log"
	"os"
	"strconv"

	"picotera/pkg/configx"
	"picotera/pkg/db"
	"picotera/pkg/server"

	"github.com/danielgtaylor/huma/v2/humacli"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
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

	cli.Root().AddCommand(&cobra.Command{
		Use:   "set-admin <user-id>",
		Short: "Grant admin to a user by id",
		Args:  cobra.ExactArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			id, err := strconv.ParseInt(args[0], 10, 64)
			if err != nil {
				log.Fatalf("invalid user id %q: %v", args[0], err)
			}

			ctx := context.Background()
			config, err := configx.Parse()
			if err != nil {
				log.Fatalf("failed to parse config: %v", err)
			}

			pool, err := pgxpool.New(ctx, config.DatabaseURL)
			if err != nil {
				log.Fatalf("failed to connect to database: %v", err)
			}
			defer pool.Close()

			user, err := db.New(pool).UpdateUserAdmin(ctx, db.UpdateUserAdminParams{ID: id, IsAdmin: true})
			if err != nil {
				if errors.Is(err, pgx.ErrNoRows) {
					log.Printf("user %d not found", id)
					os.Exit(1)
				}
				log.Fatalf("failed to set admin: %v", err)
			}

			fmt.Printf("user %d (%s) is now an admin\n", user.ID, user.DisplayName)
		},
	})

	cli.Run()
}
