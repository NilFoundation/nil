package clickhouse

import (
	"fmt"
	"reflect"
	"strings"

	"github.com/NilFoundation/nil/nil/common/logging"
)

var logger = logging.NewLogger("ch-reflection")

func mapTypeToClickhouseType(t reflect.Type) string {
	switch t.Kind() { //nolint:exhaustive
	case reflect.Bool:
		return "Boolean"
	case reflect.Uint64:
		return "UInt64"
	case reflect.Uint32:
		return "UInt32"
	case reflect.Uint16:
		return "UInt16"
	case reflect.Uint8:
		return "UInt8"
	case reflect.Int64:
		return "Int64"
	case reflect.Int32:
		return "Int32"
	case reflect.Int16:
		return "Int16"
	case reflect.Int8:
		return "Int8"
	case reflect.String:
		return "String"
	case reflect.Slice:
		return fmt.Sprintf("Array(%s)", mapTypeToClickhouseType(t.Elem()))
	case reflect.Array:
		if t.Name() == "Value" {
			return "UInt256"
		}
		if t.Name() == "Uint256" {
			return "UInt256"
		}
		if t.Elem().Kind() == reflect.Uint8 {
			return fmt.Sprintf("FixedString(%d)", t.Len())
		} else {
			return fmt.Sprintf("Array(%s)", mapTypeToClickhouseType(t.Elem()))
		}
	case reflect.Struct:
		if t.Name() == "Value" {
			return "UInt256"
		}
		if t.Name() == "Uint256" {
			return "UInt256"
		}
		if t.Name() == "TransactionFlags" {
			return "UInt8"
		}
		// return tuple of field type
		fields := make([]string, 0, t.NumField())
		for i := 0; i < t.NumField(); i++ {
			fields = append(fields, mapTypeToClickhouseType(t.Field(i).Type))
		}
		return fmt.Sprintf("Tuple(%s)", strings.Join(fields, ", "))
	case reflect.Pointer:
		return mapTypeToClickhouseType(t.Elem())
	default:
		panic(fmt.Sprintf("unknown type %v", t))
	}
}

type reflectedScheme struct {
	fieldTypes map[string]string
	fieldNames map[string]string
}

func mergeScheme(schemes []reflectedScheme) (reflectedScheme, error) {
	fieldTypes := make(map[string]string)
	fieldNames := make(map[string]string)
	for _, scheme := range schemes {
		// check if there are any conflicts
		for k := range scheme.fieldTypes {
			if _, ok := fieldTypes[k]; ok {
				return reflectedScheme{}, fmt.Errorf("field %s already exists", k)
			}
		}
		for k := range scheme.fieldNames {
			if _, ok := fieldNames[k]; ok {
				return reflectedScheme{}, fmt.Errorf("field name mapping %s already exists", k)
			}
		}
		for k, v := range scheme.fieldTypes {
			fieldTypes[k] = v
		}
		for k, v := range scheme.fieldNames {
			fieldNames[k] = v
		}
	}
	return reflectedScheme{
		fieldTypes: fieldTypes,
		fieldNames: fieldNames,
	}, nil
}

func reflectSchemeToClickhouse(f any) (reflectedScheme, error) {
	fieldTypes := make(map[string]string)
	fieldNames := make(map[string]string)
	t := reflect.TypeOf(f).Elem()
	additionalSchemes := make([]reflectedScheme, 0)

	for i := range t.NumField() {
		field := t.Field(i)
		clickhouseName := field.Tag.Get("ch")
		jsonName := field.Tag.Get("json")
		fieldNameInDb := field.Name
		if jsonName != "" {
			fieldNameInDb = strings.Split(jsonName, ",")[0]
		}
		if clickhouseName != "" {
			fieldNameInDb = strings.Split(clickhouseName, ",")[0]
		}
		if fieldNameInDb == "-" {
			continue
		}
		switch {
		case field.Type.PkgPath() == "time" && field.Type.Name() == "Time":
			fieldTypes[fieldNameInDb] = "DateTime64"
			fieldNames[field.Name] = fieldNameInDb
		case field.Type.Kind() == reflect.Struct:
			if field.Type.Name() == "Value" {
				fieldTypes[fieldNameInDb] = "UInt256"
				fieldNames[field.Name] = fieldNameInDb
				continue
			}
			if field.Type.Name() == "TransactionFlags" {
				fieldTypes[fieldNameInDb] = "UInt8"
				fieldNames[field.Name] = fieldNameInDb
				continue
			}
			scheme, err := reflectSchemeToClickhouse(reflect.New(field.Type).Interface())
			if err != nil {
				return reflectedScheme{}, err
			}
			additionalSchemes = append(additionalSchemes, scheme)
		default:
			fieldTypes[fieldNameInDb] = mapTypeToClickhouseType(field.Type)
			fieldNames[field.Name] = fieldNameInDb
		}
	}

	logger.Debug().Msgf("fieldTypes: %v", fieldTypes)
	logger.Debug().Msgf("fieldNames: %v", fieldNames)
	logger.Debug().Msgf("additionalSchemes: %v", additionalSchemes)

	return mergeScheme(append(additionalSchemes, reflectedScheme{
		fieldTypes: fieldTypes,
		fieldNames: fieldNames,
	}))
}

func (s reflectedScheme) Fields() string {
	fields := make([]string, 0, len(s.fieldTypes))
	for name, typ := range s.fieldTypes {
		fields = append(fields, fmt.Sprintf("%s %s", name, typ))
	}
	return strings.Join(fields, ", ")
}

func (s reflectedScheme) CreateTableQuery(tableName, engine string, primaryKeys, orderKeys []string) string {
	query := createTableQuery(tableName, s.Fields(), engine, primaryKeys, orderKeys)
	logger.Debug().Msgf("CreateTableQuery: %s", query)
	return query
}
