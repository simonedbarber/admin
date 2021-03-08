package admin_test

import (
	"testing"

	"github.com/qor/admin"
	. "github.com/qor/admin/tests/dummy"
	qorTestUtils "github.com/qor/qor/test/utils"
)

func TestDefaultFilter(t *testing.T) {
	//initData()
	user := Admin.GetResource("User")
	user.Filter(&admin.Filter{
		Name: "ID",
	})
	//filter bool
	user.Filter(&admin.Filter{
		Name: "Active",
	})
	//filter intiger
	user.Filter(&admin.Filter{
		Name: "Age",
		Config: &admin.SelectOneConfig{
			Collection: []string{"10", "11", "12", "13", "14", "15"},
		},
	})
	//filter string
	user.Filter(&admin.Filter{
		Name: "Role",
		Config: &admin.SelectOneConfig{
			Collection: []string{Role_editor, Role_supervisor, Role_system_administrator, Role_developer},
		},
	})
	//filter belongs_to
	user.Filter(&admin.Filter{
		Name:   "CreditCard",
		Config: &admin.SelectOneConfig{},
	})
	//filter has_many
	user.Filter(&admin.Filter{
		Name:   "Addresses",
		Config: &admin.SelectOneConfig{},
	})
	//filter belongs_to
	user.Filter(&admin.Filter{
		Name:   "Company",
		Config: &admin.SelectOneConfig{},
	})
	//filter many_to_many
	user.Filter(&admin.Filter{
		Name:   "Languages",
		Config: &admin.SelectOneConfig{},
	})

}

func TestSelectManyConfigFilter(t *testing.T) {
	//initData()
	userM := Admin.NewResource(&User{}, &admin.Config{Name: "SelectManyFilter"})
	userM.Filter(&admin.Filter{
		Name: "Age",
		Config: &admin.SelectManyConfig{
			Collection: []string{"10", "11", "12", "13", "14", "15"},
		},
	})

	userM.Filter(&admin.Filter{
		Name: "Role",
		Config: &admin.SelectManyConfig{
			Collection: []string{Role_editor, Role_supervisor, Role_system_administrator, Role_developer},
		},
	})
	//filter belongs_to
	userM.Filter(&admin.Filter{
		Name:   "CreditCard",
		Config: &admin.SelectManyConfig{},
	})
	//filter has_many
	userM.Filter(&admin.Filter{
		Name:   "Addresses",
		Config: &admin.SelectManyConfig{},
	})
	//filter belongs_to
	userM.Filter(&admin.Filter{
		Name:   "Company",
		Config: &admin.SelectManyConfig{},
	})
	//filter many_to_many
	userM.Filter(&admin.Filter{
		Name:   "Languages",
		Config: &admin.SelectManyConfig{},
	})

}

func initData() {
	db.DropTableIfExists("user_languages")
	qorTestUtils.ResetDBTables(db, &Language{}, &User{}, &Company{}, &CreditCard{})
	coms := []Company{{Name: "Company A"}, {Name: "Company B"}}
	db.Save(&coms[0])
	db.Save(&coms[1])

	lans := []Language{{Name: "en-gb"}, {Name: "en-us"}, {Name: "cn-zh"}, {Name: "ja-jp"}}
	db.Save(&lans[0])
	db.Save(&lans[1])
	db.Save(&lans[2])
	db.Save(&lans[3])

	cards := []CreditCard{{Number: "1111111111111"}, {Number: "1211111111111"}, {Number: "1311111111111"}, {Number: "1411111111111"}, {Number: "1511111111111"}, {Number: "1611111111111"}}
	db.Save(&cards[0])
	db.Save(&cards[1])
	db.Save(&cards[2])
	db.Save(&cards[3])

	roles := []string{Role_editor, Role_supervisor, Role_system_administrator, Role_developer}
	db.Save(&User{Name: "user_0", Age: 10, Role: roles[0], Active: true, CreditCard: cards[0], Company: &coms[0], Languages: []Language{lans[0], lans[1]}})
	db.Save(&User{Name: "user_1", Age: 11, Role: roles[0], Active: true, CreditCard: cards[1], Company: &coms[0], Languages: []Language{lans[0], lans[2]}})
	db.Save(&User{Name: "user_2", Age: 12, Role: roles[1], Active: true, CreditCard: cards[2], Company: &coms[0], Languages: []Language{lans[0], lans[3]}})
	db.Save(&User{Name: "user_3", Age: 13, Role: roles[1], Active: false, CreditCard: cards[3], Company: &coms[1], Languages: []Language{lans[1]}})
	db.Save(&User{Name: "user_4", Age: 14, Role: roles[2], Active: false, CreditCard: cards[4], Company: &coms[1], Languages: []Language{lans[1], lans[2]}})
	db.Save(&User{Name: "user_5", Age: 15, Role: roles[3], Active: false, CreditCard: cards[5], Company: &coms[1], Languages: []Language{lans[1], lans[3]}})
}
