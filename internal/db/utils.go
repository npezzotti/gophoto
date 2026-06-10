package db

import "database/sql"

func nullInt32Ptr(value sql.NullInt32) *int32 {
	if !value.Valid {
		return nil
	}

	v := value.Int32
	return &v
}

func ptrToNullInt32(value *int32) sql.NullInt32 {
	if value == nil {
		return sql.NullInt32{}
	}

	return sql.NullInt32{Int32: *value, Valid: true}
}
