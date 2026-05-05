-- +goose Up
ALTER TABLE request ADD COLUMN user_message_preview TEXT;

-- +goose Down
ALTER TABLE request DROP COLUMN user_message_preview;
