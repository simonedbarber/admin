package admin

import (
	"database/sql"
	"fmt"
	"reflect"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/simonedbarber/qor"
	"github.com/simonedbarber/qor/resource"
	"github.com/simonedbarber/qor/utils"
	"gorm.io/gorm"
	"gorm.io/gorm/schema"
)

// filterRegexp used to parse url query to get filters
var filterRegexp = regexp.MustCompile(`^filters\[(.*?)\]`)

// PaginationPageCount default pagination page count
var PaginationPageCount = 20

// Pagination is used to hold pagination related information when rendering tables
type Pagination struct {
	Total       int
	Pages       int
	CurrentPage int
	PerPage     int
}

// Searcher is used to search results
type Searcher struct {
	*Context
	scopes     []*Scope
	filters    map[*Filter]*resource.MetaValues
	Pagination Pagination
}

func (s *Searcher) clone() *Searcher {
	return &Searcher{Context: s.Context, scopes: s.scopes, filters: s.filters}
}

// Page set current page, if current page equal -1, then show all records
func (s *Searcher) Page(num int) *Searcher {
	s.Pagination.CurrentPage = num
	return s
}

// PerPage set pre page count
func (s *Searcher) PerPage(num int) *Searcher {
	s.Pagination.PerPage = num
	return s
}

// Scope filter with defined scopes
func (s *Searcher) Scope(names ...string) *Searcher {
	newSearcher := s.clone()
	for _, name := range names {
		for _, scope := range s.Resource.scopes {
			if scope.Name == name && !scope.Default {
				newSearcher.scopes = append(newSearcher.scopes, scope)
				break
			}
		}
	}
	return newSearcher
}

// Filter filter with defined filters, filter with columns value
func (s *Searcher) Filter(filter *Filter, values *resource.MetaValues) *Searcher {
	newSearcher := s.clone()
	if newSearcher.filters == nil {
		newSearcher.filters = map[*Filter]*resource.MetaValues{}
	}
	newSearcher.filters[filter] = values
	return newSearcher
}

// FindOne find one record based on current conditions
func (s *Searcher) FindOne() (interface{}, error) {
	var (
		err     error
		context = s.parseContext(false)
		result  = s.Resource.NewStruct()
	)

	if context.HasError() {
		return result, context.Errors
	}

	err = s.Resource.CallFindOne(result, nil, context)
	return result, err
}

// FindMany find many records based on current conditions
func (s *Searcher) FindMany() (interface{}, error) {
	var (
		err     error
		context = s.parseContext(true)
		result  = s.Resource.NewSlice()
	)

	if context.HasError() {
		return result, context.Errors
	}

	err = s.Resource.CallFindMany(result, context)
	return result, err
}

// filterData filter data by scopes, filters, order by and keyword
func (s *Searcher) filterData(context *qor.Context, withDefaultScope bool) *qor.Context {
	db := context.GetDB()

	// call default scopes
	if withDefaultScope {
		for _, scope := range s.Resource.scopes {
			if scope.Default {
				filterWithThisScope := true

				if scope.Group != "" {
					for _, s := range s.scopes {
						if s.Group == scope.Group {
							filterWithThisScope = false
							break
						}
					}
				}

				if filterWithThisScope {
					db = scope.Handler(db, context)
				}
			}
		}
	}

	// call scopes
	for _, scope := range s.scopes {
		db = scope.Handler(db, context)
	}

	// call filters
	if s.filters != nil {
		for filter, value := range s.filters {
			if filter.Handler != nil {
				filterArgument := &FilterArgument{
					Value:    value,
					Context:  context,
					Resource: s.Resource,
				}
				db = filter.Handler(db, filterArgument)
			}
		}
	}

	// add order by
	if orderBy := context.Request.Form.Get("order_by"); orderBy != "" {
		if regexp.MustCompile("^[a-zA-Z_]+$").MatchString(orderBy) {
			scope := utils.NewScope(s.Context.Resource.Value)
			if field := scope.LookUpField(strings.TrimSuffix(orderBy, "_desc")); field != nil {
				if strings.HasSuffix(orderBy, "_desc") {
					db = db.Order(field.DBName + " DESC")
				} else {
					db = db.Order(field.DBName)
				}
			}
		}
	}

	context.SetDB(db)

	// call search
	var keyword string
	if keyword = context.Request.Form.Get("keyword"); keyword == "" {
		keyword = context.Request.URL.Query().Get("keyword")
	}

	if s.Resource.SearchHandler != nil {
		context.SetDB(s.Resource.SearchHandler(keyword, context))
		return context
	}

	return context
}

func (s *Searcher) parseContext(withDefaultScope bool) *qor.Context {
	var (
		searcher = s.clone()
		context  = searcher.Context.Context.Clone()
	)

	if context != nil && context.Request != nil {
		// parse scopes
		scopes := context.Request.Form["scopes"]
		searcher = searcher.Scope(scopes...)

		// parse filters
		for key := range context.Request.Form {
			if matches := filterRegexp.FindStringSubmatch(key); len(matches) > 0 {
				var prefix = fmt.Sprintf("filters[%v].", matches[1])
				for _, filter := range s.Resource.filters {
					if filter.Name == matches[1] {
						if metaValues, err := resource.ConvertFormToMetaValues(context.Request, []resource.Metaor{}, prefix); err == nil {
							searcher = searcher.Filter(filter, metaValues)
						}
					}
				}
			}
		}

		if savingName := context.Request.Form.Get("filter_saving_name"); savingName != "" {
			var filters []SavedFilter
			requestURL := context.Request.URL
			requestURLQuery := context.Request.URL.Query()
			requestURLQuery.Del("filter_saving_name")
			requestURL.RawQuery = requestURLQuery.Encode()
			newFilters := []SavedFilter{{Name: savingName, URL: requestURL.String()}}
			if context.AddError(searcher.Admin.SettingsStorage.Get("saved_filters", &filters, searcher.Context)); !context.HasError() {
				for _, filter := range filters {
					if filter.Name != savingName {
						newFilters = append(newFilters, filter)
					}
				}

				context.AddError(searcher.Admin.SettingsStorage.Save("saved_filters", newFilters, searcher.Resource, context.CurrentUser, searcher.Context))
			}
		}

		if savingName := context.Request.Form.Get("delete_saved_filter"); savingName != "" {
			var filters, newFilters []SavedFilter
			if context.AddError(searcher.Admin.SettingsStorage.Get("saved_filters", &filters, searcher.Context)); !context.HasError() {
				for _, filter := range filters {
					if filter.Name != savingName {
						newFilters = append(newFilters, filter)
					}
				}

				context.AddError(searcher.Admin.SettingsStorage.Save("saved_filters", newFilters, searcher.Resource, context.CurrentUser, searcher.Context))
			}
		}
	}

	searcher.filterData(context, withDefaultScope)

	db := context.GetDB()

	// pagination
	context.SetDB(db.Model(s.Resource.Value).Set("qor:getting_total_count", true))
	total := int64(0)
	s.Resource.CallFindMany(&total, context)
	s.Pagination.Total = int(total)

	if s.Pagination.CurrentPage == 0 {
		if s.Context.Request != nil {
			if page, err := strconv.Atoi(s.Context.Request.Form.Get("page")); err == nil {
				s.Pagination.CurrentPage = page
			}
		}

		if s.Pagination.CurrentPage == 0 {
			s.Pagination.CurrentPage = 1
		}
	}

	if s.Pagination.PerPage == 0 {
		if perPage, err := strconv.Atoi(s.Context.Request.Form.Get("per_page")); err == nil {
			s.Pagination.PerPage = perPage
		} else if all := s.Context.Request.Form.Get("per_page"); all == "all" { // added all to support kanban TODO: change this to be removal of limit in future
			s.Pagination.PerPage = 1000000
		} else if s.Resource.Config.PageCount > 0 {
			s.Pagination.PerPage = s.Resource.Config.PageCount
		} else {
			s.Pagination.PerPage = PaginationPageCount
		}
	}

	limit := s.Pagination.PerPage
	if s.Context.Request != nil {
		if l, err := strconv.Atoi(s.Context.Request.Form.Get("limit")); err == nil {
			limit = l
		}
	}

	if s.Pagination.CurrentPage > 0 {
		s.Pagination.Pages = (s.Pagination.Total-1)/s.Pagination.PerPage + 1
		db = db.Limit(limit).Offset((s.Pagination.CurrentPage - 1) * s.Pagination.PerPage)
	}

	db.Set("qor:getting_total_count", false)
	context.SetDB(db)

	return context
}

type filterField struct {
	FieldName string
	Operation string
}

func filterResourceByFields(res *Resource, filterFields []filterField, keyword string, db *gorm.DB, context *qor.Context) *gorm.DB {
	if keyword != "" {
		var (
			joinConditionsMap  = map[string][]string{}
			conditions         []string
			keywords           []interface{}
			keywordEx          []string //for select_many_config filter
			generateConditions func(field filterField, value interface{})
		)

		generateConditions = func(filterfield filterField, value interface{}) {
			column := filterfield.FieldName
			scope := utils.NewScope(value)
			rfValue := reflect.ValueOf(value)
			currentScope, nextScope := scope, scope

			if strings.Contains(column, ".") {
				for _, field := range strings.Split(column, ".") {
					column = field
					currentScope = nextScope
					if field := currentScope.LookUpField(field); field != nil {
						if relationship := scope.Relationships.Relations[column]; relationship != nil {
							nextScope := utils.NewScope(reflect.New(field.FieldType).Interface())
							if relationship.Type == "many_to_many" {
								var (
									condition string
									jointable = relationship.JoinTable.Table
									key       = fmt.Sprintf("LEFT JOIN %v ON", jointable)
								)

								conditions := []string{}
								for index := range relationship.References {
									conditions = append(conditions,
										fmt.Sprintf("%v.%v = %v.%v",
											currentScope.Table, relationship.References[index].ForeignKey.DBName,
											jointable, relationship.References[index].ForeignKey.DBName,
										))
								}
								condition = strings.Join(conditions, " AND ")

								conditions = []string{}
								//for index := range relationship.AssociationForeignDBNames {
								for index := range relationship.Schema.PrimaryFieldDBNames {
									conditions = append(conditions,
										fmt.Sprintf("%v.%v = %v.%v",
											nextScope.Table, relationship.Schema.PrimaryFieldDBNames[index],
											jointable, relationship.Schema.PrimaryFieldDBNames[index],
										))
								}

								joinConditionsMap[key] = []string{fmt.Sprintf("%v LEFT JOIN %v ON %v", condition, nextScope.Table, strings.Join(conditions, " AND "))}
							} else {
								key := fmt.Sprintf("LEFT JOIN %v ON", nextScope.Table)
								for index := range relationship.References {
									if relationship.Type == "has_one" || relationship.Type == "has_many" {
										joinConditionsMap[key] = append(joinConditionsMap[key],
											fmt.Sprintf("%v.%v = %v.%v",
												nextScope.Table, relationship.References[index].ForeignKey.DBName,
												currentScope.Table, relationship.References[index].PrimaryKey.DBName,
											))
									} else if relationship.Type == "belongs_to" {
										joinConditionsMap[key] = append(joinConditionsMap[key],
											fmt.Sprintf("%v.%v = %v.%v",
												currentScope.Table, relationship.References[index].ForeignKey.DBName,
												nextScope.Table, relationship.References[index].PrimaryKey.DBName,
											))
									}
								}
							}
						}
					}
				}
			}
			tableName := currentScope.Table

			if filterfield.Operation == "In" {
				keywordEx = strings.Split(strings.ToUpper(keyword), ",")
			}

			appendString := func(field *schema.Field) {
				switch filterfield.Operation {
				case "equal", "eq":
					conditions = append(conditions, fmt.Sprintf("upper(%v.%v) = upper(?)", tableName, field.DBName))
					keywords = append(keywords, keyword)
				case "start_with":
					conditions = append(conditions, fmt.Sprintf("upper(%v.%v) like upper(?)", tableName, field.DBName))
					keywords = append(keywords, keyword+"%")
				case "end_with":
					conditions = append(conditions, fmt.Sprintf("upper(%v.%v) like upper(?)", tableName, field.DBName))
					keywords = append(keywords, "%"+keyword)
				case "present":
					conditions = append(conditions, fmt.Sprintf("%v.%v <> ?", tableName, field.DBName))
					keywords = append(keywords, "")
				case "blank":
					conditions = append(conditions, fmt.Sprintf("%v.%v = ? OR %v.%v IS NULL", tableName, field.DBName, tableName, field.DBName))
					keywords = append(keywords, "")
				case "In":
					conditions = append(conditions, fmt.Sprintf("upper(%v.%v) in (?)", tableName, field.DBName))
					keywords = append(keywords, keywordEx)
				default:
					conditions = append(conditions, fmt.Sprintf("upper(%v.%v) like upper(?)", tableName, field.DBName))
					keywords = append(keywords, "%"+keyword+"%")
				}
			}

			appendInteger := func(field *schema.Field) {
				if num, err := strconv.Atoi(keyword); err == nil {
					keywords = append(keywords, num)
					switch filterfield.Operation {
					case "gt":
						conditions = append(conditions, fmt.Sprintf("%v.%v > ?", tableName, field.DBName))
					case "lt":
						conditions = append(conditions, fmt.Sprintf("%v.%v < ?", tableName, field.DBName))
					case "present":
						conditions = append(conditions, fmt.Sprintf("%v.%v IS NOT NULL", tableName, field.DBName))
					case "blank":
						conditions = append(conditions, fmt.Sprintf("%v.%v IS NULL", tableName, field.DBName))
					default:
						conditions = append(conditions, fmt.Sprintf("%v.%v = ?", tableName, field.DBName))
					}
				} else if filterfield.Operation == "In" {
					conditions = append(conditions, fmt.Sprintf("%v.%v in (?)", tableName, field.DBName))
					keywords = append(keywords, keywordEx)
				}
			}

			appendFloat := func(field *schema.Field) {
				if f, err := strconv.ParseFloat(keyword, 64); err == nil {
					keywords = append(keywords, f)
					switch filterfield.Operation {
					case "gt":
						conditions = append(conditions, fmt.Sprintf("%v.%v > ?", tableName, field.DBName))
					case "lt":
						conditions = append(conditions, fmt.Sprintf("%v.%v < ?", tableName, field.DBName))
					default:
						conditions = append(conditions, fmt.Sprintf("%v.%v = ?", tableName, field.DBName))
					}
				}
			}

			appendBool := func(field *schema.Field) {
				if value, err := strconv.ParseBool(keyword); err == nil {
					conditions = append(conditions, fmt.Sprintf("%v.%v = ?", tableName, field.DBName))
					keywords = append(keywords, value)
				} else {
					switch keyword {
					case "present":
						conditions = append(conditions, fmt.Sprintf("%v.%v IS NOT NULL", tableName, field.DBName))
					case "blank":
						conditions = append(conditions, fmt.Sprintf("%v.%v IS NULL", tableName, field.DBName))
					}
				}
			}

			appendTime := func(field *schema.Field) {
				if parsedTime, err := utils.ParseTime(keyword, context); err == nil {
					conditions = append(conditions, fmt.Sprintf("%v.%v = ?", tableName, field.DBName))
					keywords = append(keywords, parsedTime)
				}
			}

			appendStruct := func(field *schema.Field, rvfield reflect.Value) {
				switch rvfield.Interface().(type) {
				case time.Time, *time.Time:
					appendTime(field)
					// add support for sql null fields
				case sql.NullInt64:
					appendInteger(field)
				case sql.NullFloat64:
					appendFloat(field)
				case sql.NullString:
					appendString(field)
				case sql.NullBool:
					appendBool(field)
				default:
					// if we don't recognize the struct type, just ignore it
				}
			}

			if field := currentScope.LookUpField(column); field != nil {

				if rvfield := rfValue.FieldByName(column); rvfield.FieldByName(field.Name).Kind() != reflect.Struct {
					switch rvfield.FieldByName(field.Name).Kind() {
					case reflect.String:
						appendString(field)
					case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64, reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
						appendInteger(field)
					case reflect.Float32, reflect.Float64:
						appendFloat(field)
					case reflect.Bool:
						appendBool(field)
					case reflect.Struct, reflect.Ptr:
						appendStruct(field, rvfield)
					default:
						if filterfield.Operation == "In" {
							conditions = append(conditions, fmt.Sprintf("%v.%v in (?)", tableName, field.DBName))
							keywords = append(keywords, keywordEx)
						} else {
							conditions = append(conditions, fmt.Sprintf("%v.%v = ?", tableName, field.DBName))
							keywords = append(keywords, keyword)
						}
					}
				} else if relationship := currentScope.Relationships.Relations[field.Name]; relationship != nil {
					switch relationship.Type {
					case "select_one", "select_many", "has_many", "has_one":
						for _, foreignField := range relationship.ParseConstraint().ForeignKeys {
							generateConditions(filterField{
								FieldName: strings.Join([]string{field.Name, foreignField.Name}, "."),
								Operation: filterfield.Operation,
							}, currentScope)
						}
					case "belongs_to":
						for _, foreignField := range relationship.ParseConstraint().ForeignKeys {
							generateConditions(filterField{
								FieldName: foreignField.Name,
								Operation: filterfield.Operation,
							}, currentScope)
						}
					case "many_to_many":
						for _, foreignField := range relationship.ParseConstraint().ForeignKeys {
							generateConditions(filterField{
								FieldName: strings.Join([]string{field.Name, foreignField.Name}, "."),
								Operation: filterfield.Operation,
							}, currentScope)
						}
					}
				}
			} else {
				// context.AddError(fmt.Errorf("filter `%v` is not supported", column))
			}
		}

		for _, field := range filterFields {
			generateConditions(field, res.Value)
		}

		// join conditions
		if len(joinConditionsMap) > 0 {
			var joinConditions []string
			for key, values := range joinConditionsMap {
				joinConditions = append(joinConditions, fmt.Sprintf("%v %v", key, strings.Join(values, " AND ")))
			}
			db = db.Joins(strings.Join(joinConditions, " "))
		}

		// search conditions
		if len(conditions) > 0 {
			return db.Where(strings.Join(conditions, " OR "), keywords...)
		}
	}

	return db
}
