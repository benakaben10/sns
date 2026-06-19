-- Sample SMTP seed data for local development.
-- Replace CHANGE_ME values with real credentials before use.
-- Do NOT commit real credentials to version control.

INSERT INTO smtp_configs (name, from_email, host, port, username, password, use_tls, use_starttls, is_default)
VALUES
    (
        'Default SMTP (Gmail STARTTLS)',
        NULL,
        'smtp.gmail.com',
        587,
        'no-reply@example.com',
        'CHANGE_ME',
        false,
        true,
        true
    ),
    (
        'Sender Specific SMTP (TLS)',
        'sender@example.com',
        'smtp.example.com',
        465,
        'sender@example.com',
        'CHANGE_ME',
        true,
        false,
        false
    )
ON CONFLICT DO NOTHING;


