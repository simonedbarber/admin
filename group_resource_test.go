package admin_test

import (
	"testing"

	"github.com/jinzhu/gorm"
	"github.com/qor/admin"
	"github.com/qor/qor/test/utils"
)

func TestGenResourceList(t *testing.T) {
	adm := NewTestAdmin()

	result := admin.GenResourceList(adm)
	if len(result) != 4 {
		t.Error("not get expected resource list count")
	}

	var flag bool
	flag = admin.Contains(result[0], "Product")
	flag = admin.Contains(result[1], "Collection")
	flag = admin.Contains(result[2], "FakeNews")
	flag = admin.Contains(result[3], "External Independent menu")

	if !flag {
		t.Error("lack of expected resource in the result")
	}
}

func NewTestAdmin() *admin.Admin {
	var (
		db     = utils.TestDB()
		models = []interface{}{&Product{}, &Collection{}, &News{}}
		adm    = admin.New(&admin.AdminConfig{DB: db})
	)

	for _, value := range models {
		db.AutoMigrate(value)
	}

	adm.AddResource(&Product{}, &admin.Config{Menu: []string{"Product Management"}})
	adm.AddResource(&Collection{}, &admin.Config{Menu: []string{"Product Management"}})
	adm.AddResource(&News{}, &admin.Config{Name: "FakeNews"})

	adm.AddMenu(&admin.Menu{Name: "External Independent menu", Ancestors: []string{"External Management"}})

	return adm
}

type Product struct {
	gorm.Model
	Name string `gorm:"size:50"`
}

type Collection struct {
	gorm.Model
	Name string `gorm:"size:50"`
}

type News struct {
	gorm.Model
	Name string `gorm:"size:50"`
}
