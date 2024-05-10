package clickhouse

import (
	"fmt"
	"reflect"
)

func mapTypeToClickhouseType(t reflect.Type) string {
	// Map Go type to Clickhouse type
	switch t.Kind() {
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
	case reflect.Array:
		// return FixedString of length of array uint8
		if t.Elem().Kind() == reflect.Uint8 {
			return fmt.Sprintf("FixedString(%d)", t.Len())
		} else {
			// return Array
			return fmt.Sprintf("Array(%s)", mapTypeToClickhouseType(t.Elem()))
		}
	default:
		return "String"
	}
}

func ReflectSchemeToClickhouse(f any) ([]string, error) {
	// reflect types.Block

	fields := make([]string, 0)
	t := reflect.TypeOf(f).Elem()

	for i := 0; i < t.NumField(); i++ {
		field := t.Field(i)

		fields = append(fields, field.Name+" "+mapTypeToClickhouseType(field.Type))
	}

	return fields, nil
}
