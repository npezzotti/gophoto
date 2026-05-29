-- name: GetPhoto :one
SELECT *
FROM photos
WHERE id = $1 
LIMIT 1;

-- name: CreatePhoto :one
INSERT INTO photos (
  user_id,
  key,
  status
) VALUES (
  $1,
  $2,
  $3
)
RETURNING *;

-- name: UpdatePhotoStatus :exec
UPDATE photos
SET
  status = $2,
  updated_at = $3
WHERE id = $1;

-- name: DeletePhoto :exec
DELETE FROM photos
WHERE id = $1;

-- name: GetOrphanedPhotos :many
SELECT * FROM photos p
WHERE p.id NOT IN (
  SELECT ap.photo_id 
  FROM album_photos ap
)
AND p.id NOT IN (
  SELECT u.profile_picture_id 
  FROM users u 
  WHERE u.profile_picture_id IS NOT NULL
)
LIMIT 10;
