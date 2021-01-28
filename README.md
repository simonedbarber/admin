## QOR Admin

Instantly create a beautiful, cross platform, configurable Admin Interface and API for managing your data in minutes.

[![GoDoc](https://godoc.org/github.com/qor/admin?status.svg)](https://godoc.org/github.com/qor/admin)
[![Build Status](https://travis-ci.com/qor/admin.svg?branch=master)](https://travis-ci.com/qor/admin)

**For security issues, please send us an email to security@getqor.com and give us time to respond BEFORE posting as an issue or reporting on public forums.**

## Features

- Generate Admin Interface for managing data
- RESTFul JSON API
- Association handling
- Search and filtering
- Actions/Batch Actions
- Authentication and Authorization
- Extendability

## Quick Start

```go
package main

import (
  "fmt"
  "net/http"
  "github.com/jinzhu/gorm"
  _ "github.com/mattn/go-sqlite3"
  "github.com/qor/admin"
)

// Create a GORM-backend model
type User struct {
  gorm.Model
  Name string
}

// Create another GORM-backend model
type Product struct {
  gorm.Model
  Name        string
  Description string
}

func main() {
  DB, _ := gorm.Open("sqlite3", "demo.db")
  DB.AutoMigrate(&User{}, &Product{})

  // Initialize
  Admin := admin.New(&admin.AdminConfig{DB: DB})

  // Allow to use Admin to manage User, Product
  Admin.AddResource(&User{})
  Admin.AddResource(&Product{})

  // initialize an HTTP request multiplexer
  mux := http.NewServeMux()

  // Mount admin interface to mux
  Admin.MountTo("/admin", mux)

  fmt.Println("Listening on: 9000")
  http.ListenAndServe(":9000", mux)
}
```

`go run main.go` and visit `localhost:9000/admin` to see the result!

## How to use remoteSelector with publish2.version record
Suppose we have 3 models. Factory **has many** Items and Factory **has one** Manager. See code comment for detail

```go
type Factory struct {
	gorm.Model
	Name string

	publish2.Version
	Items       []Item `gorm:"many2many:factory_items;association_autoupdate:false"`
	ItemsSorter sorting.SortableCollection

	ManagerID          uint
	ManagerVersionName string // Required. in "xxxVersionName" format.
	Manager            Manager
}

type Item struct {
	gorm.Model
	Name string
	publish2.Version

	// github.com/qor/qor/resource
	resource.CompositePrimaryKeyField // Required
}

type Manager struct {
	gorm.Model
	Name string

	publish2.Version

	// github.com/qor/qor/resource
	resource.CompositePrimaryKeyField // Required
}

itemSelector := generateRemoteItemSelector(adm)
factoryRes.Meta(&admin.Meta{
	Name: "Items",
	Config: &admin.SelectManyConfig{
	RemoteDataResource: itemSelector,
	},
})

managerSelector := generateRemoteManagerSelector(adm)
factoryRes.Meta(&admin.Meta{
	Name: "Manager",
	Config: &admin.SelectOneConfig{
	RemoteDataResource: managerSelector,
	},
})

func generateRemoteItemSelector(adm *admin.Admin) (res *admin.Resource) {
	res = adm.AddResource(&Item{}, &admin.Config{Name: "ItemSelector"})
	res.IndexAttrs("ID", "Name")

	// Required. Convert single ID into composite primary key
	res.Meta(&admin.Meta{
	Name: "ID",
	Valuer: func(value interface{}, ctx *qor.Context) interface{} {
		if r, ok := value.(*Item); ok {
		// github.com/qor/qor/resource
		return resource.GenCompositePrimaryKey(r.ID, r.GetVersionName())
		}
		return ""
	},
	})

	return res
}

func generateRemoteManagerSelector(adm *admin.Admin) (res *admin.Resource) {
	res = adm.AddResource(&Manager{}, &admin.Config{Name: "ManagerSelector"})
	res.IndexAttrs("ID", "Name")

	// Required. Convert single ID into composite primary key
	res.Meta(&admin.Meta{
	Name: "ID",
	Valuer: func(value interface{}, ctx *qor.Context) interface{} {
		if r, ok := value.(*Manager); ok {
		// github.com/qor/qor/resource
		return resource.GenCompositePrimaryKey(r.ID, r.GetVersionName())
		}
		return ""
	},
	})

	return res
}

```

If you need to overwrite Collection. you have to pass composite primary key like this

```go
factoryRes.Meta(&admin.Meta{
  Name: "Items",
  Config: &admin.SelectManyConfig{
  Collection: func(value interface{}, ctx *qor.Context) (results [][]string) {
    if c, ok := value.(*Factory); ok {
    var items []Item
    ctx.GetDB().Model(c).Related(&items, "Items")

    for _, p := range items {
      // The first element must be the composite primary key instead of ID
      results = append(results, []string{resource.GenCompositePrimaryKey(p.ID, p.GetVersionName()), p.Name})
      }
    }
    return
  },
  RemoteDataResource: itemSelector,
  },
})
```


## Live DEMO

* Live Demo [http://demo.getqor.com/admin](http://demo.getqor.com/admin)
* Source Code of Live Demo [https://github.com/qor/qor-example](https://github.com/qor/qor-example)

## Documentation

<https://doc.getqor.com/admin>

## License

Released under the [MIT License](http://opensource.org/licenses/MIT).
