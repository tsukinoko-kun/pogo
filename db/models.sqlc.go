// Code generated by sqlc. DO NOT EDIT.
// versions:
//   sqlc v1.29.0

package db

import (
	"github.com/jackc/pgx/v5/pgtype"
)

type Bookmark struct {
	ID           int64
	RepositoryID int32
	Name         string
	ChangeID     int64
}

type Change struct {
	ID           int64
	RepositoryID int32
	Name         string
	Description  *string
	Author       string
	Device       string
	Depth        int64
	CreatedAt    pgtype.Timestamptz
	UpdatedAt    pgtype.Timestamptz
}

type ChangeFile struct {
	ChangeID int64
	FileID   int64
}

type ChangeRelation struct {
	ChangeID int64
	ParentID *int64
}

type File struct {
	ID          int64
	Name        string
	Executable  bool
	ContentHash []byte
	Conflict    bool
}

type Repository struct {
	ID   int32
	Name string
}
