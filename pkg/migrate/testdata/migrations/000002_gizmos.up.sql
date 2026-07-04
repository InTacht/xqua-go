CREATE TABLE gizmos (
    id       BIGSERIAL PRIMARY KEY,
    widget_id BIGINT NOT NULL REFERENCES widgets (id),
    label    TEXT NOT NULL
);
