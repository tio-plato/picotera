# Design

The model resource will keep only operational fields that are still used by routing and management: `name`, `disabled`, `pricing`, and `annotations`. The removed fields are deleted from the database schema, sqlc queries, Go contract, generated OpenAPI schema, generated dashboard API types, and dashboard UI. No compatibility fields are retained in request or response payloads.

## Backend

The `model` table no longer stores `title`, `developer`, or `series`. A new goose migration drops these columns in the up migration and recreates them in the down migration. The initial migration is updated so a new database starts with the current schema.

`db/queries/model.sql` selects and upserts only the remaining model columns. `contract.ModelView` removes the deleted JSON properties, and `FromModelView` writes only the remaining fields. After these changes, `sqlc generate`, `mise run openapi`, and `pnpm --dir dashboard generate-openapi` update generated code and API types.

## Dashboard

`ModelForm.vue` removes the title, developer, and series inputs. The panel title uses the model `name`.

`ModelsView.vue` removes the title, developer, and series table columns and stops sending those fields when toggling disabled state. The model list uses a computed display array sorted by enabled state first and name second:

- enabled models sort before disabled models
- names sort ascending with `localeCompare`

The registered-model set and count continue to use the canonical `models` array from the API. The table iterates the sorted display array.

No third-party libraries are introduced.
