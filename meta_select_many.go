package admin

import (
	"errors"
	"fmt"
	"html/template"
	"reflect"

	"github.com/jinzhu/gorm"
	"github.com/qor/qor"
	"github.com/qor/qor/resource"
	"github.com/qor/qor/utils"
)

// SelectManyConfig meta configuration used for select many
type SelectManyConfig struct {
	Collection               interface{} // []string, [][]string, func(interface{}, *qor.Context) [][]string, func(interface{}, *admin.Context) [][]string
	DefaultCreating          bool
	Placeholder              string
	SelectionTemplate        string
	SelectMode               string // select, select_async, bottom_sheet
	Select2ResultTemplate    template.JS
	Select2SelectionTemplate template.JS
	ForSerializedObject      bool
	RemoteDataResource       *Resource
	RemoteDataHasImage       bool
	PrimaryField             string
	SelectOneConfig
}

// GetTemplate get template for selection template
func (selectManyConfig SelectManyConfig) GetTemplate(context *Context, metaType string) ([]byte, error) {
	if metaType == "form" && selectManyConfig.SelectionTemplate != "" {
		return context.Asset(selectManyConfig.SelectionTemplate)
	}
	return nil, errors.New("not implemented")
}

// ConfigureQorMeta configure select many meta
func (selectManyConfig *SelectManyConfig) ConfigureQorMeta(metaor resource.Metaor) {
	if meta, ok := metaor.(*Meta); ok {
		selectManyConfig.SelectOneConfig.Collection = selectManyConfig.Collection
		selectManyConfig.SelectOneConfig.SelectMode = selectManyConfig.SelectMode
		selectManyConfig.SelectOneConfig.DefaultCreating = selectManyConfig.DefaultCreating
		selectManyConfig.SelectOneConfig.Placeholder = selectManyConfig.Placeholder
		selectManyConfig.SelectOneConfig.RemoteDataResource = selectManyConfig.RemoteDataResource
		selectManyConfig.SelectOneConfig.PrimaryField = selectManyConfig.PrimaryField

		selectManyConfig.SelectOneConfig.ConfigureQorMeta(meta)

		selectManyConfig.RemoteDataResource = selectManyConfig.SelectOneConfig.RemoteDataResource
		selectManyConfig.SelectMode = selectManyConfig.SelectOneConfig.SelectMode
		selectManyConfig.DefaultCreating = selectManyConfig.SelectOneConfig.DefaultCreating
		selectManyConfig.PrimaryField = selectManyConfig.SelectOneConfig.PrimaryField
		meta.Type = "select_many"

		// Set FormattedValuer
		if meta.FormattedValuer == nil {
			meta.SetFormattedValuer(func(record interface{}, context *qor.Context) interface{} {
				reflectValues := reflect.Indirect(reflect.ValueOf(meta.GetValuer()(record, context)))
				var results []string
				if reflectValues.IsValid() {
					for i := 0; i < reflectValues.Len(); i++ {
						results = append(results, utils.Stringify(reflectValues.Index(i).Interface()))
					}
				}
				return results
			})
		}
	}
}

// ConfigureQORAdminFilter configure admin filter
func (selectManyConfig *SelectManyConfig) ConfigureQORAdminFilter(filter *Filter) {
	var structField *gorm.StructField

	if field, ok := filter.Resource.GetAdmin().DB.NewScope(filter.Resource.Value).FieldByName(filter.Name); ok {
		structField = field.StructField
	}

	selectManyConfig.SelectOneConfig.Collection = selectManyConfig.Collection
	selectManyConfig.SelectOneConfig.SelectMode = selectManyConfig.SelectMode
	selectManyConfig.SelectOneConfig.RemoteDataResource = selectManyConfig.RemoteDataResource
	selectManyConfig.SelectOneConfig.PrimaryField = selectManyConfig.PrimaryField
	selectManyConfig.prepareDataSource(structField, filter.Resource, "!remote_data_filter")

	filter.Operations = []string{"contains"}
	filter.Type = "select_many"
}

// FilterValue filter value
func (selectManyConfig *SelectManyConfig) FilterValue(filter *Filter, context *Context) interface{} {
	var (
		prefix  = fmt.Sprintf("filters[%v].", filter.Name)
		keyword string
	)

	if metaValues, err := resource.ConvertFormToMetaValues(context.Request, []resource.Metaor{}, prefix); err == nil {
		if metaValue := metaValues.Get("Value"); metaValue != nil {
			keyword = utils.ToString(metaValue.Value)
		}
	}

	if keyword != "" && selectManyConfig.RemoteDataResource != nil {
		result := selectManyConfig.RemoteDataResource.NewStruct()
		clone := context.Clone()
		clone.ResourceID = keyword
		if selectManyConfig.RemoteDataResource.CallFindOne(result, nil, clone) == nil {
			return result
		}
	}

	return keyword
}
