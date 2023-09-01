package admin

import (
	"errors"
	"fmt"
	"net/http"
	"path"
	"reflect"
	"strings"

	"github.com/jinzhu/inflection"
	"github.com/simonedbarber/qor"
	"github.com/simonedbarber/qor/resource"
	"github.com/simonedbarber/qor/utils"
	"github.com/simonedbarber/roles"
	"gorm.io/gorm"
)

// Config resource config struct
type Config struct {
	Name       string
	IconName   string
	Menu       []string
	Permission *roles.Permission
	Themes     []ThemeInterface
	Priority   int
	Singleton  bool
	Invisible  bool
	PageCount  int
}

// Resource is the most important thing for qor admin, every model is defined as a resource, qor admin will genetate management interface based on its definition
type Resource struct {
	*resource.Resource
	Config         *Config
	ParentResource *Resource
	ChildResources []*Resource
	SearchHandler  func(keyword string, context *qor.Context) *gorm.DB

	params   string
	admin    *Admin
	metas    []*Meta
	actions  []*Action
	scopes   []*Scope
	filters  []*Filter
	mounted  bool
	sections struct {
		IndexSections                  []*Section
		OverriddingIndexAttrs          bool
		OverriddingIndexAttrsCallbacks []func()
		NewSections                    []*Section
		OverriddingNewAttrs            bool
		OverriddingNewAttrsCallbacks   []func()
		EditSections                   []*Section
		OverriddingEditAttrs           bool
		OverriddingEditAttrsCallbacks  []func()
		ShowSections                   []*Section
		OverriddingShowAttrs           bool
		ConfiguredShowAttrs            bool
		OverriddingShowAttrsCallbacks  []func()
		SortableAttrs                  *[]string
	}
	layouts struct {
		//IndexLayout //TODO: not implemented
		NewLayout  *Layout
		EditLayout *Layout
		ShowLayout *Layout
	}
}

// GetAdmin get admin from resource
func (res Resource) GetAdmin() *Admin {
	return res.admin
}

// ToParam used as urls to register routes for resource
func (res *Resource) ToParam() string {
	if res.params == "" {
		if value, ok := res.Value.(interface {
			ToParam() string
		}); ok {
			res.params = value.ToParam()
		} else {
			if res.Config.Singleton == true {
				res.params = utils.ToParamString(res.Name)
			} else {
				res.params = utils.ToParamString(inflection.Plural(res.Name))
			}
		}
	}
	return res.params
}

// ParamIDName return param name for primary key like :product_id
func (res Resource) ParamIDName() string {
	return fmt.Sprintf(":%v_id", inflection.Singular(utils.ToParamString(res.Name)))
}

// RoutePrefix return route prefix of resource
func (res *Resource) RoutePrefix() string {
	var params string
	for res.ParentResource != nil {
		params = path.Join(res.ParentResource.ToParam(), res.ParentResource.ParamIDName(), params)
		res = res.ParentResource
	}
	return params
}

// GetPrimaryValue get priamry value from request
func (res Resource) GetPrimaryValue(request *http.Request) string {
	if request != nil {
		return request.URL.Query().Get(res.ParamIDName())
	}
	return ""
}

// UseTheme use them for resource, will auto load the theme's javascripts, stylesheets for this resource
func (res *Resource) UseTheme(theme interface{}) []ThemeInterface {
	var themeInterface ThemeInterface
	if ti, ok := theme.(ThemeInterface); ok {
		themeInterface = ti
	} else if str, ok := theme.(string); ok {
		for _, theme := range res.Config.Themes {
			if theme.GetName() == str {
				return res.Config.Themes
			}
		}

		themeInterface = Theme{Name: str}
	}

	if themeInterface != nil {
		res.Config.Themes = append(res.Config.Themes, themeInterface)

		// Config Admin Theme
		for _, pth := range themeInterface.GetViewPaths() {
			res.GetAdmin().RegisterViewPath(pth)
		}
		themeInterface.ConfigAdminTheme(res)
	}
	return res.Config.Themes
}

// GetTheme get registered theme with name
func (res *Resource) GetTheme(name string) ThemeInterface {
	for _, theme := range res.Config.Themes {
		if theme.GetName() == name {
			return theme
		}
	}
	return nil
}

// NewResource initialize a new qor resource, won't add it to admin, just initialize it
func (res *Resource) NewResource(value interface{}, config ...*Config) *Resource {
	subRes := res.GetAdmin().newResource(value, config...)
	subRes.ParentResource = res
	subRes.configure()
	return subRes
}

// AddSubResource register sub-resource
func (res *Resource) AddSubResource(fieldName string, config ...*Config) (subRes *Resource, err error) {
	var (
		admin = res.GetAdmin()
		scope = utils.NewScope(res.Value)
	)

	relationship := scope.Relationships.Relations[fieldName]
	if field := scope.LookUpField(fieldName); field != nil && relationship != nil {

		modelType := utils.ModelType(reflect.New(field.StructField.Type).Interface())
		subRes = admin.NewResource(reflect.New(modelType).Interface(), config...)
		subRes.setupParentResource(field.StructField.Name, res)

		subRes.Action(&Action{
			Name:   "Delete",
			Method: "DELETE",
			URL: func(record interface{}, context *Context) string {
				return context.URLFor(record, subRes)
			},
			Permission: subRes.Config.Permission,
			Modes:      []string{"menu_item"},
		})

		admin.RegisterResourceRouters(subRes, "create", "update", "read", "delete")
		return
	}

	err = errors.New("invalid sub resource")
	return
}

func (res *Resource) setupParentResource(fieldName string, parent *Resource) {
	res.ParentResource = parent
	parent.ChildResources = append(parent.ChildResources, res)

	var findParent = func(context *qor.Context) (interface{}, error) {
		clone := context.Clone()
		//clone.DB = (*clone.Config.DB).New()
		clone.ResourceID = parent.GetPrimaryValue(context.Request)
		parentValue := parent.NewStruct()
		err := parent.FindOneHandler(parentValue, nil, clone)
		return parentValue, err
	}

	findOneHandler := res.FindOneHandler
	res.FindOneHandler = func(value interface{}, metaValues *resource.MetaValues, context *qor.Context) (err error) {
		if metaValues != nil {
			return findOneHandler(value, metaValues, context)
		}

		if primaryKey := res.GetPrimaryValue(context.Request); primaryKey != "" {
			parentValue := parent.NewStruct()
			if parentValue, err = findParent(context); err == nil {
				primaryQuerySQL, primaryParams := res.ToPrimaryQueryParams(primaryKey, context)
				result := context.GetDB().Model(parentValue).Where(primaryQuerySQL, primaryParams...).Preload(primaryKey).Find(value)
				if result.Error != nil {
					err = result.Error
				}

				scope := utils.NewScope(res.Value)
				if utils.PrimaryKeyZero(scope) && result.RowsAffected == 0 {
					err = gorm.ErrRecordNotFound
				}
			}
		}

		return
	}

	res.FindManyHandler = func(value interface{}, context *qor.Context) (err error) {
		parentValue := parent.NewStruct()
		if parentValue, err = findParent(context); err == nil {
			if _, ok := context.GetDB().Get("qor:getting_total_count"); ok {
				*(value.(*int64)) = context.GetDB().Model(parentValue).Association(fieldName).Count()
				return nil
			}
			return context.GetDB().Model(parentValue).Association(fieldName).Find(value)
		}
		return err
	}

	res.SaveHandler = func(value interface{}, context *qor.Context) (err error) {
		parentValue := parent.NewStruct()
		if parentValue, err = findParent(context); err == nil {
			return context.GetDB().Model(parentValue).Association(fieldName).Append(value)
		}
		return err
	}

	res.DeleteHandler = func(value interface{}, context *qor.Context) (err error) {
		if primaryKey := res.GetPrimaryValue(context.Request); primaryKey != "" {
			primaryQuerySQL, primaryParams := res.ToPrimaryQueryParams(primaryKey, context)
			if err = context.GetDB().Where(primaryQuerySQL, primaryParams...).First(value).Error; err == nil {
				parentValue := parent.NewStruct()
				if parentValue, err = findParent(context); err == nil {
					return context.GetDB().Model(parentValue).Association(fieldName).Delete(value)
				}
			}
		}
		return
	}
}

// Decode decode context into a value
func (res *Resource) Decode(context *qor.Context, value interface{}) error {
	return resource.Decode(context, value, res)
}

func (res *Resource) allAttrs() []string {
	var attrs []string
	scope := utils.NewScope(res.Value)

Fields:
	for _, field := range scope.Fields {
		for _, meta := range res.metas {
			if field.Name == meta.FieldName {
				attrs = append(attrs, meta.Name)
				continue Fields
			}
		}

		//if field.IsForeignKey {
		if _, ok := field.Schema.Relationships.Relations[field.Name]; ok {
			continue
		}

		for _, value := range []string{"CreatedAt", "UpdatedAt", "DeletedAt"} {
			if value == field.Name {
				continue Fields
			}
		}

		if _, ok := field.TagSettings["-"]; (field.FieldType.Kind() != reflect.Struct || field.Schema.Relationships.Relations[field.Name] != nil) && !ok {
			attrs = append(attrs, field.Name)
			continue
		}

		fieldType := field.StructField.Type
		for fieldType.Kind() == reflect.Ptr || fieldType.Kind() == reflect.Slice {
			fieldType = fieldType.Elem()
		}

		if fieldType.Kind() == reflect.Struct {
			attrs = append(attrs, field.Name)
		}
	}

MetaIncluded:
	for _, meta := range res.metas {
		for _, attr := range attrs {
			if attr == meta.FieldName || attr == meta.Name {
				continue MetaIncluded
			}
		}
		attrs = append(attrs, meta.Name)
	}

	return attrs
}

func (res *Resource) getAttrs(attrs []string) []string {
	if len(attrs) == 0 {
		return res.allAttrs()
	}

	var onlyExcludeAttrs = true
	for _, attr := range attrs {
		if !strings.HasPrefix(attr, "-") {
			onlyExcludeAttrs = false
			break
		}
	}

	if onlyExcludeAttrs {
		return append(res.allAttrs(), attrs...)
	}
	return attrs
}

// IndexAttrs set attributes will be shown in the index page
//
//	// show given attributes in the index page
//	order.IndexAttrs("User", "PaymentAmount", "ShippedAt", "CancelledAt", "State", "ShippingAddress")
//	// show all attributes except `State` in the index page
//	order.IndexAttrs("-State")
func (res *Resource) IndexAttrs(values ...interface{}) []*Section {
	overriddingIndexAttrs := res.sections.OverriddingIndexAttrs
	res.sections.OverriddingIndexAttrs = true

	res.setSections(&res.sections.IndexSections, values...)
	res.SearchAttrs()

	// don't call callbacks when overridding
	if !overriddingIndexAttrs {
		for _, callback := range res.sections.OverriddingIndexAttrsCallbacks {
			callback()
		}

		res.sections.OverriddingIndexAttrs = false
	}

	return res.sections.IndexSections
}

// OverrideIndexAttrs register function that will be run everytime index attrs changed
func (res *Resource) OverrideIndexAttrs(fc func()) {
	overriddingIndexAttrs := res.sections.OverriddingIndexAttrs
	res.sections.OverriddingIndexAttrs = true
	res.sections.OverriddingIndexAttrsCallbacks = append(res.sections.OverriddingIndexAttrsCallbacks, fc)
	fc()

	if !overriddingIndexAttrs {
		res.sections.OverriddingIndexAttrs = false
	}
}

// NewAttrs set attributes will be shown in the new page
//
//	   // show given attributes in the new page
//	   order.NewAttrs("User", "PaymentAmount", "ShippedAt", "CancelledAt", "State", "ShippingAddress")
//	   // show all attributes except `State` in the new page
//	   order.NewAttrs("-State")
//	You could also use `Section` to structure form to make it tidy and clean
//	   product.NewAttrs(
//	     &admin.Section{
//	     	Title: "Basic Information",
//	     	Rows: [][]string{
//	     		{"Name"},
//	     		{"Code", "Price"},
//	     	}},
//	     &admin.Section{
//	     	Title: "Organization",
//	     	Rows: [][]string{
//	     		{"Category", "Collections", "MadeCountry"},
//	     	}},
//	     "Description",
//	     "ColorVariations",
//	   }
func (res *Resource) NewAttrs(values ...interface{}) []*Section {
	overriddingNewAttrs := res.sections.OverriddingNewAttrs
	res.sections.OverriddingNewAttrs = true

	res.setSections(&res.sections.NewSections, values...)

	// don't call callbacks when overridding
	if !overriddingNewAttrs {
		for _, callback := range res.sections.OverriddingNewAttrsCallbacks {
			callback()
		}

		res.sections.OverriddingNewAttrs = false
	}

	return res.sections.NewSections
}

func (res *Resource) SetNewLayout(layout ...*Layout) *Layout {
	var primaryLayout Layout
	if len(layout) > 0 {
		res.layouts.NewLayout = layout[0] //TODO: Make this a nullable single varidic function
	}

	if res.layouts.EditLayout != nil {
		return res.layouts.NewLayout
	}

	return &primaryLayout
}

func (res *Resource) NewLayout(sections []*Section) (layout *Layout) {
	if res.layouts.EditLayout != nil {
		layout = res.SetLayoutSections(sections, res.layouts.NewLayout)
	} else {
		layout = &Layout{}

		layout.Type = "layout"
		for i, _ := range sections {
			l := &Layout{}
			l.Section = &LayoutSection{Number: i}
			layout.Layouts = append(layout.Layouts, l)
		}
		layout = res.SetLayoutSections(sections, layout)
	}
	return layout
}

// OverrideNewAttrs register function that will be run everytime new attrs changed
func (res *Resource) OverrideNewAttrs(fc func()) {
	overriddingNewAttrs := res.sections.OverriddingNewAttrs
	res.sections.OverriddingNewAttrs = true
	res.sections.OverriddingNewAttrsCallbacks = append(res.sections.OverriddingNewAttrsCallbacks, fc)
	fc()

	if !overriddingNewAttrs {
		res.sections.OverriddingNewAttrs = false
	}
}

// EditAttrs set attributes will be shown in the edit page
//
//	   // show given attributes in the new page
//	   order.EditAttrs("User", "PaymentAmount", "ShippedAt", "CancelledAt", "State", "ShippingAddress")
//	   // show all attributes except `State` in the edit page
//	   order.EditAttrs("-State")
//	You could also use `Section` to structure form to make it tidy and clean
//	   product.EditAttrs(
//	     &admin.Section{
//	     	Title: "Basic Information",
//	     	Rows: [][]string{
//	     		{"Name"},
//	     		{"Code", "Price"},
//	     	}},
//	     &admin.Section{
//	     	Title: "Organization",
//	     	Rows: [][]string{
//	     		{"Category", "Collections", "MadeCountry"},
//	     	}},
//	     "Description",
//	     "ColorVariations",
//	   }
func (res *Resource) EditAttrs(values ...interface{}) []*Section {
	overriddingEditAttrs := res.sections.OverriddingEditAttrs
	res.sections.OverriddingEditAttrs = true

	res.setSections(&res.sections.EditSections, values...)

	// don't call callbacks when overridding
	if !overriddingEditAttrs {
		for _, callback := range res.sections.OverriddingEditAttrsCallbacks {
			callback()
		}

		res.sections.OverriddingEditAttrs = false
	}

	return res.sections.EditSections
}

func (res *Resource) SetEditLayout(layout ...*Layout) *Layout {
	var primaryLayout Layout
	if len(layout) > 0 {
		res.layouts.EditLayout = layout[0] //TODO: Make this a nullable single varidic function
	}

	if res.layouts.EditLayout != nil {
		return res.layouts.EditLayout
	}

	return &primaryLayout
}

func (res *Resource) EditLayout(sections []*Section) (layout *Layout) {
	if res.layouts.EditLayout != nil {
		layout = res.SetLayoutSections(sections, res.layouts.EditLayout)
	} else {
		layout = &Layout{}

		layout.Type = "layout"
		for i, _ := range sections {
			l := &Layout{}
			l.Section = &LayoutSection{Number: i}
			layout.Layouts = append(layout.Layouts, l)
		}
		layout = res.SetLayoutSections(sections, layout)
	}
	return layout
}

func (res *Resource) SetLayoutSections(sections []*Section, layout *Layout) *Layout {
	var renderLayout Layout
	renderLayout = *layout

	var validLayouts []*Layout
	for _, l := range renderLayout.Layouts {
		// Check that layout Section ID is in range
		if l.Section != nil && l.Section.Number < len(sections) {
			var title string
			if len(l.Title) > 0 {
				title = l.Title
			} else if len(sections[l.Section.Number].Title) > 0 {
				title = sections[l.Section.Number].Title
			}
			validLayouts = append(validLayouts, &Layout{
				Type:  "section_layout",
				Title: title,
				Section: &LayoutSection{
					Section: sections[l.Section.Number],
					Number:  l.Section.Number,
				},
			})
		} else if len(l.Layouts) > 0 {
			sl := res.SetLayoutSections(sections, l)
			if len(sl.Type) == 0 {
				sl.Type = "layout"
			}
			validLayouts = append(validLayouts, sl)
		}
	}
	renderLayout.Layouts = validLayouts

	return &renderLayout
}

// OverrideEditAttrs register function that will be run everytime edit attrs changed
func (res *Resource) OverrideEditAttrs(fc func()) {
	overriddingEditAttrs := res.sections.OverriddingEditAttrs
	res.sections.OverriddingEditAttrs = true
	res.sections.OverriddingEditAttrsCallbacks = append(res.sections.OverriddingEditAttrsCallbacks, fc)
	fc()

	if !overriddingEditAttrs {
		res.sections.OverriddingEditAttrs = false
	}
}

// ShowAttrs set attributes will be shown in the show page
//
//	   // show given attributes in the show page
//	   order.ShowAttrs("User", "PaymentAmount", "ShippedAt", "CancelledAt", "State", "ShippingAddress")
//	   // show all attributes except `State` in the show page
//	   order.ShowAttrs("-State")
//	You could also use `Section` to structure form to make it tidy and clean
//	   product.ShowAttrs(
//	     &admin.Section{
//	     	Title: "Basic Information",
//	     	Rows: [][]string{
//	     		{"Name"},
//	     		{"Code", "Price"},
//	     	}},
//	     &admin.Section{
//	     	Title: "Organization",
//	     	Rows: [][]string{
//	     		{"Category", "Collections", "MadeCountry"},
//	     	}},
//	     "Description",
//	     "ColorVariations",
//	   }
func (res *Resource) ShowAttrs(values ...interface{}) []*Section {
	overriddingShowAttrs := res.sections.OverriddingShowAttrs
	settingShowAttrs := true
	res.sections.OverriddingShowAttrs = true

	if len(values) > 0 {
		if values[len(values)-1] == false {
			settingShowAttrs = false
			values = values[:len(values)-1]
		}
	}

	res.setSections(&res.sections.ShowSections, values...)

	// don't call callbacks when overridding
	if !overriddingShowAttrs {
		if settingShowAttrs && len(values) > 0 {
			res.sections.ConfiguredShowAttrs = true
		}

		for _, callback := range res.sections.OverriddingShowAttrsCallbacks {
			callback()
		}

		res.sections.OverriddingShowAttrs = false
	}

	return res.sections.ShowSections
}

func (res *Resource) SetShowLayout(layout ...*Layout) *Layout {
	var primaryLayout Layout
	if len(layout) > 0 {
		res.layouts.NewLayout = layout[0] //TODO: Make this a nullable single varidic function
	}

	if res.layouts.EditLayout != nil {
		return res.layouts.NewLayout
	}

	return &primaryLayout
}

func (res *Resource) ShowLayout(sections []*Section) (layout *Layout) {
	if res.layouts.EditLayout != nil {
		layout = res.SetLayoutSections(sections, res.layouts.ShowLayout)
	} else {
		layout = &Layout{}

		layout.Type = "layout"
		for i, _ := range sections {
			l := &Layout{}
			l.Section = &LayoutSection{Number: i}
			layout.Layouts = append(layout.Layouts, l)
		}
		layout = res.SetLayoutSections(sections, layout)
	}
	return layout
}

// OverrideShowAttrs register function that will be run everytime show attrs changed
func (res *Resource) OverrideShowAttrs(fc func()) {
	overriddingShowAttrs := res.sections.OverriddingShowAttrs
	res.sections.OverriddingShowAttrs = true
	res.sections.OverriddingShowAttrsCallbacks = append(res.sections.OverriddingShowAttrsCallbacks, fc)
	fc()

	if !overriddingShowAttrs {
		res.sections.OverriddingShowAttrs = false
	}
}

// SortableAttrs set sortable attributes, sortable attributes are clickable to sort data in index page
func (res *Resource) SortableAttrs(columns ...string) []string {
	if len(columns) != 0 || res.sections.SortableAttrs == nil {
		if len(columns) == 0 {
			columns = res.ConvertSectionToStrings(res.sections.IndexSections)
		}
		res.sections.SortableAttrs = &[]string{}
		scope := utils.NewScope(res.Value)
		for _, column := range columns {
			if field := scope.LookUpField(column); field != nil && field.DBName != "" {
				attrs := append(*res.sections.SortableAttrs, column)
				res.sections.SortableAttrs = &attrs
			}
		}
	}
	return *res.sections.SortableAttrs
}

// SearchAttrs set searchable attributes, e.g:
//
//		   product.SearchAttrs("Name", "Code", "Category.Name", "Brand.Name")
//	    // Search products with its name, code, category's name, brand's name
func (res *Resource) SearchAttrs(columns ...string) []string {
	if len(columns) != 0 || res.SearchHandler == nil {
		if len(columns) == 0 {
			columns = res.ConvertSectionToStrings(res.sections.IndexSections)
		}

		if len(columns) > 0 {
			res.SearchHandler = func(keyword string, context *qor.Context) *gorm.DB {
				var filterFields []filterField
				for _, column := range columns {
					filterFields = append(filterFields, filterField{FieldName: column})
				}
				return filterResourceByFields(res, filterFields, keyword, context.GetDB(), context)
			}
		}
	}

	return columns
}

// Meta register meta for admin resource
func (res *Resource) Meta(meta *Meta) *Meta {
	if oldMeta := res.GetMeta(meta.Name); oldMeta != nil {
		if meta.Type != "" {
			oldMeta.Type = meta.Type
			oldMeta.Config = nil
		}

		if meta.Label != "" {
			oldMeta.Label = meta.Label
		}

		if meta.FieldName != "" {
			oldMeta.FieldName = meta.FieldName
		}

		if meta.Setter != nil {
			oldMeta.Setter = meta.Setter
		}

		if meta.Valuer != nil {
			oldMeta.Valuer = meta.Valuer
		}

		if meta.FormattedValuer != nil {
			oldMeta.FormattedValuer = meta.FormattedValuer
		}

		if meta.Resource != nil {
			oldMeta.Resource = meta.Resource
		}

		if meta.Permission != nil {
			oldMeta.Permission = meta.Permission
		}

		if meta.Config != nil {
			oldMeta.Config = meta.Config
		}

		if meta.Collection != nil {
			oldMeta.Collection = meta.Collection
		}
		meta = oldMeta
	} else {
		res.metas = append(res.metas, meta)
		meta.baseResource = res
	}

	meta.configure()
	return meta
}

// GetMetas get metas with give attrs
func (res *Resource) GetMetas(attrs []string) []resource.Metaor {
	if len(attrs) == 0 {
		attrs = res.allAttrs()
	}
	var showSections, ignoredAttrs []string
	for _, attr := range attrs {
		if strings.HasPrefix(attr, "-") {
			ignoredAttrs = append(ignoredAttrs, strings.TrimLeft(attr, "-"))
		} else {
			showSections = append(showSections, attr)
		}
	}

	metas := []resource.Metaor{}

Attrs:
	for _, attr := range showSections {
		for _, a := range ignoredAttrs {
			if attr == a {
				continue Attrs
			}
		}

		var meta *Meta
		for _, m := range res.metas {
			if m.GetName() == attr {
				meta = m
				break
			}
		}

		if meta == nil {
			meta = &Meta{Name: attr, baseResource: res}
			for _, primaryField := range res.PrimaryFields {
				if attr == primaryField.Name {
					meta.Type = "hidden_primary_key"
					break
				}
			}
			meta.configure()
		}

		metas = append(metas, meta)
	}

	return metas
}

// GetMeta get meta with name
func (res *Resource) GetMeta(name string) *Meta {
	var fallbackMeta *Meta

	for _, meta := range res.metas {
		if meta.Name == name {
			return meta
		}

		if meta.GetFieldName() == name {
			fallbackMeta = meta
		}
	}

	if fallbackMeta == nil {
		scope := utils.NewScope(res.Value)
		if field := scope.LookUpField(name); field != nil {
			meta := &Meta{Name: name, baseResource: res}
			if field.PrimaryKey {
				meta.Type = "hidden_primary_key"
			}
			meta.configure()
			res.metas = append(res.metas, meta)
			return meta
		}
	}

	return fallbackMeta
}

func (res *Resource) allowedSections(sections []*Section, context *Context, roles ...roles.PermissionMode) []*Section {
	var newSections []*Section
	for _, section := range sections {
		newSection := Section{Resource: section.Resource, Title: section.Title}
		var editableRows [][]string
		for _, row := range section.Rows {
			var editableColumns []string
			for _, column := range row {
				for _, role := range roles {
					meta := res.GetMeta(column)
					if meta != nil && meta.HasPermission(role, context.Context) {
						editableColumns = append(editableColumns, column)
						break
					}
				}
			}
			if len(editableColumns) > 0 {
				editableRows = append(editableRows, editableColumns)
			}
		}
		newSection.Rows = editableRows
		newSections = append(newSections, &newSection)
	}
	return newSections
}

func (res *Resource) configure() {
	var configureModel func(value interface{})

	configureModel = func(value interface{}) {
		modelType := utils.ModelType(value)
		if modelType.Kind() == reflect.Struct {
			for i := 0; i < modelType.NumField(); i++ {
				if fieldStruct := modelType.Field(i); fieldStruct.Anonymous {
					if injector, ok := reflect.New(fieldStruct.Type).Interface().(resource.ConfigureResourceInterface); ok {
						injector.ConfigureQorResource(res)
					} else {
						configureModel(reflect.New(fieldStruct.Type).Interface())
					}
				}
			}
		}
	}

	configureModel(res.Value)

	scope := utils.NewScope(res.Value)

	for _, field := range scope.Fields {
		if field.StructField.Type.Kind() == reflect.Struct {
			fieldData := reflect.New(field.StructField.Type).Interface()
			_, configureMetaBeforeInitialize := fieldData.(resource.ConfigureMetaBeforeInitializeInterface)
			_, configureMeta := fieldData.(resource.ConfigureMetaInterface)

			if configureMetaBeforeInitialize || configureMeta {
				res.Meta(&Meta{Name: field.Name})
			}
		}
	}

	if injector, ok := res.Value.(resource.ConfigureResourceInterface); ok {
		injector.ConfigureQorResource(res)
	}
}
