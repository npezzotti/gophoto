// Code generated by sqlc. DO NOT EDIT.
// versions:
//   sqlc v1.24.0
// source: photos.sql

package db

import (
	"context"
	"database/sql"
)

const createPhoto = `-- name: CreatePhoto :one
INSERT INTO photos (
  album_id,
  key
) VALUES (
  $1,
  $2
)
RETURNING id, album_id, key, created_at
`

type CreatePhotoParams struct {
	AlbumID sql.NullInt32
	Key     string
}

func (q *Queries) CreatePhoto(ctx context.Context, arg CreatePhotoParams) (Photo, error) {
	row := q.db.QueryRowContext(ctx, createPhoto, arg.AlbumID, arg.Key)
	var i Photo
	err := row.Scan(
		&i.ID,
		&i.AlbumID,
		&i.Key,
		&i.CreatedAt,
	)
	return i, err
}

const deletePhoto = `-- name: DeletePhoto :exec
DELETE FROM photos
WHERE id = $1
`

func (q *Queries) DeletePhoto(ctx context.Context, id int32) error {
	_, err := q.db.ExecContext(ctx, deletePhoto, id)
	return err
}

const getAlbumCover = `-- name: GetAlbumCover :one
SELECT id, album_id, key, created_at FROM photos
WHERE album_id = $1
ORDER BY created_at DESC
LIMIT 1
`

func (q *Queries) GetAlbumCover(ctx context.Context, albumID sql.NullInt32) (Photo, error) {
	row := q.db.QueryRowContext(ctx, getAlbumCover, albumID)
	var i Photo
	err := row.Scan(
		&i.ID,
		&i.AlbumID,
		&i.Key,
		&i.CreatedAt,
	)
	return i, err
}

const getOrphanedPhotos = `-- name: GetOrphanedPhotos :many
SELECT id, album_id, key, created_at FROM photos
WHERE album_id IS NULL
LIMIT 10
`

func (q *Queries) GetOrphanedPhotos(ctx context.Context) ([]Photo, error) {
	rows, err := q.db.QueryContext(ctx, getOrphanedPhotos)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var items []Photo
	for rows.Next() {
		var i Photo
		if err := rows.Scan(
			&i.ID,
			&i.AlbumID,
			&i.Key,
			&i.CreatedAt,
		); err != nil {
			return nil, err
		}
		items = append(items, i)
	}
	if err := rows.Close(); err != nil {
		return nil, err
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return items, nil
}

const getPhoto = `-- name: GetPhoto :one
SELECT id, album_id, key, created_at FROM photos
WHERE id = $1 LIMIT 1
`

func (q *Queries) GetPhoto(ctx context.Context, id int32) (Photo, error) {
	row := q.db.QueryRowContext(ctx, getPhoto, id)
	var i Photo
	err := row.Scan(
		&i.ID,
		&i.AlbumID,
		&i.Key,
		&i.CreatedAt,
	)
	return i, err
}

const listPhotosByAlbum = `-- name: ListPhotosByAlbum :many
SELECT id, album_id, key, created_at FROM photos
WHERE album_id = $1
LIMIT $2 
OFFSET $3
`

type ListPhotosByAlbumParams struct {
	AlbumID sql.NullInt32
	Limit   int32
	Offset  int32
}

func (q *Queries) ListPhotosByAlbum(ctx context.Context, arg ListPhotosByAlbumParams) ([]Photo, error) {
	rows, err := q.db.QueryContext(ctx, listPhotosByAlbum, arg.AlbumID, arg.Limit, arg.Offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var items []Photo
	for rows.Next() {
		var i Photo
		if err := rows.Scan(
			&i.ID,
			&i.AlbumID,
			&i.Key,
			&i.CreatedAt,
		); err != nil {
			return nil, err
		}
		items = append(items, i)
	}
	if err := rows.Close(); err != nil {
		return nil, err
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return items, nil
}