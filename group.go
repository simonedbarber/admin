package admin

import (
	"fmt"
	"strings"

	"github.com/jinzhu/gorm"
	"github.com/theplant/nhk/utils"
)

type Group struct {
	gorm.Model

	Name      string
	Users     string
	AllowList string // "Product, Collection"
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

func allowedResources(db *gorm.DB, uid uint) (result []string) {
	idStr := fmt.Sprintf("%d", uid)
	groups := []Group{}
	if err := db.Find(&groups).Error; err != nil {
		return
	}

	for _, g := range groups {
		if g.Users != "" && g.AllowList != "" && utils.Contains(strings.Split(g.Users, ","), idStr) {
			result = append(result, strings.Split(g.Users, ",")...)
		}
	}

	return
}
