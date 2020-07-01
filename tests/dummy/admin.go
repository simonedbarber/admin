package dummy

import (
	"fmt"

	"github.com/qor/admin"
	"github.com/qor/media"
	"github.com/qor/qor"
	"github.com/qor/qor/test/utils"
)

// NewDummyAdmin generate admin for dummy app
func NewDummyAdmin(keepData ...bool) *admin.Admin {
	var (
		db     = utils.TestDB()
		models = []interface{}{&User{}, &CreditCard{}, &Address{}, &Language{}, &Profile{}, &Phone{}, &Company{}}
		Admin  = admin.New(&admin.AdminConfig{Auth: DummyAuth{}, DB: db})
	)

	media.RegisterCallbacks(db)

	for _, value := range models {
		if len(keepData) == 0 {
			db.DropTableIfExists(value)
		}
		db.AutoMigrate(value)
	}

	Admin.AddResource(&Company{})
	Admin.AddResource(&CreditCard{})

	Admin.AddResource(&Language{}, &admin.Config{Name: "语种 & 语言", Priority: -1})
	user := Admin.AddResource(&User{})
	user.Meta(&admin.Meta{
		Name: "CreditCard",
		Type: "single_edit",
	})
	user.Meta(&admin.Meta{
		Name: "Languages",
		Type: "select_many",
		Collection: func(resource interface{}, context *qor.Context) (results [][]string) {
			if languages := []Language{}; !context.GetDB().Find(&languages).RecordNotFound() {
				for _, language := range languages {
					results = append(results, []string{fmt.Sprint(language.ID), language.Name})
				}
			}
			return
		},
	})

	admin.RegisterGroup(Admin, []string{"Company", "CreditCard", "Address"}, user, User{})

	return Admin
}

const LoggedInUserName = "QOR"

type DummyAuth struct {
}

func (DummyAuth) LoginURL(ctx *admin.Context) string {
	return "/auth/login"
}

func (DummyAuth) LogoutURL(ctx *admin.Context) string {
	return "/auth/logout"
}

func (DummyAuth) GetCurrentUser(ctx *admin.Context) qor.CurrentUser {
	u := User{}

	if err := ctx.Admin.DB.Where("name = ?", LoggedInUserName).First(&u).Error; err != nil {
		fmt.Println("Cannot load logged in user", err.Error())
		return nil
	}

	return u
}
