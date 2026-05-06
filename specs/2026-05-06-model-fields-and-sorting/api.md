# API

## ModelView

`ModelView` removes these JSON properties:

- `title`
- `developer`
- `series`

The model payload becomes:

```json
{
  "name": "gpt-4o",
  "disabled": false,
  "pricing": {
    "currency": "USD",
    "tiers": []
  },
  "annotations": {}
}
```

`pricing` remains optional in API responses and requests. `annotations` remains present and defaults to an empty object.

## Endpoints

The following operations use the updated `ModelView` schema:

- `GET /api/picotera/models`
- `GET /api/picotera/models/{name}`
- `PUT /api/picotera/models`

Clients must stop sending `title`, `developer`, and `series` in `PUT /api/picotera/models` bodies. The server contract rejects payloads according to the generated Huma schema.
