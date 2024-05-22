package clickhouse

import (
	"fmt"
	"reflect"
)

func mapTypeToClickhouseType(t reflect.Type) string {
	switch t.Kind() { //nolint:exhaustive
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
		if t.Elem().Kind() == reflect.Uint8 {
			return fmt.Sprintf("FixedString(%d)", t.Len())
		} else {
			return fmt.Sprintf("Array(%s)", mapTypeToClickhouseType(t.Elem()))
		}
	default:
		panic(fmt.Sprintf("unknown type %v", t))
	}
}

func ReflectSchemeToClickhouse(f any) ([]string, error) {
	fields := make([]string, 0)
	t := reflect.TypeOf(f).Elem()

	for i := range t.NumField() {
		field := t.Field(i)

		fields = append(fields, field.Name+" "+mapTypeToClickhouseType(field.Type))
	}

	return fields, nil
}
