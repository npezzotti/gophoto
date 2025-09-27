-- name: GetPhoto :one
SELECT * FROM photos
WHERE id = $1 LIMIT 1;

-- name: GetAlbumCover :one
SELECT * FROM photos
WHERE album_id = $1
ORDER BY created_at DESC
LIMIT 1; 

-- name: ListPhotosByAlbum :many
SELECT * FROM photos
WHERE album_id = $1
LIMIT $2 
OFFSET $3;

-- name: CreatePhoto :one
INSERT INTO photos (
  album_id,
  key,
  status
) VALUES (
  $1,
  $2,
  $3
)
RETURNING *;

-- name: UpdatePhoto :one
UPDATE photos
SET
  album_id = $2
WHERE id = $1
RETURNING *;

-- name: UpdatePhotoStatus :exec
UPDATE photos
SET
  status = $2
WHERE id = $1;

-- name: DeletePhoto :exec
DELETE FROM photos
WHERE id = $1;

-- name: GetOrphanedPhotos :many
SELECT * FROM photos
WHERE album_id IS NULL
LIMIT 10;
