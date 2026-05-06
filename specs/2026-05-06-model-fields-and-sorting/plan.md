# Plan

1. Add a goose migration that drops `model.title`, `model.developer`, and `model.series` in the up migration and recreates them as nullable `TEXT` columns in the down migration.
2. Update `db/migrations/001_initial.sql` so the initial `model` table contains only `name`.
3. Update `db/queries/model.sql`:
   - `GetModelByName` selects `name`, `disabled`, `pricing`, and `annotations`.
   - `GetModels` selects `name`, `disabled`, `pricing`, and `annotations`.
   - `UpsertModel` inserts and updates `name`, `disabled`, `pricing`, and `annotations`.
4. Run `sqlc generate`.
5. Update `pkg/contract/model.go`:
   - remove `Title`, `Developer`, and `Series` from `ModelView`
   - remove deleted fields from `ToModelView`
   - remove deleted fields from `FromModelView`
6. Update dashboard model form:
   - remove `title`, `developer`, and `series` from local form state
   - remove the three fields from the submit body
   - remove the three input controls
   - use `name` for the side panel title
7. Update dashboard models view:
   - create a computed sorted model list with enabled models first and disabled models last
   - sort each enabled/disabled group by `name`
   - render the table from the sorted list
   - remove title, developer, and series columns
   - remove deleted fields from the disabled-toggle PUT body
8. Regenerate API artifacts:
   - run `mise run openapi`
   - run `pnpm --dir dashboard generate-openapi`
9. Validate:
   - run `go test ./pkg/server ./pkg/llmbridge`
   - run `pnpm --dir dashboard type-check`
   - run `pnpm --dir dashboard build`
10. Review the diff to confirm no deleted model fields remain in source, generated API types, or OpenAPI schema.
