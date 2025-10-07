package store

import (
	"fmt"

	"github.com/npezzotti/gophoto/db"
)

func BuildPhotoPath(key string, variant db.PhotoVariant) string {
	shardLvl1 := key[0:2]
	shardLvl2 := key[2:4]

	return fmt.Sprintf("/%s/%s/%s/%s", shardLvl1, shardLvl2, key, (string(variant)))
}
