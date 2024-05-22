-- name: GetUserByEmail :one
SELECT * FROM users
WHERE email = $1
LIMIT 1;

-- name: GetUserById :one
SELECT * FROM users
WHERE id = $1
LIMIT 1;

-- name: UserExists :one
SELECT EXISTS(
  SELECT 1
  FROM users
  WHERE id = $1
);

-- name: CreateUser :one
INSERT INTO users (
  first_name, last_name, email, password_hash
) VALUES (
  $1, $2, $3, $4
)
RETURNING *;

-- name: UpdateUser :one
UPDATE users
SET 
  first_name = $2,
  last_name = $3,
  email = $4,
  password_hash = $5,
  profile_picture_key = $6,
  updated_at = $7
WHERE id = $1
RETURNING *;

-- name: DeleteUser :exec
DELETE FROM users
WHERE id = $1;
