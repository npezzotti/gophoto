-- name: GetAlbumById :one
SELECT 
  a.*, 
  (SELECT COUNT(*) FROM album_photos WHERE album_id = a.id) AS num_photos
FROM albums a
WHERE a.id = $1;

-- name: ListAlbumsByUser :many
SELECT                      
  a.id, 
  a.user_id,
  a.title,
  a.num_photos,
  a.created_at,
  a.updated_at,
  p_cover.id AS cover_photo_id,
  p_cover.key AS cover_photo_key
FROM albums a
LEFT JOIN photos p_cover ON p_cover.id = (
  SELECT photo_id 
  FROM album_photos 
  WHERE id = a.cover_photo_id 
  LIMIT 1
)
WHERE a.user_id = $1
GROUP BY 
  a.id, 
  a.user_id, 
  a.title, 
  a.created_at, 
  a.updated_at,
  p_cover.id, 
  p_cover.key
LIMIT $2
OFFSET $3;

-- name: CreateAlbum :one
INSERT INTO albums (
  user_id, title
) VALUES (
  $1, $2
)
RETURNING *;

-- name: SetAlbumCoverPhoto :exec
UPDATE albums
SET 
  cover_photo_id = $2,
  updated_at = $3
WHERE id = $1;

-- name: UpdateAlbum :one
UPDATE albums
  SET user_id = $2,
  title = $3,
  cover_photo_id = $4,
  updated_at = $5
WHERE id = $1
RETURNING *;

-- name: IncrementAlbumPhotoCount :exec
UPDATE albums
SET 
  num_photos = num_photos + 1,
  updated_at = $2
WHERE id = $1;

-- name: DecrementAlbumPhotoCount :exec
UPDATE albums
SET 
  num_photos = GREATEST(num_photos - 1, 0),
  updated_at = $2
WHERE id = $1;

-- name: DeleteAlbum :exec
DELETE FROM albums
WHERE id = $1;

-- name: ListAlbumPhotoViewRows :many
WITH page_photos AS (
  SELECT p.id, p.key, p.created_at
  FROM photos p
  JOIN album_photos ap ON ap.photo_id = p.id
  WHERE ap.album_id = $1
  ORDER BY p.created_at DESC, p.id DESC
  LIMIT $2
  OFFSET $3
)
SELECT
  pp.id AS photo_id,
  pp.key AS photo_key,
  pm.variant,
  pm.width,
  pm.height,
  pm.mime_type
FROM page_photos pp
LEFT JOIN photo_metadata pm ON pm.photo_id = pp.id
ORDER BY
  pp.created_at DESC,
  pp.id DESC;
