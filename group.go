package admin

import (
	"fmt"
	"strings"

	"github.com/jinzhu/gorm"
	"github.com/theplant/nhk/utils"
)

type Group struct {
	gorm.Model

	// TODO: consider add ignoring roles ? if it is Admin,or developer, always return true.
	Name      string
	Users     string
	AllowList string
}

func (g Group) TableName() string {
	return "qor_groups"
}

// IsAllowed checks if current user allowed to access current resource
func IsAllowed(context *Context) bool {
	uid := context.CurrentUser.GetID()
	resources := allowedResources(context.DB, uid)

	return utils.Contains(resources, context.Resource.Config.Name)
}

// IsMenuAllowed checks if current user allowed to access current menu
func IsMenuAllowed(context *Context, menuName string) bool {
	uid := context.CurrentUser.GetID()
	resources := allowedResources(context.DB, uid)

	return utils.Contains(resources, menuName)
}

func allowedResources(db *gorm.DB, uid uint) (result []string) {
	idStr := fmt.Sprintf("%d", uid)
	groups := []Group{}
	if err := db.Find(&groups).Error; err != nil {
		return
	}

	for _, g := range groups {
		if g.Users != "" && g.AllowList != "" && utils.Contains(strings.Split(g.Users, ","), idStr) {
			result = append(result, strings.Split(g.AllowList, ",")...)
		}
	}

	return
}
