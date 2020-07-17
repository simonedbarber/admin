package admin_test

import (
	"fmt"
	"net/http"
	"net/url"
	"testing"

	"github.com/qor/admin"
	. "github.com/qor/admin/tests/dummy"
	"github.com/qor/admin/tests/utils"
	"github.com/qor/qor"
	qorTestUtils "github.com/qor/qor/test/utils"
	"github.com/qor/roles"
)

func TestGroupMenuPermission(t *testing.T) {
	qorTestUtils.ResetDBTables(db, &admin.Group{}, &User{})
	user := User{Name: LoggedInUserName, Role: Role_system_administrator}
	utils.AssertNoErr(t, db.Save(&user).Error)

	group := admin.Group{Name: "test group", Users: fmt.Sprintf("%d", user.ID), AllowList: "Company,Credit Card"}
	utils.AssertNoErr(t, db.Save(&group).Error)

	// setup Admin
	ctx := &admin.Context{Context: &qor.Context{CurrentUser: user, DB: Admin.DB}, Admin: Admin, Settings: map[string]interface{}{}}

	companyMenu := Admin.GetMenu("Companies")
	if !companyMenu.HasPermission(roles.Read, ctx) {
		t.Error("user should have permission to access allowed Company resource")
	}

	// check no group permission menu
	group.AllowList = ""
	utils.AssertNoErr(t, db.Save(&group).Error)
	if companyMenu.HasPermission(roles.Read, ctx) {
		t.Error("user should not have permission to access company when it is not allowed")
	}

	individualMenuWithPermission := Admin.AddMenu(&admin.Menu{Name: "ExternalURL", Permission: roles.Allow(roles.CRUD, Role_developer)})
	if individualMenuWithPermission.HasPermission(roles.Read, ctx) {
		t.Error("admin user should not have permission to access menu which is visible to Developer only")
	}
}

func TestIndividualNoPermissionMenu(t *testing.T) {
	qorTestUtils.ResetDBTables(db, &admin.Group{}, &User{})
	user := User{Name: LoggedInUserName, Role: Role_system_administrator}
	utils.AssertNoErr(t, db.Save(&user).Error)

	group := admin.Group{Name: "test group", Users: fmt.Sprintf("%d", user.ID), AllowList: "Company,Credit Card"}
	utils.AssertNoErr(t, db.Save(&group).Error)

	// setup Admin and current role in context
	ctx := &admin.Context{Context: &qor.Context{CurrentUser: user, DB: Admin.DB}, Admin: Admin, Settings: map[string]interface{}{}}
	ctx.Context.Roles = []string{Role_system_administrator}

	// Check no permission menu
	noPermissionMenu := Admin.AddMenu(&admin.Menu{Name: "Dashboard", Link: "/admin"})
	if noPermissionMenu.HasPermission(roles.Read, ctx) {
		t.Error("individual menu with no permission set should not be accessible without group permission")
	}

	Admin.SetGroupEnabled(false)
	if !noPermissionMenu.HasPermission(roles.Read, ctx) {
		t.Error("individual menu with no permission set should be accessible when group permission is not enabled")
	}
	Admin.SetGroupEnabled(true)
}

func TestGroupMenuPermissionShouldHasLowerPriorityThanRole(t *testing.T) {
	qorTestUtils.ResetDBTables(db, &admin.Group{}, &User{})
	user := User{Name: LoggedInUserName, Role: Role_system_administrator}
	utils.AssertNoErr(t, db.Save(&user).Error)

	// setup Admin, group enabled but this user has no group registered
	ctx := &admin.Context{Context: &qor.Context{CurrentUser: user, DB: Admin.DB},
		Admin: Admin, Settings: map[string]interface{}{}}
	ctx.Context.Roles = []string{Role_system_administrator}

	Admin.AddResource(&Profile{}, &admin.Config{Permission: roles.Allow(roles.CRUD, Role_system_administrator)})
	profileMenu := Admin.GetMenu("Profiles")
	if !profileMenu.HasPermission(roles.Read, ctx) {
		t.Error("user should have permission to access roles allowed resource")
	}
}

func TestRouterGroupPermission(t *testing.T) {
	// Allow to access company test
	RouterGroupPermissionTest(t, "Company,Credit Card", 200)
	// Not allowed to access company test
	RouterGroupPermissionTest(t, "Credit Card", 404)
}

func RouterGroupPermissionTest(t *testing.T, allowList string, responseCode int) {
	qorTestUtils.ResetDBTables(db, &admin.Group{}, &User{}, &Company{})
	user := User{Name: LoggedInUserName, Role: Role_system_administrator}
	utils.AssertNoErr(t, db.Save(&user).Error)

	group := admin.Group{Name: "test group", Users: fmt.Sprintf("%d", user.ID), AllowList: allowList}
	utils.AssertNoErr(t, db.Save(&group).Error)

	newCompanyName := "a test company"

	updatedCompanyName := "a new company"
	company := Company{Name: "old company"}
	utils.AssertNoErr(t, db.Save(&company).Error)

	toBeDeletedCompanyName := "a legacy company"
	toBeDeletedCompany := Company{Name: toBeDeletedCompanyName}
	utils.AssertNoErr(t, db.Save(&toBeDeletedCompany).Error)

	cases := []struct {
		desc         string
		url          string
		responseCode int
		formValues   url.Values
	}{
		{"read", server.URL + "/admin/companies", responseCode, nil},
		{"create", server.URL + "/admin/companies", responseCode, url.Values{"QorResource.Name": {newCompanyName}}},
		{"update", server.URL + fmt.Sprintf("/admin/companies/%d", company.ID), responseCode, url.Values{"QorResource.Name": {updatedCompanyName}}},
		{"delete", server.URL + fmt.Sprintf("/admin/companies/%d", toBeDeletedCompany.ID), responseCode, url.Values{"_method": {"delete"}}},
	}
	for _, c := range cases {
		t.Run(c.desc, func(t *testing.T) {
			var (
				resp *http.Response
				err  error
			)

			switch c.desc {
			case "read":
				resp, err = http.Get(c.url)
			case "create", "update", "delete":
				resp, err = http.PostForm(c.url, c.formValues)
			}

			utils.AssertNoErr(t, err)

			if got, want := resp.StatusCode, c.responseCode; want != got {
				t.Errorf("expect user with group permission to have %v when %s companies but got %v", want, c.desc, got)
			}
		})
	}
}

func TestRegisterUserToGroups(t *testing.T) {
	qorTestUtils.ResetDBTables(db, &admin.Group{}, &User{})
	user := User{Name: LoggedInUserName, Role: Role_system_administrator}
	utils.AssertNoErr(t, db.Save(&user).Error)

	groupA := createTestGroup("A")
	groupB := createTestGroup("B")
	groupC := createTestGroup("C")

	err := admin.RegisterUserToGroups(db, []uint{groupA.ID, groupB.ID}, &user.ID)
	if err != nil {
		t.Fatal(err)
	}

	utils.AssertNoErr(t, db.First(groupA, groupA.ID).Error)
	utils.AssertNoErr(t, db.First(groupB, groupB.ID).Error)
	utils.AssertNoErr(t, db.First(groupC, groupC.ID).Error)

	if groupA.Users != fmt.Sprintf(",%d", user.ID) {
		t.Error("user didn't registered in group A", groupA.Users, user.ID)
	}
	if groupB.Users != fmt.Sprintf(",%d", user.ID) {
		t.Error("user didn't registered in group B", groupB.Users, user.ID)
	}
	if groupC.Users != "" {
		t.Error("user incorrectly registered in group C")
	}
}

func TestRegisterUserToGroupsEdgeCases(t *testing.T) {
	qorTestUtils.ResetDBTables(db, &admin.Group{}, &User{})
	user := User{Name: LoggedInUserName, Role: Role_system_administrator}
	utils.AssertNoErr(t, db.Save(&user).Error)

	groupA := createTestGroup("A")

	// Empty group ids
	err := admin.RegisterUserToGroups(db, []uint{}, &user.ID)
	if err == nil {
		t.Error("empty group ids doesn't return error")
	}

	// no user id
	err1 := admin.RegisterUserToGroups(db, []uint{groupA.ID}, nil)
	if err1 == nil {
		t.Error("blank user id doesn't return error")
	}

	// non-exist groups
	err2 := admin.RegisterUserToGroups(db, []uint{1000, 1010}, &user.ID)
	if err2 == nil {
		t.Error("non-exists group ids doesn't return error")
	}
}

func createTestGroup(name string) *admin.Group {
	group := admin.Group{Name: name}
	if err := db.Save(&group).Error; err != nil {
		panic(err)
	}

	return &group
}
