package utils

import (
	"fmt"

	"github.com/npezzotti/gophoto/internal/db"
)

// BuildPhotoPath constructs a hierarchical file path for storing photos based on the provided key and variant.
func BuildPhotoPath(key string, variant db.PhotoVariant) string {
	// Use the first four characters of the UUID to create a two-level directory structure.
	shardLvl1 := key[0:2]
	shardLvl2 := key[2:4]

	return fmt.Sprintf("/%s/%s/%s/%s", shardLvl1, shardLvl2, key, (string(variant)))
}
