package admin

import (
	"fmt"
	"strings"

	"github.com/jinzhu/gorm"
	"github.com/jinzhu/inflection"
	"github.com/qor/qor"
	"github.com/qor/qor/resource"
	"github.com/qor/qor/utils"
	"github.com/qor/roles"
	"github.com/qor/validations"
)

type UserModel interface {
	GetUsersByIDs(*gorm.DB, []string) interface{}
}

func RegisterGroup(adm *Admin, resourceList []string, userSelectRes *Resource, userModel UserModel, resConfig *Config) *Resource {
	ValidateResourceList(adm, resourceList)

	adm.DB.AutoMigrate(&Group{})

	adm.SetGroupEnabled(true)

	if resConfig.Name == "" {
		resConfig.Name = "Groups"
	}

	group := adm.AddResource(&Group{}, resConfig)

	group.IndexAttrs("ID", "Name", "CreatedAt", "UpdatedAt")
	group.NewAttrs("Name",
		&Section{
			Title: "Resource Permission",
			Rows: [][]string{
				{"AllowList"},
			}},
		&Section{
			Title: "Select people to this group",
			Rows: [][]string{
				{"Users"},
			}})
	group.EditAttrs("Name",
		&Section{
			Title: "Resource Permission",
			Rows: [][]string{
				{"AllowList"},
			}},
		&Section{
			Title: "people in this group",
			Rows: [][]string{
				{"Users"},
			}})

	group.Meta(&Meta{
		Name: "Users", Label: "",
		Config: &SelectManyConfig{
			RemoteDataResource: userSelectRes,
		},
		Setter: func(record interface{}, metaValue *resource.MetaValue, context *qor.Context) {
			if g, ok := record.(*Group); ok {
				primaryKeys := utils.ToArray(metaValue.Value)
				g.Users = strings.Join(primaryKeys, ",")
			}
		},
		Valuer: func(record interface{}, context *qor.Context) interface{} {
			if g, ok := record.(*Group); ok {
				ids := strings.Split(g.Users, ",")

				return userModel.GetUsersByIDs(context.GetDB(), ids)
			}

			return nil
		},
	})

	group.Meta(&Meta{
		Name: "AllowList",
		Config: &SelectManyConfig{
			Collection: resourceList, // TODO: validate resourceList are included in Resource. which means group needs to be registered by the end of the admin registration
		},
		Setter: func(record interface{}, metaValue *resource.MetaValue, context *qor.Context) {
			if g, ok := record.(*Group); ok {
				allowedResources := utils.ToArray(metaValue.Value)
				g.AllowList = strings.Join(allowedResources, ",")
			}
		},
		Valuer: func(record interface{}, context *qor.Context) interface{} {
			if g, ok := record.(*Group); ok {
				allowedResources := strings.Split(g.AllowList, ",")

				return allowedResources
			}

			return nil
		},
	})
	group.Meta(&Meta{Name: "Name", Label: "Group Name"})

	group.AddValidator(&resource.Validator{
		Handler: func(value interface{}, metaValues *resource.MetaValues, ctx *qor.Context) error {
			if meta := metaValues.Get("Name"); meta != nil {
				if name := utils.ToString(meta.Value); strings.TrimSpace(name) == "" {
					return validations.NewError(value, "Group Name", "Group Name can't be blank")
				}
			}
			return nil
		},
	})

	return initGroupSelectorRes(adm)
	// router := adm.GetRouter()
	// router.Delete("/groups/:id/delete", deleteGroup)
}

// func deleteGroup(ctx *admin.Context) {
// 	id := ctx.Request.URL.Query().Get(":id")
// 	var group Group
// 	ctx.DB.New().Model(ctx.Resource.NewStruct()).Where("id = ?", id).Find(&group)

// 	status := http.StatusOK
// 	var err error

// 	if len(group.GetUserIDs()) > 0 {
// 		err = errors.New("cannot delete non-empty group")
// 		ctx.AddError(err)
// 		status = http.StatusUnprocessableEntity
// 	} else {
// 		if err = ctx.DB.New().Delete(&group).Error; err != nil {
// 			err = errors.New("delete error")
// 			ctx.AddError(err)
// 			status = http.StatusUnprocessableEntity
// 		}
// 	}
// 	ctx.Writer.WriteHeader(status)
// 	if err != nil {
// 		ctx.Encode("OK", map[string]interface{}{"errors": err.Error()})
// 	} else {
// 		ctx.Encode("OK", map[string]interface{}{"status": "ok"})
// 	}
// }

func initGroupSelectorRes(adm *Admin) *Resource {
	res := adm.AddResource(&Group{}, &Config{Name: "GroupSelector"})
	res.SearchAttrs("ID", "Name")
	adm.GetMenu("GroupSelectors").Permission = roles.Deny(roles.CRUD, roles.Anyone)
	return res
}

// ValidateResourceList validates resources or menu name passed in are registered in admin.
// It will panic immediately if not.
func ValidateResourceList(adm *Admin, resourceList []string) {
	availableResourcesName := []string{}
	for _, r := range adm.GetResources() {
		availableResourcesName = append(availableResourcesName, r.Name)
	}

	for _, m := range adm.GetMenus() {
		if !Contains(availableResourcesName, m.Name) && !Contains(availableResourcesName, inflection.Singular(m.Name)) {
			availableResourcesName = append(availableResourcesName, m.Name)
		}
	}

	for _, resName := range resourceList {
		if !Contains(availableResourcesName, resName) {
			panic(fmt.Sprintf("given resource '%s' cannot be found. Available names are '%q'. Make sure RegisterGroup is executed AFTER all the resource and menu get registered", resName, availableResourcesName))
		}
	}
}
