-- +goose Up
CREATE TABLE records (
	id uuid NOT NULL,
	user_id uuid NOT NULL,
	note text NOT NULL DEFAULT '',
	secret text NOT NULL,
	updated_at timestamptz NOT NULL DEFAULT now(),
	PRIMARY KEY (user_id, id)
);

-- +goose Down
DROP TABLE records;
