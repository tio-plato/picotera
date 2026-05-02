-- +goose Up
UPDATE provider
   SET provider_models = COALESCE(
     (
       SELECT jsonb_agg(v || jsonb_build_object('model', k))
         FROM jsonb_each(provider_models) AS x(k, v)
        WHERE jsonb_typeof(v) = 'object'
     ),
     '[]'::jsonb
   )
 WHERE jsonb_typeof(provider_models) = 'object';

-- +goose Down
UPDATE provider
   SET provider_models = COALESCE(
     (
       SELECT jsonb_object_agg(elem ->> 'model', elem - 'model')
         FROM jsonb_array_elements(provider_models) AS elem
        WHERE elem ? 'model'
     ),
     '{}'::jsonb
   )
 WHERE jsonb_typeof(provider_models) = 'array';
