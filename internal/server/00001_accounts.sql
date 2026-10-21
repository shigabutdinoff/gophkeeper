-- +goose Up
CREATE TABLE IF NOT EXISTS accounts (
	id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
	email varchar(254) NOT NULL UNIQUE,
	encrypted_password varchar(60) NOT NULL,
	created_at timestamptz NOT NULL DEFAULT now()
);

-- +goose Down
DROP TABLE accounts;
