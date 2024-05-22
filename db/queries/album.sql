-- name: GetAlbum :one
SELECT a.*, COUNT(p.*) AS num_photos
FROM albums a
  LEFT JOIN photos p on p.album_id = a.id
WHERE a.id = $1 
GROUP BY a.id
LIMIT 1;

-- name: ListAlbumsByUser :many
SELECT a.*, COUNT(p.*) AS num_photos
FROM albums a
  LEFT JOIN photos p ON p.album_id = a.id
WHERE a.user_id = $1
GROUP BY a.id
LIMIT $2
OFFSET $3;

-- name: CreateAlbum :one
INSERT INTO albums (
  user_id, title
) VALUES (
  $1, $2
)
RETURNING *;

-- name: UpdateAlbum :exec
UPDATE albums
  SET user_id = $2,
  title = $3,
  updated_at = $4
WHERE id = $1
RETURNING *;

-- name: DeleteAlbum :exec
DELETE FROM albums
WHERE id = $1;

-- name: CountAlbumsByUser :one
SELECT COUNT(*) FROM albums
WHERE user_id = $1;
