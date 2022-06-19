package admin

import (
	"fmt"
	"regexp"

	"gorm.io/gorm"
)

var primaryKeyRegexp = regexp.MustCompile(`primary_key\[.+_.+\]`)

func (admin Admin) registerCompositePrimaryKeyCallback() {
	if db := admin.DB; db != nil {
		// register middleware
		router := admin.GetRouter()
		router.Use(&Middleware{
			Name: "composite primary key filter",
			Handler: func(context *Context, middleware *Middleware) {
				db := context.GetDB()
				for key, value := range context.Request.URL.Query() {
					if primaryKeyRegexp.MatchString(key) {
						db = db.Set(key, value)
					}
				}
				context.SetDB(db)

				middleware.Next(context)
			},
		})

		callbackProc := db.Callback().Query().Before("gorm:query")
		callbackName := "qor_admin:composite_primary_key"
		callbackProc.Register(callbackName, compositePrimaryKeyQueryCallback)

		callbackProc = db.Callback().Row().Before("gorm:row_query")
		callbackProc.Register(callbackName, compositePrimaryKeyQueryCallback)
	}
}

// DisableCompositePrimaryKeyMode disable composite primary key mode
var DisableCompositePrimaryKeyMode = "composite_primary_key:query:disable"

func compositePrimaryKeyQueryCallback(db *gorm.DB) {
	if value, ok := db.Get(DisableCompositePrimaryKeyMode); ok && value != "" {
		return
	}

	tableName := db.Begin().Statement.Table
	for _, primaryField := range db.Statement.Schema.PrimaryFields {
		if value, ok := db.Get(fmt.Sprintf("primary_key[%v_%v]", tableName, primaryField.DBName)); ok && value != "" {
			db.Where(fmt.Sprintf("%v = ?", db.NamingStrategy.ColumnName("", primaryField.DBName)), value)
		}
	}
}
