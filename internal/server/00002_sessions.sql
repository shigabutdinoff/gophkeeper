-- +goose Up
CREATE TABLE sessions (
	id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
	account_id uuid NOT NULL REFERENCES accounts (id) ON DELETE CASCADE,
	refresh_token varchar(26) NOT NULL UNIQUE
);

-- +goose Down
DROP TABLE sessions;
