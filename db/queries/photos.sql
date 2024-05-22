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
  key
) VALUES (
  $1,
  $2
)
RETURNING *;

-- name: DeletePhoto :exec
DELETE FROM photos
WHERE id = $1;

-- name: GetOrphanedPhotos :many
SELECT * FROM photos
WHERE album_id IS NULL
LIMIT 10;
