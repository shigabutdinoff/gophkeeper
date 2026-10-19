-- +goose Up
CREATE TABLE records (
	id uuid NOT NULL,
	user_id uuid NOT NULL REFERENCES accounts (id) ON DELETE CASCADE,
	note varchar(65536) NOT NULL DEFAULT '',
	secret varchar(65536) NOT NULL,
	updated_at timestamptz NOT NULL DEFAULT now(),
	PRIMARY KEY (user_id, id)
);

-- +goose Down
DROP TABLE records;
