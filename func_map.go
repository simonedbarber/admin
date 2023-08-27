package admin

import (
	"bytes"
	"database/sql/driver"
	"encoding/json"
	"fmt"
	"html"
	"math/rand"
	"net/url"
	"path"
	"path/filepath"
	"reflect"
	"regexp"
	"runtime/debug"
	"sort"
	"strconv"
	"strings"

	"github.com/simonedbarber/go-template/html/template"

	sprig "github.com/go-task/slim-sprig"
	"github.com/jinzhu/inflection"
	"github.com/simonedbarber/qor"
	"github.com/simonedbarber/qor/resource"
	"github.com/simonedbarber/qor/utils"
	"github.com/simonedbarber/roles"
	"github.com/simonedbarber/session"
)

// FuncMap funcs map for current context
func (context *Context) FuncMap() template.FuncMap {
	funcMap := template.FuncMap{
		"current_user":         func() qor.CurrentUser { return context.CurrentUser },
		"get_resource":         context.Admin.GetResource,
		"new_resource_context": context.NewResourceContext,
		"is_new_record":        context.isNewRecord,
		"is_equal":             context.isEqual,
		"is_included":          context.isIncluded,
		"primary_key_of":       context.primaryKeyOf,
		"unique_key_of":        context.uniqueKeyOf,
		"formatted_value_of":   context.FormattedValueOf,
		"raw_value_of":         context.RawValueOf,

		"controller_action": context.controllerAction,

		"t": context.t,
		"flashes": func() []session.Message {
			return context.Admin.SessionManager.Flashes(context.Writer, context.Request)
		},
		"pagination": context.Pagination,
		"escape":     html.EscapeString,
		"raw":        func(str string) template.HTML { return template.HTML(utils.HTMLSanitizer.Sanitize(str)) },
		"unsafe_raw": func(str string) template.HTML { return template.HTML(str) },
		"equal":      equal,
		"stringify":  utils.Stringify,
		"lower": func(value interface{}) string {
			return strings.ToLower(fmt.Sprint(value))
		},
		"plural": func(value interface{}) string {
			return inflection.Plural(fmt.Sprint(value))
		},
		"singular": func(value interface{}) string {
			return inflection.Singular(fmt.Sprint(value))
		},
		"get_icon": func(m *menu) string {
			if m.IconName != "" {
				return m.IconName
			}
			return m.Name
		},
		"marshal": func(v interface{}) template.JS {
			switch value := v.(type) {
			case string:
				return template.JS(value)
			case template.HTML:
				return template.JS(value)
			default:
				byt, _ := json.Marshal(v)
				return template.JS(byt)
			}
		},

		"to_map": func(values ...interface{}) map[string]interface{} {
			results := map[string]interface{}{}
			for i := 0; i < len(values)-1; i += 2 {
				results[fmt.Sprint(values[i])] = values[i+1]
			}
			return results
		},
		"render":             context.Render,
		"render_text":        context.renderText,
		"render_with":        context.renderWith,
		"render_form":        context.renderForm,
		"render_form_layout": context.renderFormLayout,
		"render_meta": func(value interface{}, meta *Meta, types ...string) template.HTML {
			var (
				result = bytes.NewBufferString("")
				typ    = "index"
			)

			for _, t := range types {
				typ = t
			}

			context.renderMeta(meta, value, []string{}, typ, result)
			return template.HTML(result.String())
		},
		"render_filter": context.renderFilter,
		"saved_filters": context.savedFilters,
		"has_filter": func() bool {
			query := context.Request.URL.Query()
			for key := range query {
				if regexp.MustCompile("filter[(\\w+)]").MatchString(key) && query.Get(key) != "" {
					return true
				}
			}
			return false
		},
		"page_title": context.pageTitle,
		"meta_label": func(meta *Meta) template.HTML {
			key := fmt.Sprintf("%v.attributes.%v", meta.baseResource.ToParam(), meta.Label)
			return context.Admin.T(context.Context, key, meta.Label)
		},
		"meta_placeholder": func(meta *Meta, context *Context, placeholder string) template.HTML {
			if getPlaceholder, ok := meta.Config.(interface {
				GetPlaceholder(*Context) (template.HTML, bool)
			}); ok {
				if str, ok := getPlaceholder.GetPlaceholder(context); ok {
					return str
				}
			}

			key := fmt.Sprintf("%v.attributes.%v.placeholder", meta.baseResource.ToParam(), meta.Label)
			return context.Admin.T(context.Context, key, placeholder)
		},

		"url_for":            context.URLFor,
		"link_to":            context.linkTo,
		"patch_current_url":  context.patchCurrentURL,
		"patch_url":          context.patchURL,
		"join_current_url":   context.joinCurrentURL,
		"join_url":           context.joinURL,
		"logout_url":         context.logoutURL,
		"search_center_path": func() string { return path.Join(context.Admin.router.Prefix, "!search") },
		"new_resource_path":  context.newResourcePath,
		"defined_resource_show_page": func(res *Resource) bool {
			if res != nil {
				if r := context.Admin.GetResource(utils.ModelType(res.Value).String()); r != nil {
					return r.sections.ConfiguredShowAttrs
				}
			}

			return false
		},

		"get_menus":            context.getMenus,
		"get_scopes":           context.getScopes,
		"get_filters":          context.getFilters,
		"get_formatted_errors": context.getFormattedErrors,
		"load_actions":         context.loadActions,
		"allowed_actions":      context.AllowedActions,
		"is_sortable_meta":     context.isSortableMeta,
		"index_sections":       context.indexSections,

		"show_sections": context.showSections,
		"show_layout":   context.showLayout,

		"new_sections": context.newSections,
		"new_layout":   context.newLayout,

		"edit_sections": context.editSections,
		"edit_layout":   context.editLayout,

		"convert_sections_to_metas": context.convertSectionToMetas,

		"has_create_permission": context.hasCreatePermission,
		"has_read_permission":   context.hasReadPermission,
		"has_update_permission": context.hasUpdatePermission,
		"has_delete_permission": context.hasDeletePermission,
		"has_change_permission": context.hasChangePermission,

		"qor_theme_class":        context.themesClass,
		"javascript_tag":         context.javaScriptTag,
		"javascript_tag_defer":   context.javaScriptTagDefer,
		"stylesheet_tag":         context.styleSheetTag,
		"load_theme_stylesheets": context.loadThemeStyleSheets,
		"load_theme_javascripts": context.loadThemeJavaScripts,
		"load_admin_stylesheets": context.loadAdminStyleSheets,
		"load_admin_javascripts": context.loadAdminJavaScripts,

		"resource_name": context.resourceName,
		"edit_layout_mode": func() bool {
			v := context.Request.URL.Query()
			return len(v["edit_layout"]) > 0
		},

		"environment": func() string {
			return context.Request.Context().Value("ENV").(string)
		},
		"breadcrumbs":       context.breadcrumbs,
		"filter_by":         context.filterBy,
		"is_kanban":         context.isKanban,
		"get_columns":       context.getColumns,
		"get_new_resources": context.getNewResources,
		"new_relic":         context.newRelic,
	}

	for key, value := range context.Admin.funcMaps {
		funcMap[key] = value
	}

	for key, value := range context.funcMaps {
		funcMap[key] = value
	}
	// Add sprig additional templating options
	for k, v := range sprig.FuncMap() {
		funcMap["sprig_"+k] = v
	}

	return funcMap
}

// NewResourceContext new context with resource
func (context *Context) NewResourceContext(name ...interface{}) *Context {
	clone := &Context{Context: context.Context.Clone(), Admin: context.Admin, Result: context.Result, Action: context.Action}
	if len(name) > 0 {
		if str, ok := name[0].(string); ok {
			clone.setResource(context.Admin.GetResource(str))
		} else if res, ok := name[0].(*Resource); ok {
			clone.setResource(res)
		}
	} else {
		clone.setResource(context.Resource)
	}
	return clone
}

// URLFor generate url for resource value
//
//	context.URLFor(&Product{})
//	context.URLFor(&Product{ID: 111})
//	context.URLFor(productResource)
func (context *Context) URLFor(value interface{}, resources ...*Resource) string {
	getPrefix := func(res *Resource) string {
		var params string
		for res.ParentResource != nil {
			params = path.Join(res.ParentResource.ToParam(), res.ParentResource.GetPrimaryValue(context.Request), params)
			res = res.ParentResource
		}
		return path.Join(res.GetAdmin().router.Prefix, params)
	}

	if admin, ok := value.(*Admin); ok {
		return admin.router.Prefix
	} else if res, ok := value.(*Resource); ok {
		return path.Join(getPrefix(res), res.ToParam())
	} else {
		var res *Resource

		if len(resources) > 0 {
			res = resources[0]
		}

		if res == nil {
			res = context.Admin.GetResource(reflect.Indirect(reflect.ValueOf(value)).Type().String())
		}

		if res != nil {
			if res.Config.Singleton {
				return path.Join(getPrefix(res), res.ToParam())
			}

			var (
				scope         = utils.NewScope(value)
				primaryFields []string
				primaryValues = map[string]string{}
			)

			for _, primaryField := range res.PrimaryFields {
				if field := scope.LookUpField(primaryField.Name); field != nil {
					rf := reflect.Indirect(reflect.ValueOf(value))
					primaryFields = append(primaryFields, url.PathEscape(fmt.Sprint(rf.FieldByName(field.Name).Interface())))
				}
			}

			for _, field := range scope.PrimaryFields {
				useAsPrimaryField := false
				for _, primaryField := range res.PrimaryFields {
					if field.DBName == primaryField.DBName {
						useAsPrimaryField = true
						break
					}
				}

				if !useAsPrimaryField {
					primaryValues[fmt.Sprintf("primary_key[%v_%v]", scope.Table, field.DBName)] = fmt.Sprint(reflect.Indirect(reflect.ValueOf(value)).FieldByName(field.Name).Interface())
				}
			}

			urlPath := path.Join(getPrefix(res), res.ToParam(), strings.Join(primaryFields, ","))

			if len(primaryValues) > 0 {
				var primaryValueParams []string
				for key, value := range primaryValues {
					primaryValueParams = append(primaryValueParams, fmt.Sprintf("%v=%v", key, url.QueryEscape(value)))
				}
				urlPath = urlPath + "?" + strings.Join(primaryValueParams, "&")
			}
			return urlPath
		}
	}
	return ""
}

// RawValueOf return raw value of a meta for current resource
func (context *Context) RawValueOf(value interface{}, meta *Meta) interface{} {
	return context.valueOf(meta.GetValuer(), value, meta)
}

// FormattedValueOf return formatted value of a meta for current resource
func (context *Context) FormattedValueOf(value interface{}, meta *Meta) interface{} {
	result := context.valueOf(meta.GetFormattedValuer(), value, meta)
	if resultValuer, ok := result.(driver.Valuer); ok {
		if result, err := resultValuer.Value(); err == nil {
			return result
		}
	}

	return result
}

var visiblePageCount = 8

// Page contain pagination information
type Page struct {
	Page       int
	Current    bool
	IsPrevious bool
	IsNext     bool
	IsFirst    bool
	IsLast     bool
}

// PaginationResult pagination result struct
type PaginationResult struct {
	Pagination Pagination
	Pages      []Page
}

// Pagination return pagination information
// Keep visiblePageCount's pages visible, exclude prev and next link
// Assume there are 12 pages in total.
// When current page is 1
// [current, 2, 3, 4, 5, 6, 7, 8, next]
// When current page is 6
// [prev, 2, 3, 4, 5, current, 7, 8, 9, 10, next]
// When current page is 10
// [prev, 5, 6, 7, 8, 9, current, 11, 12]
// If total page count less than VISIBLE_PAGE_COUNT, always show all pages
func (context *Context) Pagination() *PaginationResult {
	var (
		pages      []Page
		pagination = context.Searcher.Pagination
		pageCount  = pagination.PerPage
	)

	if pageCount == 0 {
		if context.Resource != nil && context.Resource.Config.PageCount != 0 {
			pageCount = context.Resource.Config.PageCount
		} else {
			pageCount = PaginationPageCount
		}
	}

	if pagination.Total <= pageCount && pagination.CurrentPage <= 1 {
		return &PaginationResult{Pagination: pagination, Pages: pages}
	}

	start := pagination.CurrentPage - visiblePageCount/2
	if start < 1 {
		start = 1
	}

	end := start + visiblePageCount - 1 // -1 for "start page" itself
	if end > pagination.Pages {
		end = pagination.Pages
	}

	if (end-start) < visiblePageCount && start != 1 {
		start = end - visiblePageCount + 1
	}
	if start < 1 {
		start = 1
	}

	// Append prev link
	if start > 1 {
		pages = append(pages, Page{Page: 1, IsFirst: true})
		pages = append(pages, Page{Page: pagination.CurrentPage - 1, IsPrevious: true})
	}

	for i := start; i <= end; i++ {
		pages = append(pages, Page{Page: i, Current: pagination.CurrentPage == i})
	}

	// Append next link
	if end < pagination.Pages {
		pages = append(pages, Page{Page: pagination.CurrentPage + 1, IsNext: true})
		pages = append(pages, Page{Page: pagination.Pages, IsLast: true})
	}

	return &PaginationResult{Pagination: pagination, Pages: pages}
}

func (context *Context) primaryKeyOf(value interface{}) interface{} {
	if reflect.Indirect(reflect.ValueOf(value)).Kind() == reflect.Struct {
		obj := reflect.Indirect(reflect.ValueOf(value))

		for i := 0; i < obj.Type().NumField(); i++ {
			// If given struct has CompositePrimaryKey field and it is not nil. return it as the primary key.
			if obj.Type().Field(i).Name == resource.CompositePrimaryKeyFieldName && obj.Field(i).FieldByName("CompositePrimaryKey").String() != "" {
				return obj.Field(i).FieldByName("CompositePrimaryKey")
			}
		}

		scope := utils.NewScope(value)
		if len(scope.PrimaryFields) > 0 {
			if len(scope.PrimaryFields) > 1 {
				return fmt.Sprint(obj.FieldByName("ID").Interface())
			}
			return fmt.Sprint(obj.FieldByName(scope.PrimaryFields[0].Name).Interface())
		}
		return nil
	}
	return fmt.Sprint(value)
}

func (context *Context) uniqueKeyOf(value interface{}) interface{} {
	if valueIndirect := reflect.Indirect(reflect.ValueOf(value)); valueIndirect.Kind() == reflect.Struct {
		scope := utils.NewScope(value)
		var primaryValues []string

		for _, primaryField := range scope.PrimaryFields {
			rvField := valueIndirect.FieldByName(primaryField.Name)
			primaryValues = append(primaryValues, fmt.Sprint(rvField.Interface()))
		}
		primaryValues = append(primaryValues, fmt.Sprint(rand.Intn(1000)))
		return utils.ToParamString(url.QueryEscape(strings.Join(primaryValues, "_")))
	}
	return fmt.Sprint(value)
}

func (context *Context) isNewRecord(value interface{}) bool {
	if value == nil {
		return true
	}
	return reflect.ValueOf(value).IsZero()
}

func (context *Context) newResourcePath(res *Resource) string {
	return path.Join(context.URLFor(res), "new")
}

func (context *Context) linkTo(text interface{}, link interface{}) template.HTML {
	text = reflect.Indirect(reflect.ValueOf(text)).Interface()
	if linkStr, ok := link.(string); ok {
		return template.HTML(fmt.Sprintf(`<a href="%v">%v</a>`, linkStr, text))
	}
	return template.HTML(fmt.Sprintf(`<a href="%v">%v</a>`, context.URLFor(link), text))
}

func (context *Context) valueOf(valuer func(interface{}, *qor.Context) interface{}, value interface{}, meta *Meta) interface{} {
	if valuer != nil {
		reflectValue := reflect.ValueOf(value)
		if reflectValue.Kind() != reflect.Ptr {
			if !reflectValue.IsValid() {
				return nil
			}
			reflectPtr := reflect.New(reflectValue.Type())
			reflectPtr.Elem().Set(reflectValue)
			value = reflectPtr.Interface()
		}

		result := valuer(value, context.Context)

		if reflectValue := reflect.ValueOf(result); reflectValue.IsValid() {
			if reflectValue.Kind() == reflect.Ptr {
				if reflectValue.IsNil() || !reflectValue.Elem().IsValid() {
					return nil
				}

				result = reflectValue.Elem().Interface()
			}

			if meta.Type == "number" || meta.Type == "float" {
				if context.isNewRecord(value) && equal(reflect.Zero(reflect.TypeOf(result)).Interface(), result) {
					return nil
				}
			}
			return result
		}
		return nil
	}

	utils.ExitWithMsg(fmt.Sprintf("No valuer found for meta %v of resource %v", meta.Name, meta.baseResource.Name))
	return nil
}

func (context *Context) renderFormLayout(value interface{}, layout *Layout) template.HTML {

	templatedPage := context.templateLayout(value, layout, []string{"QorResource"}, "form")

	return templatedPage
}

func (context *Context) templateLayout(value interface{}, layout *Layout, prefix []string, kind string) template.HTML {

	var subLayouts []struct {
		Title      string
		LayoutHTML template.HTML
	}

	for _, subLayout := range layout.Layouts {
		subLayouts = append(subLayouts, struct {
			Title      string
			LayoutHTML template.HTML
		}{
			Title:      subLayout.Title,
			LayoutHTML: context.templateLayout(value, subLayout, []string{"QorResource"}, "form"),
		})
	}

	var sectionHTML template.HTML
	if layout.Section != nil && layout.Section.Section != nil {
		sectionHTML = template.HTML(context.templateSection(value, layout.Section.Section, []string{"QorResource"}, "form"))
	}

	var result = bytes.NewBufferString("")

	var n *int
	if layout.Section != nil {
		n = &layout.Section.Number
	}
	var data = map[string]interface{}{
		"Title":         template.HTML(layout.Title),
		"Layouts":       subLayouts,
		"Section":       sectionHTML,
		"SectionNumber": n,
	}
	if len(layout.Type) > 0 {
		if content, err := context.Asset("metas/" + layout.Type + ".tmpl"); err == nil {

			if tmpl, err := template.New("section").Funcs(context.FuncMap()).Parse(string(content)); err == nil {
				tmpl.Execute(result, data)
			}
		}
	} else { // Fallback to layout
		if content, err := context.Asset("metas/layout.tmpl"); err == nil {
			if tmpl, err := template.New("section").Funcs(context.FuncMap()).Parse(string(content)); err == nil {
				tmpl.Execute(result, data)
			}
		}
	}

	//TODO: fix Templates being returned, add titles to template
	t := template.HTML(result.String())
	return t
}

func (context *Context) renderForm(value interface{}, sections []*Section) template.HTML {
	var result = bytes.NewBufferString("")
	result.Write(context.renderSections(value, sections, []string{"QorResource"}, "form"))

	return template.HTML(result.String())
}

func (context *Context) renderSections(value interface{}, sections []*Section, prefix []string, kind string) []byte {
	var renderedSection []byte
	for _, section := range sections {
		renderedSection = append(renderedSection[:], context.templateSection(value, section, prefix, kind)[:]...)
	}
	return renderedSection
}

func (context *Context) templateSection(value interface{}, section *Section, prefix []string, kind string) []byte {

	var rows []struct {
		Length      int
		Columns     []string
		ColumnsHTML template.HTML
	}

	for _, columnInterface := range section.Rows {
		column := columnInterface
		columnsHTML := bytes.NewBufferString("")
		for _, col := range column {
			meta := section.Resource.GetMeta(col)
			if meta != nil {
				context.renderMeta(meta, value, prefix, kind, columnsHTML)
			}
		}

		rows = append(rows, struct {
			Length      int
			Columns     []string
			ColumnsHTML template.HTML
		}{
			Length:      len(column),
			Columns:     column,
			ColumnsHTML: template.HTML(string(columnsHTML.Bytes())),
		})
	}
	renderedSection := bytes.NewBufferString("")

	if len(rows) > 0 {
		var data = map[string]interface{}{
			"Section": section,
			"Title":   template.HTML(section.Title),
			"Rows":    rows,
		}
		if len(section.Type) > 0 {
			if content, err := context.Asset("metas/" + section.Type + ".tmpl"); err == nil {

				if tmpl, err := template.New("section").Funcs(context.FuncMap()).Parse(string(content)); err == nil {
					tmpl.Execute(renderedSection, data)
				}
			}
		} else { // Fallback to section
			if content, err := context.Asset("metas/section.tmpl"); err == nil {
				if tmpl, err := template.New("section").Funcs(context.FuncMap()).Parse(string(content)); err == nil {
					tmpl.Execute(renderedSection, data)
				}
			}
		}
	}

	return renderedSection.Bytes()
}

func (context *Context) renderFilter(filter *Filter) template.HTML {
	var (
		err     error
		content []byte
		result  = bytes.NewBufferString("")
	)

	defer func() {
		if r := recover(); r != nil {
			fmt.Println(r)
			debug.PrintStack()
			result.WriteString(fmt.Sprintf("Get error when render template for filter %v (%v): %v", filter.Name, filter.Type, r))
		}
	}()

	if content, err = context.Asset(fmt.Sprintf("metas/filter/%v.tmpl", filter.Type)); err == nil {
		tmpl := template.New(filter.Type + ".tmpl").Funcs(context.FuncMap())
		if tmpl, err = tmpl.Parse(string(content)); err == nil {
			var data = map[string]interface{}{
				"Filter":          filter,
				"Label":           filter.Label,
				"InputNamePrefix": fmt.Sprintf("filters[%v]", filter.Name),
				"Context":         context,
				"Resource":        context.Resource,
			}

			err = tmpl.Execute(result, data)
		}
	}

	if err != nil {
		result.WriteString(fmt.Sprintf("got error when render filter template for %v(%v):%v", filter.Name, filter.Type, err))
	}

	return template.HTML(result.String())
}

func (context *Context) savedFilters() (filters []SavedFilter) {
	context.Admin.SettingsStorage.Get("saved_filters", &filters, context)
	return
}

func (context *Context) renderMeta(meta *Meta, value interface{}, prefix []string, metaType string, writer *bytes.Buffer) {
	var (
		err      error
		funcsMap = context.FuncMap()
	)
	prefix = append(prefix, meta.Name)

	var generateNestedRenderSections = func(kind string) func(interface{}, []*Section, int) template.HTML {
		return func(value interface{}, sections []*Section, index int) template.HTML {
			var result = bytes.NewBufferString("")
			var newPrefix = append([]string{}, prefix...)

			if index >= 0 {
				last := newPrefix[len(newPrefix)-1]
				newPrefix = append(newPrefix[:len(newPrefix)-1], fmt.Sprintf("%v[%v]", last, index))
			}

			if len(sections) > 0 {
				scope := utils.NewScope(value)
				for _, field := range scope.PrimaryFields {
					if meta := sections[0].Resource.GetMeta(field.Name); meta != nil {
						context.renderMeta(meta, value, newPrefix, kind, result)
					}
				}

				result.Write(context.renderSections(value, sections, newPrefix, kind))
			}

			return template.HTML(result.String())
		}
	}

	funcsMap["has_change_permission"] = func(permissioner HasPermissioner) bool {
		if utils.PrimaryKeyZero(value) {
			return context.hasCreatePermission(permissioner)
		}
		return context.hasUpdatePermission(permissioner)
	}
	funcsMap["render_nested_form"] = generateNestedRenderSections("form")

	defer func() {
		if r := recover(); r != nil {
			debug.PrintStack()
			writer.WriteString(fmt.Sprintf("Get error when render template for meta %v (%v): %v", meta.Name, meta.Type, r))
		}
	}()

	var (
		tmpl    = template.New(meta.Type + ".tmpl").Funcs(funcsMap)
		content []byte
	)

	switch {
	case meta.Config != nil:
		if templater, ok := meta.Config.(interface {
			GetTemplate(context *Context, metaType string) ([]byte, error)
		}); ok {
			if content, err = templater.GetTemplate(context, metaType); err == nil {
				tmpl, err = tmpl.Parse(string(content))
				break
			}
		}
		fallthrough
	default:
		if content, err = context.Asset(fmt.Sprintf("%v/metas/%v/%v.tmpl", meta.baseResource.ToParam(), metaType, meta.Name), fmt.Sprintf("metas/%v/%v.tmpl", metaType, meta.Type)); err == nil {
			tmpl, err = tmpl.Parse(string(content))
		} else if metaType == "index" {
			tmpl, err = tmpl.Parse("{{.Value}}")
		} else {
			err = fmt.Errorf("haven't found template \"%v/%v.tmpl\" for meta %q", metaType, meta.Type, meta.Name)
		}
	}

	if err == nil {
		var scope = utils.NewScope(value)
		var data = map[string]interface{}{
			"Context":       context,
			"BaseResource":  meta.baseResource,
			"Meta":          meta,
			"ResourceValue": value,
			"Value":         context.FormattedValueOf(value, meta),
			"Label":         meta.Label,
			"InputName":     strings.Join(prefix, "."),
		}

		if !utils.PrimaryKeyZero(value) {
			valueIndirect := reflect.Indirect(reflect.ValueOf(value))
			var v interface{}
			if rvField := valueIndirect.FieldByName(scope.PrimaryFieldDBNames[0]); rvField.IsValid() {
				v = rvField.Interface()
			}
			data["InputId"] = utils.ToParamString(fmt.Sprintf("%v_%v_%v", valueIndirect.Type().Name(), v, meta.Name))
		}

		data["CollectionValue"] = func() [][]string {
			fmt.Printf("%v: Call .CollectionValue from views already Deprecated, get the value with `.Meta.Config.GetCollection .ResourceValue .Context`", meta.Name)
			return meta.Config.(interface {
				GetCollection(value interface{}, context *Context) [][]string
			}).GetCollection(value, context)
		}

		err = tmpl.Execute(writer, data)
	}

	if err != nil {
		msg := fmt.Sprintf("got error when render %v template for %v(%v): %v", metaType, meta.Name, meta.Type, err)
		fmt.Fprint(writer, msg)
		utils.ExitWithMsg(msg)
	}
}

// isEqual export for test only. If values are struct, compare their primary key. otherwise treat them as string
func (context *Context) isEqual(value interface{}, comparativeValue interface{}) bool {
	var result string

	if (value == nil || comparativeValue == nil) && (value != comparativeValue) {
		return false
	}

	if reflect.Indirect(reflect.ValueOf(comparativeValue)).Kind() == reflect.Struct {
		result = fmt.Sprint(context.primaryKeyOf(comparativeValue))
	} else {
		result = fmt.Sprint(comparativeValue)
	}

	reflectValue := reflect.Indirect(reflect.ValueOf(value))
	if reflectValue.Kind() == reflect.Struct {
		return fmt.Sprint(context.primaryKeyOf(value)) == result
	} else if reflectValue.Kind() == reflect.String {
		// type UserType string, alias type will panic if do
		// return reflectValue.Interface().(string) == result
		return fmt.Sprint(reflectValue.Interface()) == result
	} else {
		return fmt.Sprint(reflectValue.Interface()) == result
	}
}

func (context *Context) isIncluded(value interface{}, hasValue interface{}) bool {
	var result string
	if reflect.Indirect(reflect.ValueOf(hasValue)).Kind() == reflect.Struct {
		scope := utils.NewScope(hasValue)
		rv := reflect.ValueOf(value)
		result = fmt.Sprint(rv.FieldByName(scope.PrimaryFieldDBNames[0]).Interface())
	} else {
		result = fmt.Sprint(hasValue)
	}

	primaryKeys := []interface{}{}
	reflectValue := reflect.Indirect(reflect.ValueOf(value))

	if reflectValue.Kind() == reflect.Slice {
		for i := 0; i < reflectValue.Len(); i++ {
			if value := reflectValue.Index(i); value.IsValid() {
				if reflect.Indirect(value).Kind() == reflect.Struct {
					scope := utils.NewScope(reflectValue.Index(i).Interface())
					rv := reflect.ValueOf(value)
					primaryKeyValue := rv.FieldByName(scope.PrimaryFieldDBNames[0]).Interface()
					primaryKeys = append(primaryKeys, primaryKeyValue)
				} else {
					primaryKeys = append(primaryKeys, reflect.Indirect(reflectValue.Index(i)).Interface())
				}
			}
		}
	} else if reflectValue.Kind() == reflect.Struct {
		scope := utils.NewScope(value)
		primaryKeys = append(primaryKeys, reflect.ValueOf(value).FieldByName(scope.PrimaryFieldDBNames[0]).Interface())
	} else if reflectValue.Kind() == reflect.String {
		return strings.Contains(reflectValue.Interface().(string), result)
	} else if reflectValue.IsValid() {
		primaryKeys = append(primaryKeys, reflect.Indirect(reflectValue).Interface())
	}

	for _, key := range primaryKeys {
		if fmt.Sprint(key) == result {
			return true
		}
	}
	return false
}

func (context *Context) getResource(resources ...*Resource) *Resource {
	for _, res := range resources {
		return res
	}
	return context.Resource
}

func (context *Context) indexSections(resources ...*Resource) []*Section {
	res := context.getResource(resources...)
	return res.allowedSections(res.IndexAttrs(), context, roles.Read)
}

func (context *Context) editSections(resources ...*Resource) []*Section {
	res := context.getResource(resources...)
	return res.allowedSections(res.EditAttrs(), context, roles.Update)
}

func (context *Context) editLayout(resources ...*Resource) (layout *Layout) {
	res := context.getResource(resources...)
	layout = res.EditLayout(res.allowedSections(res.EditAttrs(), context, roles.Update))
	return layout
}

func (context *Context) newSections(resources ...*Resource) []*Section {
	res := context.getResource(resources...)
	return res.allowedSections(res.NewAttrs(), context, roles.Create)
}

func (context *Context) newLayout(resources ...*Resource) (layout *Layout) {
	res := context.getResource(resources...)
	//TODO: analyse if a section will ever be deleted if no fields exist in it. If so will need to sdjust layout integers
	layout = res.NewLayout(res.allowedSections(res.NewAttrs(), context, roles.Create))

	return layout
}

func (context *Context) showSections(resources ...*Resource) []*Section {
	res := context.getResource(resources...)
	return res.allowedSections(res.ShowAttrs(), context, roles.Read)
}

func (context *Context) showLayout(resources ...*Resource) (layout *Layout) {
	res := context.getResource(resources...)
	//TODO: analyse if a section will ever be deleted if no fields exist in it. If so will need to sdjust layout integers
	layout = res.ShowLayout(res.allowedSections(res.ShowAttrs(), context, roles.Read))

	return layout
}

type menu struct {
	*Menu
	Active   bool
	SubMenus []*menu
}

func (context *Context) getMenus() (menus []*menu) {
	var (
		globalMenu        = &menu{}
		mostMatchedMenu   *menu
		mostMatchedLength int
		addMenu           func(*menu, []*Menu)
	)

	addMenu = func(parent *menu, menus []*Menu) {
		for _, m := range menus {
			url := m.URL()
			if m.HasPermission(roles.Read, context.Context) {
				var menu = &menu{Menu: m}
				if strings.HasPrefix(context.Request.URL.Path, url) && len(url) > mostMatchedLength {
					mostMatchedMenu = menu
					mostMatchedLength = len(url)
				}

				addMenu(menu, menu.GetSubMenus())
				if len(menu.SubMenus) > 0 || menu.URL() != "" {
					parent.SubMenus = append(parent.SubMenus, menu)
				}
			}
		}
	}

	addMenu(globalMenu, context.Admin.GetMenus())

	if context.Action != "search_center" && mostMatchedMenu != nil {
		mostMatchedMenu.Active = true
	}

	return globalMenu.SubMenus
}

type scope struct {
	*Scope
	Active bool
}

type scopeMenu struct {
	Group  string
	Scopes []*scope
}

// getScopes get scopes from current context
func (context *Context) getScopes() (menus []*scopeMenu) {
	if context.Resource == nil {
		return
	}

	activatedScopeNames := context.Request.URL.Query()["scopes"]
OUT:
	for _, s := range context.Resource.scopes {
		if s.Visible != nil && !s.Visible(context) {
			continue
		}

		menu := &scope{Scope: s}

		for _, s := range activatedScopeNames {
			if s == menu.Name {
				menu.Active = true
			}
		}

		if menu.Group != "" {
			for _, m := range menus {
				if m.Group == menu.Group {
					m.Scopes = append(m.Scopes, menu)
					continue OUT
				}
			}
			menus = append(menus, &scopeMenu{Group: menu.Group, Scopes: []*scope{menu}})
		} else if !menu.Default {
			menus = append(menus, &scopeMenu{Group: menu.Group, Scopes: []*scope{menu}})
		}
	}

	for _, menu := range menus {
		hasActivedScope, hasDefaultScope := false, false
		for _, scope := range menu.Scopes {
			if scope.Active {
				hasActivedScope = true
			}
			if scope.Default {
				hasDefaultScope = true
			}
		}

		if hasDefaultScope && !hasActivedScope {
			for _, scope := range menu.Scopes {
				if scope.Default {
					scope.Active = true
				}
			}
		}
	}
	return menus
}

// getFilters get filters from current context
func (context *Context) getFilters() (filters []*Filter) {
	if context.Resource == nil {
		return
	}

	for _, f := range context.Resource.filters {
		if f.Visible == nil || f.Visible(context) {
			filters = append(filters, f)
		}
	}
	return
}

func (context *Context) hasCreatePermission(permissioner HasPermissioner) bool {
	return permissioner.HasPermission(roles.Create, context.Context)
}

func (context *Context) hasReadPermission(permissioner HasPermissioner) bool {
	return permissioner.HasPermission(roles.Read, context.Context)
}

func (context *Context) hasUpdatePermission(permissioner HasPermissioner) bool {
	return permissioner.HasPermission(roles.Update, context.Context)
}

func (context *Context) hasDeletePermission(permissioner HasPermissioner) bool {
	return permissioner.HasPermission(roles.Delete, context.Context)
}

func (context *Context) hasChangePermission(permissioner HasPermissioner) bool {
	if context.Action == "new" {
		return context.hasCreatePermission(permissioner)
	}
	return context.hasUpdatePermission(permissioner)
}

// PatchCurrentURL is a convinent wrapper for qor/utils.PatchURL
func (context *Context) patchCurrentURL(params ...interface{}) (patchedURL string, err error) {
	return utils.PatchURL(context.Request.URL.String(), params...)
}

// PatchURL is a convinent wrapper for qor/utils.PatchURL
func (context *Context) patchURL(url string, params ...interface{}) (patchedURL string, err error) {
	return utils.PatchURL(url, params...)
}

// JoinCurrentURL is a convinent wrapper for qor/utils.JoinURL
func (context *Context) joinCurrentURL(params ...interface{}) (joinedURL string, err error) {
	return utils.JoinURL(context.Request.URL.String(), params...)
}

// JoinURL is a convinent wrapper for qor/utils.JoinURL
func (context *Context) joinURL(url string, params ...interface{}) (joinedURL string, err error) {
	return utils.JoinURL(url, params...)
}

func (context *Context) themesClass() (result string) {
	var results = map[string]bool{}
	if context.Resource != nil {
		for _, theme := range context.Resource.Config.Themes {
			if strings.HasPrefix(theme.GetName(), "-") {
				results[strings.TrimPrefix(theme.GetName(), "-")] = false
			} else if _, ok := results[theme.GetName()]; !ok {
				results[theme.GetName()] = true
			}
		}
	}

	var names []string
	for name, enabled := range results {
		if enabled {
			names = append(names, "qor-theme-"+name)
		}
	}
	return strings.Join(names, " ")
}

func (context *Context) javaScriptTag(names ...string) template.HTML {
	var results []string
	for _, name := range names {
		name = path.Join(context.Admin.GetRouter().Prefix, "assets", "javascripts", name+".js")
		results = append(results, fmt.Sprintf(`<script src="%s"></script>`, name))
	}
	return template.HTML(strings.Join(results, ""))
}

func (context *Context) javaScriptTagDefer(names ...string) template.HTML {
	var results []string
	for _, name := range names {
		name = path.Join(context.Admin.GetRouter().Prefix, "assets", "javascripts", name+".js")
		results = append(results, fmt.Sprintf(`<script defer src="%s"></script>`, name))
	}
	return template.HTML(strings.Join(results, ""))
}

func (context *Context) styleSheetTag(names ...string) template.HTML {
	var results []string
	for _, name := range names {
		name = path.Join(context.Admin.GetRouter().Prefix, "assets", "stylesheets", name+".css")
		results = append(results, fmt.Sprintf(`<link type="text/css" rel="stylesheet" href="%s">`, name))
	}
	return template.HTML(strings.Join(results, ""))
}

func (context *Context) getThemeNames() (themes []string) {
	themesMap := map[string]bool{}

	if context.Resource != nil {
		for _, theme := range context.Resource.Config.Themes {
			if _, ok := themesMap[theme.GetName()]; !ok {
				themes = append(themes, theme.GetName())
			}
		}
	}

	return
}

func (context *Context) loadThemeStyleSheets() template.HTML {
	var results []string
	for _, themeName := range context.getThemeNames() {
		var file = path.Join("themes", themeName, "assets", "stylesheets", themeName+".css")
		if _, err := context.Asset(file); err == nil {
			results = append(results, fmt.Sprintf(`<link type="text/css" rel="stylesheet" href="%s?theme=%s">`, path.Join(context.Admin.GetRouter().Prefix, "assets", "stylesheets", themeName+".css"), themeName))
		}
	}

	return template.HTML(strings.Join(results, " "))
}

func (context *Context) loadThemeJavaScripts() template.HTML {
	var results []string
	for _, themeName := range context.getThemeNames() {
		var file = path.Join("themes", themeName, "assets", "javascripts", themeName+".js")
		if _, err := context.Asset(file); err == nil {
			results = append(results, fmt.Sprintf(`<script src="%s?theme=%s"></script>`, path.Join(context.Admin.GetRouter().Prefix, "assets", "javascripts", themeName+".js"), themeName))
		}
	}

	return template.HTML(strings.Join(results, " "))
}

func (context *Context) loadAdminJavaScripts() template.HTML {
	var siteName = context.Admin.SiteName
	if siteName == "" {
		siteName = "application"
	}

	var file = path.Join("assets", "javascripts", strings.ToLower(strings.Replace(siteName, " ", "_", -1))+".js")
	if _, err := context.Asset(file); err == nil {
		return template.HTML(fmt.Sprintf(`<script src="%s"></script>`, path.Join(context.Admin.GetRouter().Prefix, file)))
	}
	return ""
}

func (context *Context) loadAdminStyleSheets() template.HTML {
	var siteName = context.Admin.SiteName
	if siteName == "" {
		siteName = "application"
	}

	var file = path.Join("assets", "stylesheets", strings.ToLower(strings.Replace(siteName, " ", "_", -1))+".css")
	if _, err := context.Asset(file); err == nil {
		return template.HTML(fmt.Sprintf(`<link type="text/css" rel="stylesheet" href="%s">`, path.Join(context.Admin.GetRouter().Prefix, file)))
	}
	return ""
}

func (context *Context) loadActions(action string) template.HTML {
	var (
		actionPatterns, actionKeys, actionFiles []string
		actions                                 = map[string]string{}
	)

	switch action {
	case "index", "show", "edit", "new":
		actionPatterns = []string{filepath.Join("actions", action, "*.tmpl"), "actions/*.tmpl"}

		if !context.Resource.sections.ConfiguredShowAttrs && action == "edit" {
			actionPatterns = []string{filepath.Join("actions", "show", "*.tmpl"), "actions/*.tmpl"}
		}
	case "global":
		actionPatterns = []string{"actions/*.tmpl"}
	default:
		actionPatterns = []string{filepath.Join("actions", action, "*.tmpl")}
	}

	for _, pattern := range actionPatterns {
		for _, themeName := range context.getThemeNames() {
			if resourcePath := context.resourcePath(); resourcePath != "" {
				if matches, err := context.Admin.AssetFS.Glob(filepath.Join("themes", themeName, resourcePath, pattern)); err == nil {
					actionFiles = append(actionFiles, matches...)
				}
			}

			if matches, err := context.Admin.AssetFS.Glob(filepath.Join("themes", themeName, pattern)); err == nil {
				actionFiles = append(actionFiles, matches...)
			}
		}

		if resourcePath := context.resourcePath(); resourcePath != "" {
			if matches, err := context.Admin.AssetFS.Glob(filepath.Join(resourcePath, pattern)); err == nil {
				actionFiles = append(actionFiles, matches...)
			}
		}

		if matches, err := context.Admin.AssetFS.Glob(pattern); err == nil {
			actionFiles = append(actionFiles, matches...)
		}
	}

	// before files have higher priority
	for _, actionFile := range actionFiles {
		base := regexp.MustCompile("^\\d+\\.").ReplaceAllString(path.Base(actionFile), "")

		if _, ok := actions[base]; !ok {
			actionKeys = append(actionKeys, path.Base(actionFile))
			actions[base] = actionFile
		}
	}

	sort.Strings(actionKeys)

	var result = bytes.NewBufferString("")
	for _, key := range actionKeys {
		defer func() {
			if r := recover(); r != nil {
				err := fmt.Sprintf("Get error when render action %v: %v", key, r)
				utils.ExitWithMsg(err)
				result.WriteString(err)
			}
		}()

		base := regexp.MustCompile("^\\d+\\.").ReplaceAllString(key, "")
		if content, err := context.Asset(actions[base]); err == nil {
			if tmpl, err := template.New(filepath.Base(actions[base])).Funcs(context.FuncMap()).Parse(string(content)); err == nil {
				if err := tmpl.Execute(result, context); err != nil {
					result.WriteString(err.Error())
					utils.ExitWithMsg(err)
				}
			} else {
				result.WriteString(err.Error())
				utils.ExitWithMsg(err)
			}
		}
	}

	return template.HTML(strings.TrimSpace(result.String()))
}

func (context *Context) logoutURL() string {
	if context.Admin.Auth != nil {
		return context.Admin.Auth.LogoutURL(context)
	}
	return ""
}

func (context *Context) t(values ...interface{}) template.HTML {
	switch len(values) {
	case 1:
		return context.Admin.T(context.Context, fmt.Sprint(values[0]), fmt.Sprint(values[0]))
	case 2:
		return context.Admin.T(context.Context, fmt.Sprint(values[0]), fmt.Sprint(values[1]))
	case 3:
		return context.Admin.T(context.Context, fmt.Sprint(values[0]), fmt.Sprint(values[1]), values[2:]...)
	default:
		utils.ExitWithMsg("passed wrong params for T")
	}
	return ""
}

func (context *Context) isSortableMeta(meta *Meta) bool {
	for _, attr := range context.Resource.SortableAttrs() {
		if attr == meta.Name && meta.FieldStruct != nil && meta.FieldStruct.DBName != "" {
			return true
		}
	}
	return false
}

func (context *Context) convertSectionToMetas(res *Resource, sections []*Section) []*Meta {
	return res.ConvertSectionToMetas(sections)
}

type formatedError struct {
	Label  string
	Errors []string
}

func (context *Context) getFormattedErrors() (formatedErrors []formatedError) {
	type labelInterface interface {
		Label() string
	}

	for _, err := range context.GetErrors() {
		if labelErr, ok := err.(labelInterface); ok {
			var found bool
			label := labelErr.Label()
			for _, formatedError := range formatedErrors {
				if formatedError.Label == label {
					formatedError.Errors = append(formatedError.Errors, err.Error())
				}
			}
			if !found {
				formatedErrors = append(formatedErrors, formatedError{Label: label, Errors: []string{err.Error()}})
			}
		} else {
			formatedErrors = append(formatedErrors, formatedError{Errors: []string{err.Error()}})
		}
	}
	return
}

// AllowedActions return allowed actions based on context
func (context *Context) AllowedActions(actions []*Action, mode string, records ...interface{}) []*Action {
	var allowedActions []*Action
	for _, action := range actions {
		for _, m := range action.Modes {
			if m == mode || (m == "index" && mode == "batch") {
				var permission = roles.Update
				switch strings.ToUpper(action.Method) {
				case "POST":
					permission = roles.Create
				case "DELETE":
					permission = roles.Delete
				case "PUT":
					permission = roles.Update
				case "GET":
					permission = roles.Read
				}

				if action.isAllowed(permission, context, records...) {
					allowedActions = append(allowedActions, action)
					break
				}
			}
		}
	}
	return allowedActions
}

func (context *Context) controllerAction() template.HTML {
	return template.HTML(context.Action)
}

func (context *Context) pageTitle() template.HTML {
	if context.Action == "search_center" {
		return context.t("qor_admin.search_center.title", "Search Center")
	}

	if context.Resource == nil {
		return context.t("qor_admin.layout.title", "Admin")
	}

	if context.Action == "action" {
		if action, ok := context.Result.(*Action); ok {
			return context.t(fmt.Sprintf("%v.actions.%v", context.Resource.ToParam(), action.Label), action.Label)
		}
	}

	var (
		defaultValue string
		titleKey     = fmt.Sprintf("qor_admin.form.%v.title", context.Action)
		usePlural    bool
	)

	switch context.Action {
	case "new":
		defaultValue = "Add {{$1}}"
	case "edit":
		defaultValue = "Edit {{$1}}"
	case "show":
		defaultValue = "{{$1}} Details"
	default:
		defaultValue = "{{$1}}"
		if !context.Resource.Config.Singleton {
			usePlural = true
		}
	}

	var resourceName string
	if usePlural {
		resourceName = string(context.t(fmt.Sprintf("%v.name.plural", context.Resource.ToParam()), inflection.Plural(context.Resource.Name)))
	} else {
		resourceName = string(context.t(fmt.Sprintf("%v.name", context.Resource.ToParam()), context.Resource.Name))
	}

	return context.t(titleKey, defaultValue, resourceName)
}

func (context *Context) resourceName() template.HTML {
	return template.HTML(string(context.Resource.Name))
}

type crumb struct {
	Name string
	URL  string
}

func (context *Context) breadcrumbs() []crumb {

	res := context.Resource
	crumbs := []crumb{}
	for res != nil {

		// Append resource primary value
		id, err := strconv.ParseUint(res.GetPrimaryValue(context.Request), 10, 32)
		if id > 0 && err == nil {
			crumbs = append([]crumb{{
				Name: res.GetPrimaryValue(context.Request),
				URL:  context.URLFor(res) + "/" + strconv.FormatUint(id, 10),
			}}, crumbs...)
		} else if context.Action == "new" {
			crumbs = append([]crumb{{
				Name: "New " + res.Name,
				URL:  context.URLFor(res) + "/new",
			}}, crumbs...)
		}
		name := ""
		if res.Config.Singleton {
			name = string(context.t(fmt.Sprintf("%v.name", res.ToParam()), res.Name))
		} else {
			name = string(context.t(fmt.Sprintf("%v.name.plural", res.ToParam()), res.Name))
		}
		// Append resource
		crumbs = append([]crumb{{
			Name: name,
			URL:  context.URLFor(res),
		}}, crumbs...)

		res = res.ParentResource
	}
	return crumbs
}

// filterBy is a generic function that takes an array or a slice of pointers to objects, a field name, and a value,
// and returns an array of pointers to objects where the given field matches the given value.
func (context *Context) filterBy(values interface{}, field string, value interface{}) interface{} {
	// Convert values interface{} to a reflect.Value
	valuesValue := reflect.ValueOf(values)
	if valuesValue.Kind() == reflect.Ptr {
		valuesValue = valuesValue.Elem()
	}
	if valuesValue.Kind() != reflect.Array && valuesValue.Kind() != reflect.Slice {
		panic("values must be an array, a slice, or a pointer to an array/slice")
	}
	resultsSlice := reflect.MakeSlice(valuesValue.Type(), 0, 0)
	for i := 0; i < valuesValue.Len(); i++ {
		objectValue := valuesValue.Index(i)
		if objectValue.Kind() == reflect.Ptr {
			objectValue = objectValue.Elem()
		}
		fieldValue := objectValue.FieldByName(field)
		if fieldValue.IsValid() && fieldValue.Interface() == value {
			resultsSlice = reflect.Append(resultsSlice, valuesValue.Index(i))
		}
	}
	return resultsSlice.Interface()
}

func (context *Context) isKanban(data interface{}, fieldName string) bool {

	reflectValue := reflect.ValueOf(data).Elem()
	ok := false
	// if reflect value is a slice or array
	if reflectValue.Kind() == reflect.Slice || reflectValue.Kind() == reflect.Array {
		// if slice or array is empty
		sliceType := reflectValue.Type().Elem()
		sliceType = sliceType.Elem()
		_, ok = sliceType.FieldByName(fieldName)
	}

	return ok
}

func (context *Context) getColumns(data interface{}, fieldName string) (any, error) {

	reflectValue := reflect.ValueOf(data).Elem()

	var sliceType reflect.Type
	if reflectValue.Kind() == reflect.Slice {
		sliceType = reflectValue.Type().Elem()
		sliceType = sliceType.Elem()
	} else {
		sliceType = reflectValue.Type()
	}

	fieldType, ok := sliceType.FieldByName(fieldName)

	if !ok {
		return nil, fmt.Errorf("field not found: %s", fieldName)
	}

	// Get the element type of the slice or pointer
	elemType := fieldType.Type
	if fieldType.Type.Kind() == reflect.Ptr || fieldType.Type.Kind() == reflect.Slice {
		elemType = fieldType.Type.Elem()
	}

	values := reflect.New(reflect.SliceOf(elemType)).Interface()
	// create a new instance of type of values

	// Query the database and scan the result into the values slice
	err := context.GetDB().Find(values).Error
	if err != nil {
		return nil, err
	}

	// Return the pointer to the slice value
	return values, nil
}

func (context *Context) getNewResources() ([]*Resource, error) {
	var resources []*Resource
	for _, res := range context.Admin.GetResources() {
		//		if res.HasPermission(roles.Create, &qor.Context{CurrentUser: context.Context.CurrentUser}) {
		resources = append(resources, res)
		//		}
	}
	return resources, nil
}

func (context *Context) newRelic() (template.HTML, error) {
	script := `<script type="text/javascript">
	;window.NREUM||(NREUM={});NREUM.init={distributed_tracing:{enabled:true},privacy:{cookies_enabled:true},ajax:{deny_list:["bam.eu01.nr-data.net"]}};
	
	;NREUM.loader_config={accountID:"3418789",trustKey:"3418789",agentID:"207487725",licenseKey:"NRJS-2675a3ce33f8fb2d749",applicationID:"207487725"}
	;NREUM.info={beacon:"bam.eu01.nr-data.net",errorBeacon:"bam.eu01.nr-data.net",licenseKey:"NRJS-2675a3ce33f8fb2d749",applicationID:"207487725",sa:1}
	;(()=>{var e,t,r={9071:(e,t,r)=>{"use strict";r.d(t,{I:()=>n});var n=0,i=navigator.userAgent.match(/Firefox[\/\s](\d+\.\d+)/);i&&(n=+i[1])},8768:(e,t,r)=>{"use strict";r.d(t,{T:()=>n,p:()=>i});const n=/(iPad|iPhone|iPod)/g.test(navigator.userAgent),i=n&&Boolean("undefined"==typeof SharedWorker)},27:(e,t,r)=>{"use strict";r.d(t,{P_:()=>g,Mt:()=>v,C5:()=>d,DL:()=>y,OP:()=>I,lF:()=>k,Yu:()=>E,Dg:()=>p,CX:()=>f,GE:()=>w,sU:()=>P});var n={};r.r(n),r.d(n,{agent:()=>A,match:()=>D,version:()=>x});var i=r(6797),o=r(909),a=r(8610);class s{constructor(e,t){try{if(!e||"object"!=typeof e)return(0,a.Z)("New setting a Configurable requires an object as input");if(!t||"object"!=typeof t)return(0,a.Z)("Setting a Configurable requires a model to set its initial properties");Object.assign(this,t),Object.entries(e).forEach((e=>{let[t,r]=e;const n=(0,o.q)(t);n.length&&r&&"object"==typeof r&&n.forEach((e=>{e in r&&((0,a.Z)('"'.concat(e,'" is a protected attribute and can not be changed in feature ').concat(t,".  It will have no effect.")),delete r[e])})),this[t]=r}))}catch(e){(0,a.Z)("An error occured while setting a Configurable",e)}}}const c={beacon:i.ce.beacon,errorBeacon:i.ce.errorBeacon,licenseKey:void 0,applicationID:void 0,sa:void 0,queueTime:void 0,applicationTime:void 0,ttGuid:void 0,user:void 0,account:void 0,product:void 0,extra:void 0,jsAttributes:{},userAttributes:void 0,atts:void 0,transactionName:void 0,tNamePlain:void 0},u={};function d(e){if(!e)throw new Error("All info objects require an agent identifier!");if(!u[e])throw new Error("Info for ".concat(e," was never set"));return u[e]}function f(e,t){if(!e)throw new Error("All info objects require an agent identifier!");u[e]=new s(t,c),(0,i.Qy)(e,u[e],"info")}const l={allow_bfcache:!0,privacy:{cookies_enabled:!0},ajax:{deny_list:void 0,enabled:!0,harvestTimeSeconds:10},distributed_tracing:{enabled:void 0,exclude_newrelic_header:void 0,cors_use_newrelic_header:void 0,cors_use_tracecontext_headers:void 0,allowed_origins:void 0},ssl:void 0,obfuscate:void 0,jserrors:{enabled:!0,harvestTimeSeconds:10},metrics:{enabled:!0},page_action:{enabled:!0,harvestTimeSeconds:30},page_view_event:{enabled:!0},page_view_timing:{enabled:!0,harvestTimeSeconds:30,long_task:!1},session_trace:{enabled:!0,harvestTimeSeconds:10},spa:{enabled:!0,harvestTimeSeconds:10}},h={};function g(e){if(!e)throw new Error("All configuration objects require an agent identifier!");if(!h[e])throw new Error("Configuration for ".concat(e," was never set"));return h[e]}function p(e,t){if(!e)throw new Error("All configuration objects require an agent identifier!");h[e]=new s(t,l),(0,i.Qy)(e,h[e],"config")}function v(e,t){if(!e)throw new Error("All configuration objects require an agent identifier!");var r=g(e);if(r){for(var n=t.split("."),i=0;i<n.length-1;i++)if("object"!=typeof(r=r[n[i]]))return;r=r[n[n.length-1]]}return r}const m={accountID:void 0,trustKey:void 0,agentID:void 0,licenseKey:void 0,applicationID:void 0,xpid:void 0},b={};function y(e){if(!e)throw new Error("All loader-config objects require an agent identifier!");if(!b[e])throw new Error("LoaderConfig for ".concat(e," was never set"));return b[e]}function w(e,t){if(!e)throw new Error("All loader-config objects require an agent identifier!");b[e]=new s(t,m),(0,i.Qy)(e,b[e],"loader_config")}const E=(0,i.mF)().o;var A=null,x=null;const T=/Version\/(\S+)\s+Safari/;if(navigator.userAgent){var _=navigator.userAgent,S=_.match(T);S&&-1===_.indexOf("Chrome")&&-1===_.indexOf("Chromium")&&(A="Safari",x=S[1])}function D(e,t){if(!A)return!1;if(e!==A)return!1;if(!t)return!0;if(!x)return!1;for(var r=x.split("."),n=t.split("."),i=0;i<n.length;i++)if(n[i]!==r[i])return!1;return!0}var N=r(2400),O=r(2374),j=r(1651);const R=e=>({buildEnv:j.Re,bytesSent:{},customTransaction:void 0,disabled:!1,distMethod:j.gF,isolatedBacklog:!1,loaderType:void 0,maxBytes:3e4,offset:Math.floor(O._A?.performance?.timeOrigin||O._A?.performance?.timing?.navigationStart||Date.now()),onerror:void 0,origin:""+O._A.location,ptid:void 0,releaseIds:{},sessionId:1==v(e,"privacy.cookies_enabled")?(0,N.Bj)():null,xhrWrappable:"function"==typeof O._A.XMLHttpRequest?.prototype?.addEventListener,userAgent:n,version:j.q4}),C={};function I(e){if(!e)throw new Error("All runtime objects require an agent identifier!");if(!C[e])throw new Error("Runtime for ".concat(e," was never set"));return C[e]}function P(e,t){if(!e)throw new Error("All runtime objects require an agent identifier!");C[e]=new s(t,R(e)),(0,i.Qy)(e,C[e],"runtime")}function k(e){return function(e){try{const t=d(e);return!!t.licenseKey&&!!t.errorBeacon&&!!t.applicationID}catch(e){return!1}}(e)}},1651:(e,t,r)=>{"use strict";r.d(t,{Re:()=>i,gF:()=>o,q4:()=>n});const n="1.231.0",i="PROD",o="CDN"},9557:(e,t,r)=>{"use strict";r.d(t,{w:()=>o});var n=r(8610);const i={agentIdentifier:""};class o{constructor(e){try{if("object"!=typeof e)return(0,n.Z)("shared context requires an object as input");this.sharedContext={},Object.assign(this.sharedContext,i),Object.entries(e).forEach((e=>{let[t,r]=e;Object.keys(i).includes(t)&&(this.sharedContext[t]=r)}))}catch(e){(0,n.Z)("An error occured while setting SharedContext",e)}}}},4329:(e,t,r)=>{"use strict";r.d(t,{L:()=>d,R:()=>c});var n=r(3752),i=r(7022),o=r(4045),a=r(2325);const s={};function c(e,t){const r={staged:!1,priority:a.p[t]||0};u(e),s[e].get(t)||s[e].set(t,r)}function u(e){e&&(s[e]||(s[e]=new Map))}function d(){let e=arguments.length>0&&void 0!==arguments[0]?arguments[0]:"",t=arguments.length>1&&void 0!==arguments[1]?arguments[1]:"feature";if(u(e),!e||!s[e].get(t))return a(t);s[e].get(t).staged=!0;const r=Array.from(s[e]);function a(t){const r=e?n.ee.get(e):n.ee,a=o.X.handlers;if(r.backlog&&a){var s=r.backlog[t],c=a[t];if(c){for(var u=0;s&&u<s.length;++u)f(s[u],c);(0,i.D)(c,(function(e,t){(0,i.D)(t,(function(t,r){r[0].on(e,r[1])}))}))}delete a[t],r.backlog[t]=null,r.emit("drain-"+t,[])}}r.every((e=>{let[t,r]=e;return r.staged}))&&(r.sort(((e,t)=>e[1].priority-t[1].priority)),r.forEach((e=>{let[t]=e;a(t)})))}function f(e,t){var r=e[1];(0,i.D)(t[r],(function(t,r){var n=e[0];if(r[0]===n){var i=r[1],o=e[3],a=e[2];i.apply(o,a)}}))}},3752:(e,t,r)=>{"use strict";r.d(t,{c:()=>f,ee:()=>u});var n=r(6797),i=r(3916),o=r(7022),a=r(27),s="nr@context";let c=(0,n.fP)();var u;function d(){}function f(e){return(0,i.X)(e,s,l)}function l(){return new d}function h(){u.aborted=!0,u.backlog={}}c.ee?u=c.ee:(u=function e(t,r){var n={},c={},f={},g=!1;try{g=16===r.length&&(0,a.OP)(r).isolatedBacklog}catch(e){}var p={on:b,addEventListener:b,removeEventListener:y,emit:m,get:E,listeners:w,context:v,buffer:A,abort:h,aborted:!1,isBuffering:x,debugId:r,backlog:g?{}:t&&"object"==typeof t.backlog?t.backlog:{}};return p;function v(e){return e&&e instanceof d?e:e?(0,i.X)(e,s,l):l()}function m(e,r,n,i,o){if(!1!==o&&(o=!0),!u.aborted||i){t&&o&&t.emit(e,r,n);for(var a=v(n),s=w(e),d=s.length,f=0;f<d;f++)s[f].apply(a,r);var l=T()[c[e]];return l&&l.push([p,e,r,a]),a}}function b(e,t){n[e]=w(e).concat(t)}function y(e,t){var r=n[e];if(r)for(var i=0;i<r.length;i++)r[i]===t&&r.splice(i,1)}function w(e){return n[e]||[]}function E(t){return f[t]=f[t]||e(p,t)}function A(e,t){var r=T();p.aborted||(0,o.D)(e,(function(e,n){t=t||"feature",c[n]=t,t in r||(r[t]=[])}))}function x(e){return!!T()[c[e]]}function T(){return p.backlog}}(void 0,"globalEE"),c.ee=u)},9252:(e,t,r)=>{"use strict";r.d(t,{E:()=>n,p:()=>i});var n=r(3752).ee.get("handle");function i(e,t,r,i,o){o?(o.buffer([e],i),o.emit(e,t,r)):(n.buffer([e],i),n.emit(e,t,r))}},4045:(e,t,r)=>{"use strict";r.d(t,{X:()=>o});var n=r(9252);o.on=a;var i=o.handlers={};function o(e,t,r,o){a(o||n.E,i,e,t,r)}function a(e,t,r,i,o){o||(o="feature"),e||(e=n.E);var a=t[o]=t[o]||{};(a[r]=a[r]||[]).push([e,i])}},8544:(e,t,r)=>{"use strict";r.d(t,{bP:()=>s,iz:()=>c,m$:()=>a});var n=r(2374);let i=!1,o=!1;try{const e={get passive(){return i=!0,!1},get signal(){return o=!0,!1}};n._A.addEventListener("test",null,e),n._A.removeEventListener("test",null,e)}catch(e){}function a(e,t){return i||o?{capture:!!e,passive:i,signal:t}:!!e}function s(e,t){let r=arguments.length>2&&void 0!==arguments[2]&&arguments[2];window.addEventListener(e,t,a(r))}function c(e,t){let r=arguments.length>2&&void 0!==arguments[2]&&arguments[2];document.addEventListener(e,t,a(r))}},5526:(e,t,r)=>{"use strict";r.d(t,{Ht:()=>u,M:()=>c,Rl:()=>a,ky:()=>s});var n=r(2374);const i="xxxxxxxx-xxxx-4xxx-yxxx-xxxxxxxxxxxx";function o(e,t){return e?15&e[t]:16*Math.random()|0}function a(){const e=n._A?.crypto||n._A?.msCrypto;let t,r=0;return e&&e.getRandomValues&&(t=e.getRandomValues(new Uint8Array(31))),i.split("").map((e=>"x"===e?o(t,++r).toString(16):"y"===e?(3&o()|8).toString(16):e)).join("")}function s(e){const t=n._A?.crypto||n._A?.msCrypto;let r,i=0;t&&t.getRandomValues&&(r=t.getRandomValues(new Uint8Array(31)));const a=[];for(var s=0;s<e;s++)a.push(o(r,++i).toString(16));return a.join("")}function c(){return s(16)}function u(){return s(32)}},2053:(e,t,r)=>{"use strict";function n(){return Math.round(performance.now())}r.d(t,{z:()=>n})},6368:(e,t,r)=>{"use strict";r.d(t,{e:()=>o});var n=r(2374),i={};function o(e){if(e in i)return i[e];if(0===(e||"").indexOf("data:"))return{protocol:"data"};let t;var r=n._A?.location,o={};if(n.il)t=document.createElement("a"),t.href=e;else try{t=new URL(e,r.href)}catch(e){return o}o.port=t.port;var a=t.href.split("://");!o.port&&a[1]&&(o.port=a[1].split("/")[0].split("@").pop().split(":")[1]),o.port&&"0"!==o.port||(o.port="https"===a[0]?"443":"80"),o.hostname=t.hostname||r.hostname,o.pathname=t.pathname,o.protocol=a[0],"/"!==o.pathname.charAt(0)&&(o.pathname="/"+o.pathname);var s=!t.protocol||":"===t.protocol||t.protocol===r.protocol,c=t.hostname===r.hostname&&t.port===r.port;return o.sameOrigin=s&&(!t.hostname||c),"/"===o.pathname&&(i[e]=o),o}},8610:(e,t,r)=>{"use strict";function n(e,t){"function"==typeof console.warn&&(console.warn("New Relic: ".concat(e)),t&&console.warn(t))}r.d(t,{Z:()=>n})},3916:(e,t,r)=>{"use strict";r.d(t,{X:()=>i});var n=Object.prototype.hasOwnProperty;function i(e,t,r){if(n.call(e,t))return e[t];var i=r();if(Object.defineProperty&&Object.keys)try{return Object.defineProperty(e,t,{value:i,writable:!0,enumerable:!1}),i}catch(e){}return e[t]=i,i}},2374:(e,t,r)=>{"use strict";r.d(t,{_A:()=>o,il:()=>n,v6:()=>i});const n=Boolean("undefined"!=typeof window&&window.document),i=Boolean("undefined"!=typeof WorkerGlobalScope&&self.navigator instanceof WorkerNavigator);let o=(()=>{if(n)return window;if(i){if("undefined"!=typeof globalThis&&globalThis instanceof WorkerGlobalScope)return globalThis;if(self instanceof WorkerGlobalScope)return self}throw new Error('New Relic browser agent shutting down due to error: Unable to locate global scope. This is possibly due to code redefining browser global variables like "self" and "window".')})()},7022:(e,t,r)=>{"use strict";r.d(t,{D:()=>n});const n=(e,t)=>Object.entries(e||{}).map((e=>{let[r,n]=e;return t(r,n)}))},2438:(e,t,r)=>{"use strict";r.d(t,{P:()=>o});var n=r(3752);const i=()=>{const e=new WeakSet;return(t,r)=>{if("object"==typeof r&&null!==r){if(e.has(r))return;e.add(r)}return r}};function o(e){try{return JSON.stringify(e,i())}catch(e){try{n.ee.emit("internal-error",[e])}catch(e){}}}},2650:(e,t,r)=>{"use strict";r.d(t,{K:()=>a,b:()=>o});var n=r(8544);function i(){return"undefined"==typeof document||"complete"===document.readyState}function o(e,t){if(i())return e();(0,n.bP)("load",e,t)}function a(e){if(i())return e();(0,n.iz)("DOMContentLoaded",e)}},6797:(e,t,r)=>{"use strict";r.d(t,{EZ:()=>u,Qy:()=>c,ce:()=>o,fP:()=>a,gG:()=>d,mF:()=>s});var n=r(2053),i=r(2374);const o={beacon:"bam.nr-data.net",errorBeacon:"bam.nr-data.net"};function a(){return i._A.NREUM||(i._A.NREUM={}),void 0===i._A.newrelic&&(i._A.newrelic=i._A.NREUM),i._A.NREUM}function s(){let e=a();return e.o||(e.o={ST:i._A.setTimeout,SI:i._A.setImmediate,CT:i._A.clearTimeout,XHR:i._A.XMLHttpRequest,REQ:i._A.Request,EV:i._A.Event,PR:i._A.Promise,MO:i._A.MutationObserver,FETCH:i._A.fetch}),e}function c(e,t,r){let i=a();const o=i.initializedAgents||{},s=o[e]||{};return Object.keys(s).length||(s.initializedAt={ms:(0,n.z)(),date:new Date}),i.initializedAgents={...o,[e]:{...s,[r]:t}},i}function u(e,t){a()[e]=t}function d(){return function(){let e=a();const t=e.info||{};e.info={beacon:o.beacon,errorBeacon:o.errorBeacon,...t}}(),function(){let e=a();const t=e.init||{};e.init={...t}}(),s(),function(){let e=a();const t=e.loader_config||{};e.loader_config={...t}}(),a()}},6998:(e,t,r)=>{"use strict";r.d(t,{N:()=>i});var n=r(8544);function i(e){let t=arguments.length>1&&void 0!==arguments[1]&&arguments[1];return void(0,n.iz)("visibilitychange",(function(){if(t)return void("hidden"==document.visibilityState&&e());e(document.visibilityState)}))}},2400:(e,t,r)=>{"use strict";r.d(t,{Bj:()=>c,GD:()=>s,J8:()=>u,ju:()=>o});var n=r(5526);const i="NRBA/";function o(e,t){let r=arguments.length>2&&void 0!==arguments[2]?arguments[2]:"";try{return window.sessionStorage.setItem(i+r+e,t),!0}catch(e){return!1}}function a(e){let t=arguments.length>1&&void 0!==arguments[1]?arguments[1]:"";return window.sessionStorage.getItem(i+t+e)}function s(e){let t=arguments.length>1&&void 0!==arguments[1]?arguments[1]:"";try{window.sessionStorage.removeItem(i+t+e)}catch(e){}}function c(){try{let e;return null===(e=a("SESSION_ID"))&&(e=(0,n.ky)(16),o("SESSION_ID",e)),e}catch(e){return null}}function u(){let e=arguments.length>0&&void 0!==arguments[0]?arguments[0]:"";const t=i+e,r={};try{for(let n=0;n<window.sessionStorage.length;n++){let i=window.sessionStorage.key(n);i.startsWith(t)&&(i=i.slice(t.length),r[i]=a(i,e))}}catch(e){}return r}},6408:(e,t,r)=>{"use strict";r.d(t,{W:()=>i});var n=r(2374);function i(){return"function"==typeof n._A?.PerformanceObserver}},8675:(e,t,r)=>{"use strict";r.d(t,{t:()=>n});const n=r(2325).D.ajax},8322:(e,t,r)=>{"use strict";r.d(t,{A:()=>i,t:()=>n});const n=r(2325).D.jserrors,i="nr@seenError"},6034:(e,t,r)=>{"use strict";r.d(t,{gF:()=>o,mY:()=>i,t9:()=>n,vz:()=>s,xS:()=>a});const n=r(2325).D.metrics,i="sm",o="cm",a="storeSupportabilityMetrics",s="storeEventMetrics"},6486:(e,t,r)=>{"use strict";r.d(t,{t:()=>n});const n=r(2325).D.pageAction},2484:(e,t,r)=>{"use strict";r.d(t,{Dz:()=>i,OJ:()=>a,qw:()=>o,t9:()=>n});const n=r(2325).D.pageViewEvent,i="firstbyte",o="domcontent",a="windowload"},6382:(e,t,r)=>{"use strict";r.d(t,{t:()=>n});const n=r(2325).D.pageViewTiming},2628:(e,t,r)=>{"use strict";r.r(t),r.d(t,{ADD_EVENT_LISTENER:()=>p,BST_RESOURCE:()=>a,BST_TIMER:()=>l,END:()=>u,FEATURE_NAME:()=>i,FN_END:()=>f,FN_START:()=>d,ORIG_EVENT:()=>g,PUSH_STATE:()=>h,RESOURCE:()=>s,RESOURCE_TIMING_BUFFER_FULL:()=>o,START:()=>c});var n=r(27);const i=r(2325).D.sessionTrace,o="resourcetimingbufferfull",a="bstResource",s="resource",c="-start",u="-end",d="fn"+c,f="fn"+u,l="bstTimer",h="pushState",g=n.Yu.EV,p="addEventListener"},755:(e,t,r)=>{"use strict";r.r(t),r.d(t,{BODY:()=>A,CB_END:()=>x,CB_START:()=>u,END:()=>E,FEATURE_NAME:()=>i,FETCH:()=>_,FETCH_BODY:()=>m,FETCH_DONE:()=>v,FETCH_START:()=>p,FN_END:()=>c,FN_START:()=>s,INTERACTION:()=>l,INTERACTION_API:()=>d,INTERACTION_EVENTS:()=>o,JSONP_END:()=>b,JSONP_NODE:()=>g,JS_TIME:()=>T,MAX_TIMER_BUDGET:()=>a,REMAINING:()=>f,SPA_NODE:()=>h,START:()=>w,originalSetTimeout:()=>y});var n=r(27);r(2374);const i=r(2325).D.spa,o=["click","submit","keypress","keydown","keyup","change"],a=999,s="fn-start",c="fn-end",u="cb-start",d="api-ixn-",f="remaining",l="interaction",h="spaNode",g="jsonpNode",p="fetch-start",v="fetch-done",m="fetch-body-",b="jsonp-end",y=n.Yu.ST,w="-start",E="-end",A="-body",x="cb"+E,T="jsTime",_="fetch"},1509:(e,t,r)=>{"use strict";r.d(t,{W:()=>s});var n=r(27),i=r(3752),o=r(2384),a=r(6797);class s{constructor(e,t,r){this.agentIdentifier=e,this.aggregator=t,this.ee=i.ee.get(e,(0,n.OP)(this.agentIdentifier).isolatedBacklog),this.featureName=r,this.blocked=!1,this.checkConfiguration()}checkConfiguration(){if(!(0,n.lF)(this.agentIdentifier)){let e={...(0,a.gG)().info?.jsAttributes};try{e={...e,...(0,n.C5)(this.agentIdentifier)?.jsAttributes}}catch(e){}(0,o.j)(this.agentIdentifier,{...(0,a.gG)(),info:{...(0,a.gG)().info,jsAttributes:e}})}}}},2384:(e,t,r)=>{"use strict";r.d(t,{j:()=>w});var n=r(2325),i=r(27),o=r(9252),a=r(3752),s=r(2053),c=r(4329),u=r(2650),d=r(2374),f=r(8610),l=r(6034),h=r(6797),g=r(2400);const p="CUSTOM/";function v(){const e=(0,h.gG)();["setErrorHandler","finished","addToTrace","inlineHit","addRelease","addPageAction","setCurrentRouteName","setPageViewName","setCustomAttribute","interaction","noticeError","setUserId"].forEach((t=>{e[t]=function(){for(var r=arguments.length,n=new Array(r),i=0;i<r;i++)n[i]=arguments[i];return function(t){for(var r=arguments.length,n=new Array(r>1?r-1:0),i=1;i<r;i++)n[i-1]=arguments[i];let o=[];return Object.values(e.initializedAgents).forEach((e=>{e.exposed&&e.api[t]&&o.push(e.api[t](...n))})),o.length>1?o:o[0]}(t,...n)}}))}var m=r(7022);const b={stn:[n.D.sessionTrace],err:[n.D.jserrors,n.D.metrics],ins:[n.D.pageAction],spa:[n.D.spa]};const y={};function w(e){let t=arguments.length>1&&void 0!==arguments[1]?arguments[1]:{},w=arguments.length>2?arguments[2]:void 0,E=arguments.length>3?arguments[3]:void 0,{init:A,info:x,loader_config:T,runtime:_={loaderType:w},exposed:S=!0}=t;const D=(0,h.gG)();if(x||(A=D.init,x=D.info,T=D.loader_config),x.jsAttributes??={},d.v6&&(x.jsAttributes.isWorker=!0),d.il){let e=(0,g.J8)(p);Object.assign(x.jsAttributes,e)}(0,i.CX)(e,x),(0,i.Dg)(e,A||{}),(0,i.GE)(e,T||{}),(0,i.sU)(e,_),v();const N=function(e,t){t||(0,c.R)(e,"api");const h={};var v=a.ee.get(e),m=v.get("tracer"),b="api-",y=b+"ixn-";function w(t,r,n,o){const a=(0,i.C5)(e);return null===r?(delete a.jsAttributes[t],d.il&&(0,g.GD)(t,p)):((0,i.CX)(e,{...a,jsAttributes:{...a.jsAttributes,[t]:r}}),d.il&&o&&(0,g.ju)(t,r,p)),x(b,n,!0)()}function E(){}["setErrorHandler","finished","addToTrace","inlineHit","addRelease"].forEach((e=>h[e]=x(b,e,!0,"api"))),h.addPageAction=x(b,"addPageAction",!0,n.D.pageAction),h.setCurrentRouteName=x(b,"routeName",!0,n.D.spa),h.setPageViewName=function(t,r){if("string"==typeof t)return"/"!==t.charAt(0)&&(t="/"+t),(0,i.OP)(e).customTransaction=(r||"http://custom.transaction")+t,x(b,"setPageViewName",!0)()},h.setCustomAttribute=function(e,t){let r=arguments.length>2&&void 0!==arguments[2]&&arguments[2];if("string"==typeof e){if(["string","number"].includes(typeof t)||null===t)return w(e,t,"setCustomAttribute",r);(0,f.Z)("Failed to execute setCustomAttribute.\nNon-null value must be a string or number type, but a type of <".concat(typeof t,"> was provided."))}else(0,f.Z)("Failed to execute setCustomAttribute.\nName must be a string type, but a type of <".concat(typeof e,"> was provided."))},h.setUserId=function(e){if("string"==typeof e||null===e)return w("enduser.id",e,"setUserId",!0);(0,f.Z)("Failed to execute setUserId.\nNon-null value must be a string type, but a type of <".concat(typeof e,"> was provided."))},h.interaction=function(){return(new E).get()};var A=E.prototype={createTracer:function(e,t){var r={},i=this,a="function"==typeof t;return(0,o.p)(y+"tracer",[(0,s.z)(),e,r],i,n.D.spa,v),function(){if(m.emit((a?"":"no-")+"fn-start",[(0,s.z)(),i,a],r),a)try{return t.apply(this,arguments)}catch(e){throw m.emit("fn-err",[arguments,this,"string"==typeof e?new Error(e):e],r),e}finally{m.emit("fn-end",[(0,s.z)()],r)}}}};function x(e,t,r,i){return function(){return(0,o.p)(l.xS,["API/"+t+"/called"],void 0,n.D.metrics,v),i&&(0,o.p)(e+t,[(0,s.z)(),...arguments],r?null:this,i,v),r?void 0:this}}function T(){r.e(439).then(r.bind(r,5692)).then((t=>{let{setAPI:r}=t;r(e),(0,c.L)(e,"api")})).catch((()=>(0,f.Z)("Downloading runtime APIs failed...")))}return["actionText","setName","setAttribute","save","ignore","onEnd","getContext","end","get"].forEach((e=>{A[e]=x(y,e,void 0,n.D.spa)})),h.noticeError=function(e,t){"string"==typeof e&&(e=new Error(e)),(0,o.p)(l.xS,["API/noticeError/called"],void 0,n.D.metrics,v),(0,o.p)("err",[e,(0,s.z)(),!1,t],void 0,n.D.jserrors,v)},d.v6?T():(0,u.b)((()=>T()),!0),h}(e,E);return(0,h.Qy)(e,N,"api"),(0,h.Qy)(e,S,"exposed"),(0,h.EZ)("activatedFeatures",y),(0,h.EZ)("setToken",(t=>function(e,t){var r=a.ee.get(t);e&&"object"==typeof e&&((0,m.D)(e,(function(e,t){if(!t)return(b[e]||[]).forEach((t=>{(0,o.p)("block-"+e,[],void 0,t,r)}));y[e]||((0,o.p)("feat-"+e,[],void 0,b[e],r),y[e]=!0)})),(0,c.L)(t,n.D.pageViewEvent))}(t,e))),N}},909:(e,t,r)=>{"use strict";r.d(t,{Z:()=>i,q:()=>o});var n=r(2325);function i(e){switch(e){case n.D.ajax:return[n.D.jserrors];case n.D.sessionTrace:return[n.D.ajax,n.D.pageViewEvent];case n.D.pageViewTiming:return[n.D.pageViewEvent];default:return[]}}function o(e){return e===n.D.jserrors?[]:["auto"]}},2325:(e,t,r)=>{"use strict";r.d(t,{D:()=>n,p:()=>i});const n={ajax:"ajax",jserrors:"jserrors",metrics:"metrics",pageAction:"page_action",pageViewEvent:"page_view_event",pageViewTiming:"page_view_timing",sessionTrace:"session_trace",spa:"spa"},i={[n.pageViewEvent]:1,[n.pageViewTiming]:2,[n.metrics]:3,[n.jserrors]:4,[n.ajax]:5,[n.sessionTrace]:6,[n.pageAction]:7,[n.spa]:8}},8683:e=>{e.exports=function(e,t,r){t||(t=0),void 0===r&&(r=e?e.length:0);for(var n=-1,i=r-t||0,o=Array(i<0?0:i);++n<i;)o[n]=e[t+n];return o}}},n={};function i(e){var t=n[e];if(void 0!==t)return t.exports;var o=n[e]={exports:{}};return r[e](o,o.exports,i),o.exports}i.m=r,i.n=e=>{var t=e&&e.__esModule?()=>e.default:()=>e;return i.d(t,{a:t}),t},i.d=(e,t)=>{for(var r in t)i.o(t,r)&&!i.o(e,r)&&Object.defineProperty(e,r,{enumerable:!0,get:t[r]})},i.f={},i.e=e=>Promise.all(Object.keys(i.f).reduce(((t,r)=>(i.f[r](e,t),t)),[])),i.u=e=>(({78:"page_action-aggregate",147:"metrics-aggregate",193:"session_trace-aggregate",317:"jserrors-aggregate",348:"page_view_timing-aggregate",439:"async-api",729:"lazy-loader",786:"page_view_event-aggregate",873:"spa-aggregate",898:"ajax-aggregate"}[e]||e)+"."+{78:"42c392aa",147:"78efb4d5",193:"0938abd3",317:"0b4d6623",348:"a30a53ff",439:"8f89c105",729:"67423d16",786:"8cf0450e",862:"04af29e3",873:"19ebdf8d",898:"b0da4738"}[e]+"-1.231.0.min.js"),i.o=(e,t)=>Object.prototype.hasOwnProperty.call(e,t),e={},t="NRBA:",i.l=(r,n,o,a)=>{if(e[r])e[r].push(n);else{var s,c;if(void 0!==o)for(var u=document.getElementsByTagName("script"),d=0;d<u.length;d++){var f=u[d];if(f.getAttribute("src")==r||f.getAttribute("data-webpack")==t+o){s=f;break}}s||(c=!0,(s=document.createElement("script")).charset="utf-8",s.timeout=120,i.nc&&s.setAttribute("nonce",i.nc),s.setAttribute("data-webpack",t+o),s.src=r),e[r]=[n];var l=(t,n)=>{s.onerror=s.onload=null,clearTimeout(h);var i=e[r];if(delete e[r],s.parentNode&&s.parentNode.removeChild(s),i&&i.forEach((e=>e(n))),t)return t(n)},h=setTimeout(l.bind(null,void 0,{type:"timeout",target:s}),12e4);s.onerror=l.bind(null,s.onerror),s.onload=l.bind(null,s.onload),c&&document.head.appendChild(s)}},i.r=e=>{"undefined"!=typeof Symbol&&Symbol.toStringTag&&Object.defineProperty(e,Symbol.toStringTag,{value:"Module"}),Object.defineProperty(e,"__esModule",{value:!0})},i.p="https://js-agent.newrelic.com/",(()=>{var e={233:0,265:0};i.f.j=(t,r)=>{var n=i.o(e,t)?e[t]:void 0;if(0!==n)if(n)r.push(n[2]);else{var o=new Promise(((r,i)=>n=e[t]=[r,i]));r.push(n[2]=o);var a=i.p+i.u(t),s=new Error;i.l(a,(r=>{if(i.o(e,t)&&(0!==(n=e[t])&&(e[t]=void 0),n)){var o=r&&("load"===r.type?"missing":r.type),a=r&&r.target&&r.target.src;s.message="Loading chunk "+t+" failed.\n("+o+": "+a+")",s.name="ChunkLoadError",s.type=o,s.request=a,n[1](s)}}),"chunk-"+t,t)}};var t=(t,r)=>{var n,o,[a,s,c]=r,u=0;if(a.some((t=>0!==e[t]))){for(n in s)i.o(s,n)&&(i.m[n]=s[n]);if(c)c(i)}for(t&&t(r);u<a.length;u++)o=a[u],i.o(e,o)&&e[o]&&e[o][0](),e[o]=0},r=window.webpackChunkNRBA=window.webpackChunkNRBA||[];r.forEach(t.bind(null,0)),r.push=t.bind(null,r.push.bind(r))})();var o={};(()=>{"use strict";i.r(o);var e=i(2325),t=i(27);const r=Object.values(e.D);function n(e){const n={};return r.forEach((r=>{n[r]=function(e,r){return!1!==(0,t.Mt)(r,"".concat(e,".enabled"))}(r,e)})),n}var a=i(2384),s=i(909),c=i(9252),u=i(8768),d=i(4329),f=i(1509),l=i(2650),h=i(2374),g=i(8610);class p extends f.W{constructor(e,t,r){let n=!(arguments.length>3&&void 0!==arguments[3])||arguments[3];super(e,t,r),this.hasAggregator=!1,this.auto=n,this.abortHandler,n&&(0,d.R)(e,r)}importAggregator(){if(this.hasAggregator||!this.auto)return;this.hasAggregator=!0;const e=async()=>{try{const{lazyLoader:e}=await i.e(729).then(i.bind(i,8110)),{Aggregate:t}=await e(this.featureName,"aggregate");new t(this.agentIdentifier,this.aggregator)}catch(e){(0,g.Z)("Downloading ".concat(this.featureName," failed...")),this.abortHandler?.()}};h.v6?e():(0,l.b)((()=>e()),!0)}}var v=i(2484),m=i(2053);class b extends p{static featureName=v.t9;constructor(r,n){let i=!(arguments.length>2&&void 0!==arguments[2])||arguments[2];if(super(r,n,v.t9,i),("undefined"==typeof PerformanceNavigationTiming||u.T)&&"undefined"!=typeof PerformanceTiming){const n=(0,t.OP)(r);n[v.Dz]=Math.max(Date.now()-n.offset,0),(0,l.K)((()=>n[v.qw]=Math.max((0,m.z)()-n[v.Dz],0))),(0,l.b)((()=>{const t=(0,m.z)();n[v.OJ]=Math.max(t-n[v.Dz],0),(0,c.p)("timing",["load",t],void 0,e.D.pageViewTiming,this.ee)}))}this.importAggregator()}}var y=i(9557),w=i(7022);class E extends y.w{constructor(e){super(e),this.aggregatedData={}}store(e,t,r,n,i){var o=this.getBucket(e,t,r,i);return o.metrics=function(e,t){t||(t={count:0});return t.count+=1,(0,w.D)(e,(function(e,r){t[e]=A(r,t[e])})),t}(n,o.metrics),o}merge(e,t,r,n,i){var o=this.getBucket(e,t,n,i);if(o.metrics){var a=o.metrics;a.count+=r.count,(0,w.D)(r,(function(e,t){if("count"!==e){var n=a[e],i=r[e];i&&!i.c?a[e]=A(i.t,n):a[e]=function(e,t){if(!t)return e;t.c||(t=x(t.t));return t.min=Math.min(e.min,t.min),t.max=Math.max(e.max,t.max),t.t+=e.t,t.sos+=e.sos,t.c+=e.c,t}(i,a[e])}}))}else o.metrics=r}storeMetric(e,t,r,n){var i=this.getBucket(e,t,r);return i.stats=A(n,i.stats),i}getBucket(e,t,r,n){this.aggregatedData[e]||(this.aggregatedData[e]={});var i=this.aggregatedData[e][t];return i||(i=this.aggregatedData[e][t]={params:r||{}},n&&(i.custom=n)),i}get(e,t){return t?this.aggregatedData[e]&&this.aggregatedData[e][t]:this.aggregatedData[e]}take(e){for(var t={},r="",n=!1,i=0;i<e.length;i++)t[r=e[i]]=T(this.aggregatedData[r]),t[r].length&&(n=!0),delete this.aggregatedData[r];return n?t:null}}function A(e,t){return null==e?function(e){e?e.c++:e={c:1};return e}(t):t?(t.c||(t=x(t.t)),t.c+=1,t.t+=e,t.sos+=e*e,e>t.max&&(t.max=e),e<t.min&&(t.min=e),t):{t:e}}function x(e){return{t:e,min:e,max:e,sos:e*e,c:1}}function T(e){return"object"!=typeof e?[]:(0,w.D)(e,_)}function _(e,t){return t}var S=i(6797),D=i(5526),N=i(2438);var O=i(6998),j=i(8544),R=i(6382);class C extends p{static featureName=R.t;constructor(e,r){let n=!(arguments.length>2&&void 0!==arguments[2])||arguments[2];super(e,r,R.t,n),h.il&&((0,t.OP)(e).initHidden=Boolean("hidden"===document.visibilityState),(0,O.N)((()=>(0,c.p)("docHidden",[(0,m.z)()],void 0,R.t,this.ee)),!0),(0,j.bP)("pagehide",(()=>(0,c.p)("winPagehide",[(0,m.z)()],void 0,R.t,this.ee))),this.importAggregator())}}const I=Boolean(h._A?.Worker),P=Boolean(h._A?.SharedWorker),k=Boolean(h._A?.navigator?.serviceWorker);let H,L,z;var M=i(6034);class B extends p{static featureName=M.t9;constructor(t,r){let n=!(arguments.length>2&&void 0!==arguments[2])||arguments[2];super(t,r,M.t9,n),function(e){if(!H){if(I){H=Worker;try{h._A.Worker=r(H,"Dedicated")}catch(e){o(e,"Dedicated")}if(P){L=SharedWorker;try{h._A.SharedWorker=r(L,"Shared")}catch(e){o(e,"Shared")}}else n("Shared");if(k){z=navigator.serviceWorker.register;try{h._A.navigator.serviceWorker.register=(t=z,function(){for(var e=arguments.length,r=new Array(e),n=0;n<e;n++)r[n]=arguments[n];return i("Service",r[1]?.type),t.apply(navigator.serviceWorker,r)})}catch(e){o(e,"Service")}}else n("Service");var t;return}n("All")}function r(e,t){return"undefined"==typeof Proxy?e:new Proxy(e,{construct:(e,r)=>(i(t,r[1]?.type),new e(...r))})}function n(t){h.v6||e("Workers/".concat(t,"/Unavailable"))}function i(t,r){e("Workers/".concat(t,"module"===r?"/Module":"/Classic"))}function o(t,r){e("Workers/".concat(r,"/SM/Unsupported")),(0,g.Z)("NR Agent: Unable to capture ".concat(r," workers."),t)}}((t=>(0,c.p)(M.xS,[t],void 0,e.D.metrics,this.ee))),this.importAggregator()}}var F=i(3916),U=i(3752),q=i(8683),G=i.n(q);const W="nr@original";var V=Object.prototype.hasOwnProperty,X=!1;function Z(e,t){return e||(e=U.ee),r.inPlace=function(e,t,n,i,o){n||(n="");var a,s,c,u="-"===n.charAt(0);for(c=0;c<t.length;c++)Q(a=e[s=t[c]])||(e[s]=r(a,u?s+n:n,i,s,o))},r.flag=W,r;function r(t,r,i,o,a){return Q(t)?t:(r||(r=""),nrWrapper[W]=t,Y(t,nrWrapper,e),nrWrapper);function nrWrapper(){var s,c,u,d;try{c=this,s=G()(arguments),u="function"==typeof i?i(s,c):i||{}}catch(t){$([t,"",[s,c,o],u],e)}n(r+"start",[s,c,o],u,a);try{return d=t.apply(c,s)}catch(e){throw n(r+"err",[s,c,e],u,a),e}finally{n(r+"end",[s,c,d],u,a)}}}function n(r,n,i,o){if(!X||t){var a=X;X=!0;try{e.emit(r,n,i,t,o)}catch(t){$([t,r,n,i],e)}X=a}}}function $(e,t){t||(t=U.ee);try{t.emit("internal-error",e)}catch(e){}}function Y(e,t,r){if(Object.defineProperty&&Object.keys)try{return Object.keys(e).forEach((function(r){Object.defineProperty(t,r,{get:function(){return e[r]},set:function(t){return e[r]=t,t}})})),t}catch(e){$([e],r)}for(var n in e)V.call(e,n)&&(t[n]=e[n]);return t}function Q(e){return!(e&&e instanceof Function&&e.apply&&!e[W])}const J={},K=XMLHttpRequest,ee="addEventListener",te="removeEventListener";function re(e){var t=function(e){return(e||U.ee).get("events")}(e);if(J[t.debugId]++)return t;J[t.debugId]=1;var r=Z(t,!0);function n(e){r.inPlace(e,[ee,te],"-",i)}function i(e,t){return e[1]}return"getPrototypeOf"in Object&&(h.il&&ne(document,n),ne(h._A,n),ne(K.prototype,n)),t.on(ee+"-start",(function(e,t){var n=e[1];if(null!==n&&("function"==typeof n||"object"==typeof n)){var i=(0,F.X)(n,"nr@wrapped",(function(){var e={object:function(){if("function"!=typeof n.handleEvent)return;return n.handleEvent.apply(n,arguments)},function:n}[typeof n];return e?r(e,"fn-",null,e.name||"anonymous"):n}));this.wrapped=e[1]=i}})),t.on(te+"-start",(function(e){e[1]=this.wrapped||e[1]})),t}function ne(e,t){let r=e;for(;"object"==typeof r&&!Object.prototype.hasOwnProperty.call(r,ee);)r=Object.getPrototypeOf(r);for(var n=arguments.length,i=new Array(n>2?n-2:0),o=2;o<n;o++)i[o-2]=arguments[o];r&&t(r,...i)}var ie="fetch-",oe=ie+"body-",ae=["arrayBuffer","blob","json","text","formData"],se=h._A.Request,ce=h._A.Response,ue="prototype",de="nr@context";const fe={};function le(e){const t=function(e){return(e||U.ee).get("fetch")}(e);if(!(se&&ce&&h._A.fetch))return t;if(fe[t.debugId]++)return t;function r(e,r,n){var i=e[r];"function"==typeof i&&(e[r]=function(){var e,r=G()(arguments),o={};t.emit(n+"before-start",[r],o),o[de]&&o[de].dt&&(e=o[de].dt);var a=i.apply(this,r);return t.emit(n+"start",[r,e],a),a.then((function(e){return t.emit(n+"end",[null,e],a),e}),(function(e){throw t.emit(n+"end",[e],a),e}))})}return fe[t.debugId]=1,ae.forEach((e=>{r(se[ue],e,oe),r(ce[ue],e,oe)})),r(h._A,"fetch",ie),t.on(ie+"end",(function(e,r){var n=this;if(r){var i=r.headers.get("content-length");null!==i&&(n.rxSize=i),t.emit(ie+"done",[null,r],n)}else t.emit(ie+"done",[e],n)})),t}const he={},ge=["pushState","replaceState"];function pe(e){const t=function(e){return(e||U.ee).get("history")}(e);return!h.il||he[t.debugId]++||(he[t.debugId]=1,Z(t).inPlace(window.history,ge,"-")),t}const ve={},me=["appendChild","insertBefore","replaceChild"];function be(e){const t=function(e){return(e||U.ee).get("jsonp")}(e);if(!h.il||ve[t.debugId])return t;ve[t.debugId]=!0;var r=Z(t),n=/[?&](?:callback|cb)=([^&#]+)/,i=/(.*)\.([^.]+)/,o=/^(\w+)(\.|$)(.*)$/;function a(e,t){var r=e.match(o),n=r[1],i=r[3];return i?a(i,t[n]):t[n]}return r.inPlace(Node.prototype,me,"dom-"),t.on("dom-start",(function(e){!function(e){if(!e||"string"!=typeof e.nodeName||"script"!==e.nodeName.toLowerCase())return;if("function"!=typeof e.addEventListener)return;var o=(s=e.src,c=s.match(n),c?c[1]:null);var s,c;if(!o)return;var u=function(e){var t=e.match(i);if(t&&t.length>=3)return{key:t[2],parent:a(t[1],window)};return{key:e,parent:window}}(o);if("function"!=typeof u.parent[u.key])return;var d={};function f(){t.emit("jsonp-end",[],d),e.removeEventListener("load",f,(0,j.m$)(!1)),e.removeEventListener("error",l,(0,j.m$)(!1))}function l(){t.emit("jsonp-error",[],d),t.emit("jsonp-end",[],d),e.removeEventListener("load",f,(0,j.m$)(!1)),e.removeEventListener("error",l,(0,j.m$)(!1))}r.inPlace(u.parent,[u.key],"cb-",d),e.addEventListener("load",f,(0,j.m$)(!1)),e.addEventListener("error",l,(0,j.m$)(!1)),t.emit("new-jsonp",[e.src],d)}(e[0])})),t}const ye={};function we(e){const r=function(e){return(e||U.ee).get("mutation")}(e);if(!h.il||ye[r.debugId])return r;ye[r.debugId]=!0;var n=Z(r),i=t.Yu.MO;return i&&(window.MutationObserver=function(e){return this instanceof i?new i(n(e,"fn-")):i.apply(this,arguments)},MutationObserver.prototype=i.prototype),r}const Ee={};function Ae(e){const r=function(e){return(e||U.ee).get("promise")}(e);if(Ee[r.debugId])return r;Ee[r.debugId]=!0;var n=U.c,i=Z(r),o=t.Yu.PR;return o&&function(){function e(t){var n=r.context(),a=i(t,"executor-",n,null,!1);const s=Reflect.construct(o,[a],e);return r.context(s).getCtx=function(){return n},s}h._A.Promise=e,Object.defineProperty(e,"name",{value:"Promise"}),e.toString=function(){return o.toString()},Object.setPrototypeOf(e,o),["all","race"].forEach((function(t){const n=o[t];e[t]=function(e){let i=!1;Array.from(e||[]).forEach((e=>{this.resolve(e).then(a("all"===t),a(!1))}));const o=n.apply(this,arguments);return o;function a(e){return function(){r.emit("propagate",[null,!i],o,!1,!1),i=i||!e}}}})),["resolve","reject"].forEach((function(t){const n=o[t];e[t]=function(e){const t=n.apply(this,arguments);return e!==t&&r.emit("propagate",[e,!0],t,!1,!1),t}})),e.prototype=o.prototype;const t=o.prototype.then;o.prototype.then=function(){var e=this,o=n(e);o.promise=e;for(var a=arguments.length,s=new Array(a),c=0;c<a;c++)s[c]=arguments[c];s[0]=i(s[0],"cb-",o,null,!1),s[1]=i(s[1],"cb-",o,null,!1);const u=t.apply(this,s);return o.nextPromise=u,r.emit("propagate",[e,!0],u,!1,!1),u},o.prototype.then[W]=t,r.on("executor-start",(function(e){e[0]=i(e[0],"resolve-",this,null,!1),e[1]=i(e[1],"resolve-",this,null,!1)})),r.on("executor-err",(function(e,t,r){e[1](r)})),r.on("cb-end",(function(e,t,n){r.emit("propagate",[n,!0],this.nextPromise,!1,!1)})),r.on("propagate",(function(e,t,n){this.getCtx&&!t||(this.getCtx=function(){if(e instanceof Promise)var t=r.context(e);return t&&t.getCtx?t.getCtx():this})}))}(),r}const xe={};function Te(e){const t=function(e){return(e||U.ee).get("raf")}(e);if(!h.il||xe[t.debugId]++)return t;xe[t.debugId]=1;var r=Z(t);return r.inPlace(window,["requestAnimationFrame"],"raf-"),t.on("raf-start",(function(e){e[0]=r(e[0],"fn-")})),t}const _e={},Se="setTimeout",De="setInterval",Ne="clearTimeout",Oe="-start",je=[Se,"setImmediate",De,Ne,"clearImmediate"];function Re(e){const t=function(e){return(e||U.ee).get("timer")}(e);if(_e[t.debugId]++)return t;_e[t.debugId]=1;var r=Z(t);return r.inPlace(h._A,je.slice(0,2),Se+"-"),r.inPlace(h._A,je.slice(2,3),De+"-"),r.inPlace(h._A,je.slice(3),Ne+"-"),t.on(De+Oe,(function(e,t,n){e[0]=r(e[0],"fn-",null,n)})),t.on(Se+Oe,(function(e,t,n){this.method=n,this.timerDuration=isNaN(e[1])?0:+e[1],e[0]=r(e[0],"fn-",this,n)})),t}const Ce={},Ie=["open","send"];function Pe(e){var r=e||U.ee;const n=function(e){return(e||U.ee).get("xhr")}(r);if(Ce[n.debugId]++)return n;Ce[n.debugId]=1,re(r);var i=Z(n),o=t.Yu.XHR,a=t.Yu.MO,s=t.Yu.PR,c=t.Yu.SI,u="readystatechange",d=["onload","onerror","onabort","onloadstart","onloadend","onprogress","ontimeout"],f=[],l=h._A.XMLHttpRequest.listeners,p=h._A.XMLHttpRequest=function(e){var t=new o(e);function r(){try{n.emit("new-xhr",[t],t),t.addEventListener(u,m,(0,j.m$)(!1))}catch(e){(0,g.Z)("An error occured while intercepting XHR",e);try{n.emit("internal-error",[e])}catch(e){}}}return this.listeners=l?[...l,r]:[r],this.listeners.forEach((e=>e())),t};function v(e,t){i.inPlace(t,["onreadystatechange"],"fn-",A)}function m(){var e=this,t=n.context(e);e.readyState>3&&!t.resolved&&(t.resolved=!0,n.emit("xhr-resolved",[],e)),i.inPlace(e,d,"fn-",A)}if(function(e,t){for(var r in e)t[r]=e[r]}(o,p),p.prototype=o.prototype,i.inPlace(p.prototype,Ie,"-xhr-",A),n.on("send-xhr-start",(function(e,t){v(e,t),function(e){f.push(e),a&&(b?b.then(E):c?c(E):(y=-y,w.data=y))}(t)})),n.on("open-xhr-start",v),a){var b=s&&s.resolve();if(!c&&!s){var y=1,w=document.createTextNode(y);new a(E).observe(w,{characterData:!0})}}else r.on("fn-end",(function(e){e[0]&&e[0].type===u||E()}));function E(){for(var e=0;e<f.length;e++)v(0,f[e]);f.length&&(f=[])}function A(e,t){return t}return n}var ke,He={};try{ke=localStorage.getItem("__nr_flags").split(","),console&&"function"==typeof console.log&&(He.console=!0,-1!==ke.indexOf("dev")&&(He.dev=!0),-1!==ke.indexOf("nr_dev")&&(He.nrDev=!0))}catch(e){}function Le(e){try{He.console&&Le(e)}catch(e){}}He.nrDev&&U.ee.on("internal-error",(function(e){Le(e.stack)})),He.dev&&U.ee.on("fn-err",(function(e,t,r){Le(r.stack)})),He.dev&&(Le("NR AGENT IN DEVELOPMENT MODE"),Le("flags: "+(0,w.D)(He,(function(e,t){return e})).join(", ")));var ze=i(8322);class Me extends p{static featureName=ze.t;constructor(r,n){let i=!(arguments.length>2&&void 0!==arguments[2])||arguments[2];super(r,n,ze.t,i),this.skipNext=0;try{this.removeOnAbort=new AbortController}catch(e){}const o=this;o.ee.on("fn-start",(function(e,t,r){o.abortHandler&&(o.skipNext+=1)})),o.ee.on("fn-err",(function(e,t,r){o.abortHandler&&!r[ze.A]&&((0,F.X)(r,ze.A,(function(){return!0})),this.thrown=!0,Fe(r,void 0,o.ee))})),o.ee.on("fn-end",(function(){o.abortHandler&&!this.thrown&&o.skipNext>0&&(o.skipNext-=1)})),o.ee.on("internal-error",(function(t){(0,c.p)("ierr",[t,(0,m.z)(),!0],void 0,e.D.jserrors,o.ee)})),this.origOnerror=h._A.onerror,h._A.onerror=this.onerrorHandler.bind(this),h._A.addEventListener("unhandledrejection",(t=>{const r=function(e){let t="Unhandled Promise Rejection: ";if(e instanceof Error)try{return e.message=t+e.message,e}catch(t){return e}if(void 0===e)return new Error(t);try{return new Error(t+(0,N.P)(e))}catch(e){return new Error(t)}}(t.reason);(0,c.p)("err",[r,(0,m.z)(),!1,{unhandledPromiseRejection:1}],void 0,e.D.jserrors,this.ee)}),(0,j.m$)(!1,this.removeOnAbort?.signal)),Te(this.ee),Re(this.ee),re(this.ee),(0,t.OP)(r).xhrWrappable&&Pe(this.ee),this.abortHandler=this.#e,this.importAggregator()}#e(){this.removeOnAbort?.abort(),this.abortHandler=void 0}onerrorHandler(t,r,n,i,o){"function"==typeof this.origOnerror&&this.origOnerror(...arguments);try{this.skipNext?this.skipNext-=1:Fe(o||new Be(t,r,n),!0,this.ee)}catch(t){try{(0,c.p)("ierr",[t,(0,m.z)(),!0],void 0,e.D.jserrors,this.ee)}catch(e){}}return!1}}function Be(e,t,r){this.message=e||"Uncaught error with no additional information",this.sourceURL=t,this.line=r}function Fe(t,r,n){var i=r?null:(0,m.z)();(0,c.p)("err",[t,i],void 0,e.D.jserrors,n)}let Ue=1;const qe="nr@id";function Ge(e){const t=typeof e;return!e||"object"!==t&&"function"!==t?-1:e===h._A?0:(0,F.X)(e,qe,(function(){return Ue++}))}var We=i(9071);function Ve(e){if("string"==typeof e&&e.length)return e.length;if("object"==typeof e){if("undefined"!=typeof ArrayBuffer&&e instanceof ArrayBuffer&&e.byteLength)return e.byteLength;if("undefined"!=typeof Blob&&e instanceof Blob&&e.size)return e.size;if(!("undefined"!=typeof FormData&&e instanceof FormData))try{return(0,N.P)(e).length}catch(e){return}}}var Xe=i(6368);class Ze{constructor(e){this.agentIdentifier=e,this.generateTracePayload=this.generateTracePayload.bind(this),this.shouldGenerateTrace=this.shouldGenerateTrace.bind(this)}generateTracePayload(e){if(!this.shouldGenerateTrace(e))return null;var r=(0,t.DL)(this.agentIdentifier);if(!r)return null;var n=(r.accountID||"").toString()||null,i=(r.agentID||"").toString()||null,o=(r.trustKey||"").toString()||null;if(!n||!i)return null;var a=(0,D.M)(),s=(0,D.Ht)(),c=Date.now(),u={spanId:a,traceId:s,timestamp:c};return(e.sameOrigin||this.isAllowedOrigin(e)&&this.useTraceContextHeadersForCors())&&(u.traceContextParentHeader=this.generateTraceContextParentHeader(a,s),u.traceContextStateHeader=this.generateTraceContextStateHeader(a,c,n,i,o)),(e.sameOrigin&&!this.excludeNewrelicHeader()||!e.sameOrigin&&this.isAllowedOrigin(e)&&this.useNewrelicHeaderForCors())&&(u.newrelicHeader=this.generateTraceHeader(a,s,c,n,i,o)),u}generateTraceContextParentHeader(e,t){return"00-"+t+"-"+e+"-01"}generateTraceContextStateHeader(e,t,r,n,i){return i+"@nr=0-1-"+r+"-"+n+"-"+e+"----"+t}generateTraceHeader(e,t,r,n,i,o){if(!("function"==typeof h._A?.btoa))return null;var a={v:[0,1],d:{ty:"Browser",ac:n,ap:i,id:e,tr:t,ti:r}};return o&&n!==o&&(a.d.tk=o),btoa((0,N.P)(a))}shouldGenerateTrace(e){return this.isDtEnabled()&&this.isAllowedOrigin(e)}isAllowedOrigin(e){var r=!1,n={};if((0,t.Mt)(this.agentIdentifier,"distributed_tracing")&&(n=(0,t.P_)(this.agentIdentifier).distributed_tracing),e.sameOrigin)r=!0;else if(n.allowed_origins instanceof Array)for(var i=0;i<n.allowed_origins.length;i++){var o=(0,Xe.e)(n.allowed_origins[i]);if(e.hostname===o.hostname&&e.protocol===o.protocol&&e.port===o.port){r=!0;break}}return r}isDtEnabled(){var e=(0,t.Mt)(this.agentIdentifier,"distributed_tracing");return!!e&&!!e.enabled}excludeNewrelicHeader(){var e=(0,t.Mt)(this.agentIdentifier,"distributed_tracing");return!!e&&!!e.exclude_newrelic_header}useNewrelicHeaderForCors(){var e=(0,t.Mt)(this.agentIdentifier,"distributed_tracing");return!!e&&!1!==e.cors_use_newrelic_header}useTraceContextHeadersForCors(){var e=(0,t.Mt)(this.agentIdentifier,"distributed_tracing");return!!e&&!!e.cors_use_tracecontext_headers}}var $e=i(8675),Ye=["load","error","abort","timeout"],Qe=Ye.length,Je=t.Yu.REQ,Ke=h._A.XMLHttpRequest;class et extends p{static featureName=$e.t;constructor(r,n){let i=!(arguments.length>2&&void 0!==arguments[2])||arguments[2];super(r,n,$e.t,i),(0,t.OP)(r).xhrWrappable&&(this.dt=new Ze(r),this.handler=(e,t,r,n)=>(0,c.p)(e,t,r,n,this.ee),le(this.ee),Pe(this.ee),function(r,n,i,o){function a(e){var t=this;t.totalCbs=0,t.called=0,t.cbTime=0,t.end=x,t.ended=!1,t.xhrGuids={},t.lastSize=null,t.loadCaptureCalled=!1,t.params=this.params||{},t.metrics=this.metrics||{},e.addEventListener("load",(function(r){_(t,e)}),(0,j.m$)(!1)),We.I||e.addEventListener("progress",(function(e){t.lastSize=e.loaded}),(0,j.m$)(!1))}function s(e){this.params={method:e[0]},T(this,e[1]),this.metrics={}}function c(e,n){var i=(0,t.DL)(r);"xpid"in i&&this.sameOrigin&&n.setRequestHeader("X-NewRelic-ID",i.xpid);var a=o.generateTracePayload(this.parsedOrigin);if(a){var s=!1;a.newrelicHeader&&(n.setRequestHeader("newrelic",a.newrelicHeader),s=!0),a.traceContextParentHeader&&(n.setRequestHeader("traceparent",a.traceContextParentHeader),a.traceContextStateHeader&&n.setRequestHeader("tracestate",a.traceContextStateHeader),s=!0),s&&(this.dt=a)}}function u(e,t){var r=this.metrics,i=e[0],o=this;if(r&&i){var a=Ve(i);a&&(r.txSize=a)}this.startTime=(0,m.z)(),this.listener=function(e){try{"abort"!==e.type||o.loadCaptureCalled||(o.params.aborted=!0),("load"!==e.type||o.called===o.totalCbs&&(o.onloadCalled||"function"!=typeof t.onload)&&"function"==typeof o.end)&&o.end(t)}catch(e){try{n.emit("internal-error",[e])}catch(e){}}};for(var s=0;s<Qe;s++)t.addEventListener(Ye[s],this.listener,(0,j.m$)(!1))}function d(e,t,r){this.cbTime+=e,t?this.onloadCalled=!0:this.called+=1,this.called!==this.totalCbs||!this.onloadCalled&&"function"==typeof r.onload||"function"!=typeof this.end||this.end(r)}function f(e,t){var r=""+Ge(e)+!!t;this.xhrGuids&&!this.xhrGuids[r]&&(this.xhrGuids[r]=!0,this.totalCbs+=1)}function l(e,t){var r=""+Ge(e)+!!t;this.xhrGuids&&this.xhrGuids[r]&&(delete this.xhrGuids[r],this.totalCbs-=1)}function g(){this.endTime=(0,m.z)()}function p(e,t){t instanceof Ke&&"load"===e[0]&&n.emit("xhr-load-added",[e[1],e[2]],t)}function v(e,t){t instanceof Ke&&"load"===e[0]&&n.emit("xhr-load-removed",[e[1],e[2]],t)}function b(e,t,r){t instanceof Ke&&("onload"===r&&(this.onload=!0),("load"===(e[0]&&e[0].type)||this.onload)&&(this.xhrCbStart=(0,m.z)()))}function y(e,t){this.xhrCbStart&&n.emit("xhr-cb-time",[(0,m.z)()-this.xhrCbStart,this.onload,t],t)}function w(e){var t,r=e[1]||{};"string"==typeof e[0]?t=e[0]:e[0]&&e[0].url?t=e[0].url:h._A?.URL&&e[0]&&e[0]instanceof URL&&(t=e[0].href),t&&(this.parsedOrigin=(0,Xe.e)(t),this.sameOrigin=this.parsedOrigin.sameOrigin);var n=o.generateTracePayload(this.parsedOrigin);if(n&&(n.newrelicHeader||n.traceContextParentHeader))if("string"==typeof e[0]||h._A?.URL&&e[0]&&e[0]instanceof URL){var i={};for(var a in r)i[a]=r[a];i.headers=new Headers(r.headers||{}),s(i.headers,n)&&(this.dt=n),e.length>1?e[1]=i:e.push(i)}else e[0]&&e[0].headers&&s(e[0].headers,n)&&(this.dt=n);function s(e,t){var r=!1;return t.newrelicHeader&&(e.set("newrelic",t.newrelicHeader),r=!0),t.traceContextParentHeader&&(e.set("traceparent",t.traceContextParentHeader),t.traceContextStateHeader&&e.set("tracestate",t.traceContextStateHeader),r=!0),r}}function E(e,t){this.params={},this.metrics={},this.startTime=(0,m.z)(),this.dt=t,e.length>=1&&(this.target=e[0]),e.length>=2&&(this.opts=e[1]);var r,n=this.opts||{},i=this.target;"string"==typeof i?r=i:"object"==typeof i&&i instanceof Je?r=i.url:h._A?.URL&&"object"==typeof i&&i instanceof URL&&(r=i.href),T(this,r);var o=(""+(i&&i instanceof Je&&i.method||n.method||"GET")).toUpperCase();this.params.method=o,this.txSize=Ve(n.body)||0}function A(t,r){var n;this.endTime=(0,m.z)(),this.params||(this.params={}),this.params.status=r?r.status:0,"string"==typeof this.rxSize&&this.rxSize.length>0&&(n=+this.rxSize);var o={txSize:this.txSize,rxSize:n,duration:(0,m.z)()-this.startTime};i("xhr",[this.params,o,this.startTime,this.endTime,"fetch"],this,e.D.ajax)}function x(t){var r=this.params,n=this.metrics;if(!this.ended){this.ended=!0;for(var o=0;o<Qe;o++)t.removeEventListener(Ye[o],this.listener,!1);r.aborted||(n.duration=(0,m.z)()-this.startTime,this.loadCaptureCalled||4!==t.readyState?null==r.status&&(r.status=0):_(this,t),n.cbTime=this.cbTime,i("xhr",[r,n,this.startTime,this.endTime,"xhr"],this,e.D.ajax))}}function T(e,t){var r=(0,Xe.e)(t),n=e.params;n.hostname=r.hostname,n.port=r.port,n.protocol=r.protocol,n.host=r.hostname+":"+r.port,n.pathname=r.pathname,e.parsedOrigin=r,e.sameOrigin=r.sameOrigin}function _(e,t){e.params.status=t.status;var r=function(e,t){var r=e.responseType;return"json"===r&&null!==t?t:"arraybuffer"===r||"blob"===r||"json"===r?Ve(e.response):"text"===r||""===r||void 0===r?Ve(e.responseText):void 0}(t,e.lastSize);if(r&&(e.metrics.rxSize=r),e.sameOrigin){var n=t.getResponseHeader("X-NewRelic-App-Data");n&&(e.params.cat=n.split(", ").pop())}e.loadCaptureCalled=!0}n.on("new-xhr",a),n.on("open-xhr-start",s),n.on("open-xhr-end",c),n.on("send-xhr-start",u),n.on("xhr-cb-time",d),n.on("xhr-load-added",f),n.on("xhr-load-removed",l),n.on("xhr-resolved",g),n.on("addEventListener-end",p),n.on("removeEventListener-end",v),n.on("fn-end",y),n.on("fetch-before-start",w),n.on("fetch-start",E),n.on("fn-start",b),n.on("fetch-done",A)}(r,this.ee,this.handler,this.dt),this.importAggregator())}}var tt=i(6408),rt=i(2628);const{BST_RESOURCE:nt,BST_TIMER:it,END:ot,FEATURE_NAME:at,FN_END:st,FN_START:ct,ADD_EVENT_LISTENER:ut,PUSH_STATE:dt,RESOURCE:ft,RESOURCE_TIMING_BUFFER_FULL:lt,START:ht,ORIG_EVENT:gt}=rt,pt="clearResourceTimings";var vt=i(755);const{FEATURE_NAME:mt,START:bt,END:yt,BODY:wt,CB_END:Et,JS_TIME:At,FETCH:xt,FN_START:Tt,CB_START:_t,FN_END:St}=vt;var Dt=i(6486);class Nt extends p{static featureName=Dt.t;constructor(e,t){let r=!(arguments.length>2&&void 0!==arguments[2])||arguments[2];super(e,t,Dt.t,r),this.importAggregator()}}new class{constructor(e){let t=arguments.length>1&&void 0!==arguments[1]?arguments[1]:(0,D.ky)(16);this.agentIdentifier=t,this.sharedAggregator=new E({agentIdentifier:this.agentIdentifier}),this.features={},this.desiredFeatures=new Set(e.features||[]),this.desiredFeatures.add(b),Object.assign(this,(0,a.j)(this.agentIdentifier,e,e.loaderType||"agent")),this.start()}get config(){return{info:(0,t.C5)(this.agentIdentifier),init:(0,t.P_)(this.agentIdentifier),loader_config:(0,t.DL)(this.agentIdentifier),runtime:(0,t.OP)(this.agentIdentifier)}}start(){const t="features";try{const r=n(this.agentIdentifier),i=Array.from(this.desiredFeatures);i.sort(((t,r)=>e.p[t.featureName]-e.p[r.featureName])),i.forEach((t=>{if(r[t.featureName]||t.featureName===e.D.pageViewEvent){const e=(0,s.Z)(t.featureName);e.every((e=>r[e]))||(0,g.Z)("".concat(t.featureName," is enabled but one or more dependent features has been disabled (").concat((0,N.P)(e),"). This may cause unintended consequences or missing data...")),this.features[t.featureName]=new t(this.agentIdentifier,this.sharedAggregator)}})),(0,S.Qy)(this.agentIdentifier,this.features,t)}catch(e){(0,g.Z)("Failed to initialize all enabled instrument classes (agent aborted) -",e);for(const e in this.features)this.features[e].abortHandler?.();const r=(0,S.fP)();return delete r.initializedAgents[this.agentIdentifier]?.api,delete r.initializedAgents[this.agentIdentifier]?.[t],delete this.sharedAggregator,r.ee?.abort(),delete r.ee?.get(this.agentIdentifier),!1}}}({features:[et,b,C,class extends p{static featureName=at;constructor(t,r){if(super(t,r,at,!(arguments.length>2&&void 0!==arguments[2])||arguments[2]),!h.il)return;const n=this.ee;this.timerEE=Re(n),this.rafEE=Te(n),pe(n),re(n),n.on(ct,(function(e,t){e[0]instanceof gt&&(this.bstStart=(0,m.z)())})),n.on(st,(function(t,r){var i=t[0];i instanceof gt&&(0,c.p)("bst",[i,r,this.bstStart,(0,m.z)()],void 0,e.D.sessionTrace,n)})),this.timerEE.on(ct,(function(e,t,r){this.bstStart=(0,m.z)(),this.bstType=r})),this.timerEE.on(st,(function(t,r){(0,c.p)(it,[r,this.bstStart,(0,m.z)(),this.bstType],void 0,e.D.sessionTrace,n)})),this.rafEE.on(ct,(function(){this.bstStart=(0,m.z)()})),this.rafEE.on(st,(function(t,r){(0,c.p)(it,[r,this.bstStart,(0,m.z)(),"requestAnimationFrame"],void 0,e.D.sessionTrace,n)})),n.on(dt+ht,(function(e){this.time=(0,m.z)(),this.startPath=location.pathname+location.hash})),n.on(dt+ot,(function(t){(0,c.p)("bstHist",[location.pathname+location.hash,this.startPath,this.time],void 0,e.D.sessionTrace,n)})),(0,tt.W)()?((0,c.p)(nt,[window.performance.getEntriesByType("resource")],void 0,e.D.sessionTrace,n),function(){var t=new PerformanceObserver(((t,r)=>{var i=t.getEntries();(0,c.p)(nt,[i],void 0,e.D.sessionTrace,n)}));try{t.observe({entryTypes:["resource"]})}catch(e){}}()):window.performance[pt]&&window.performance[ut]&&window.performance.addEventListener(lt,this.onResourceTimingBufferFull,(0,j.m$)(!1)),document.addEventListener("scroll",this.noOp,(0,j.m$)(!1)),document.addEventListener("keypress",this.noOp,(0,j.m$)(!1)),document.addEventListener("click",this.noOp,(0,j.m$)(!1)),this.abortHandler=this.#e,this.importAggregator()}#e(){window.performance.removeEventListener(lt,this.onResourceTimingBufferFull,!1),this.abortHandler=void 0}noOp(e){}onResourceTimingBufferFull(t){if((0,c.p)(nt,[window.performance.getEntriesByType(ft)],void 0,e.D.sessionTrace,this.ee),window.performance[pt])try{window.performance.removeEventListener(lt,this.onResourceTimingBufferFull,!1)}catch(e){}}},B,Nt,Me,class extends p{static featureName=mt;constructor(e,r){if(super(e,r,mt,!(arguments.length>2&&void 0!==arguments[2])||arguments[2]),!h.il)return;if(!(0,t.OP)(e).xhrWrappable)return;try{this.removeOnAbort=new AbortController}catch(e){}let n,i=0;const o=this.ee.get("tracer"),a=be(this.ee),s=Ae(this.ee),c=Re(this.ee),u=Pe(this.ee),d=this.ee.get("events"),f=le(this.ee),l=pe(this.ee),g=we(this.ee);function p(e,t){l.emit("newURL",[""+window.location,t])}function v(){i++,n=window.location.hash,this[Tt]=(0,m.z)()}function b(){i--,window.location.hash!==n&&p(0,!0);var e=(0,m.z)();this[At]=~~this[At]+e-this[Tt],this[St]=e}function y(e,t){e.on(t,(function(){this[t]=(0,m.z)()}))}this.ee.on(Tt,v),s.on(_t,v),a.on(_t,v),this.ee.on(St,b),s.on(Et,b),a.on(Et,b),this.ee.buffer([Tt,St,"xhr-resolved"],this.featureName),d.buffer([Tt],this.featureName),c.buffer(["setTimeout"+yt,"clearTimeout"+bt,Tt],this.featureName),u.buffer([Tt,"new-xhr","send-xhr"+bt],this.featureName),f.buffer([xt+bt,xt+"-done",xt+wt+bt,xt+wt+yt],this.featureName),l.buffer(["newURL"],this.featureName),g.buffer([Tt],this.featureName),s.buffer(["propagate",_t,Et,"executor-err","resolve"+bt],this.featureName),o.buffer([Tt,"no-"+Tt],this.featureName),a.buffer(["new-jsonp","cb-start","jsonp-error","jsonp-end"],this.featureName),y(f,xt+bt),y(f,xt+"-done"),y(a,"new-jsonp"),y(a,"jsonp-end"),y(a,"cb-start"),l.on("pushState-end",p),l.on("replaceState-end",p),window.addEventListener("hashchange",p,(0,j.m$)(!0,this.removeOnAbort?.signal)),window.addEventListener("load",p,(0,j.m$)(!0,this.removeOnAbort?.signal)),window.addEventListener("popstate",(function(){p(0,i>1)}),(0,j.m$)(!0,this.removeOnAbort?.signal)),this.abortHandler=this.#e,this.importAggregator()}#e(){this.removeOnAbort?.abort(),this.abortHandler=void 0}}],loaderType:"spa"})})(),window.NRBA=o})();
	</script>`
	return template.HTML(script), nil

}
