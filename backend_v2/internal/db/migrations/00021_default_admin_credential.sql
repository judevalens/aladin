-- +goose Up

INSERT INTO users (id, email, password_hash, created_at, updated_at)
VALUES (
    '00000000-0000-0000-0000-000000000001',
    'admin@email.com',
    'pbkdf2_sha256$210000$YWxhZGluLWRldi1hZG1pbg$iONMwnrln6ivij4VdCYBMDNzlx8nKTrdQQhGssIkXh8',
    now(),
    now()
)
ON CONFLICT (id) DO UPDATE
SET email = EXCLUDED.email,
    password_hash = EXCLUDED.password_hash,
    updated_at = now();

-- +goose Down

UPDATE users
SET password_hash = NULL,
    updated_at = now()
WHERE id = '00000000-0000-0000-0000-000000000001'
  AND email = 'admin@email.com';
