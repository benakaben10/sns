CREATE TABLE IF NOT EXISTS smtp_configs (
    id           BIGSERIAL PRIMARY KEY,
    name         TEXT NOT NULL,
    from_email   TEXT,
    host         TEXT NOT NULL,
    port         INT  NOT NULL,
    username     TEXT NOT NULL,
    password     TEXT NOT NULL,
    use_tls      BOOLEAN NOT NULL DEFAULT true,
    use_starttls BOOLEAN NOT NULL DEFAULT false,
    is_default   BOOLEAN NOT NULL DEFAULT false,
    created_at   TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at   TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_smtp_configs_from_email ON smtp_configs(from_email);
CREATE INDEX IF NOT EXISTS idx_smtp_configs_is_default ON smtp_configs(is_default);
