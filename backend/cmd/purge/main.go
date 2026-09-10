// purge physically removes soft-deleted rows left from older versions that
// blocked unique indexes. Run once after upgrading to hard-delete behavior.
package main

import (
	"log"

	"github.com/lihefengbj/nanyicrm/backend/internal/config"
	"github.com/lihefengbj/nanyicrm/backend/internal/database"
)

func main() {
	cfg := config.Load()
	db, err := database.NewMySQL(cfg.DefaultMySQL(), false)
	if err != nil {
		log.Fatalf("mysql: %v", err)
	}
	tables := []string{"sys_user", "sys_role", "sys_dept", "sys_menu", "sys_dict", "sys_dict_item"}
	for _, t := range tables {
		res := db.Exec("DELETE FROM " + t + " WHERE deleted_at IS NOT NULL")
		if res.Error != nil {
			log.Fatalf("purge %s: %v", t, res.Error)
		}
		log.Printf("purged %s: %d rows", t, res.RowsAffected)
	}
}

