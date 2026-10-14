-- +goose Up
CREATE TABLE sessions (
	id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
	account_id uuid NOT NULL,
	refresh_token text NOT NULL UNIQUE
);

-- +goose Down
DROP TABLE sessions;
