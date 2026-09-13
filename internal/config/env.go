package config

import (
	"errors"
	"fmt"
	"os"
	"reflect"
	"strconv"
	"strings"
)

const EnvPrefix = "MGKE"

type Lookup func(string) (string, bool)

func LoadWithEnv(path string, lookup Lookup) (*Config, error) {
	cfg, err := Load(path)
	if err != nil {
		return nil, err
	}

	if err := ApplyEnv(cfg, lookup); err != nil {
		return nil, err
	}
	return cfg, nil
}

func ApplyEnv(cfg *Config, lookup Lookup) error {
	if cfg == nil {
		return nil
	}
	if lookup == nil {
		lookup = os.LookupEnv
	}

	var errs []error
	walkEnv(reflect.ValueOf(cfg).Elem(), EnvPrefix, lookup, &errs)
	return errors.Join(errs...)
}

func EnvNames() []string {
	names := make([]string, 0, 64)
	collectEnvNames(reflect.TypeOf(Config{}), EnvPrefix, &names)
	return names
}

func walkEnv(value reflect.Value, prefix string, lookup Lookup, errs *[]error) {
	valueType := value.Type()

	for i := 0; i < valueType.NumField(); i++ {
		field := valueType.Field(i)
		if !field.IsExported() {
			continue
		}

		fieldValue := value.Field(i)
		switch fieldValue.Kind() {
		case reflect.Struct:
			walkEnv(fieldValue, prefix+"_"+snakeUpper(yamlName(field)), lookup, errs)
		case reflect.Pointer:
			elem := fieldValue.Type().Elem()
			if elem.Kind() == reflect.Struct {
				section := prefix + "_" + snakeUpper(yamlName(field))
				if fieldValue.IsNil() {
					if !hasEnvBelow(elem, section, lookup) {
						continue
					}
					fieldValue.Set(reflect.New(elem))
				}
				walkEnv(fieldValue.Elem(), section, lookup, errs)
				continue
			}
			if elem.Kind() == reflect.String {
				raw, ok := lookupName(field, prefix, lookup)
				if !ok {
					continue
				}
				ptr := reflect.New(elem)
				ptr.Elem().SetString(raw)
				fieldValue.Set(ptr)
			}
		default:
			raw, ok := lookupName(field, prefix, lookup)
			if !ok {
				continue
			}
			if err := setEnvValue(fieldValue, raw); err != nil {
				*errs = append(*errs, fmt.Errorf("%s: %w", lookupNameOf(field, prefix), err))
			}
		}
	}
}

func lookupName(field reflect.StructField, prefix string, lookup Lookup) (string, bool) {
	for _, name := range envNames(field, prefix) {
		if raw, ok := lookup(name); ok {
			return raw, true
		}
	}
	return "", false
}

func lookupNameOf(field reflect.StructField, prefix string) string {
	names := envNames(field, prefix)
	if len(names) == 0 {
		return snakeUpper(yamlName(field))
	}
	return names[0]
}

func envNames(field reflect.StructField, prefix string) []string {
	derived := prefix + "_" + snakeUpper(yamlName(field))
	names := []string{derived}

	if tag := strings.TrimSpace(field.Tag.Get("env")); tag != "" && tag != derived {
		names = append(names, tag)
	}
	return names
}

func setEnvValue(value reflect.Value, raw string) error {
	switch value.Kind() {
	case reflect.String:
		value.SetString(raw)
	case reflect.Bool:
		parsed, err := parseBool(raw)
		if err != nil {
			return err
		}
		value.SetBool(parsed)
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		parsed, err := strconv.ParseInt(strings.TrimSpace(raw), 10, 64)
		if err != nil {
			return fmt.Errorf("expected an integer, got %q", raw)
		}
		value.SetInt(parsed)
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		parsed, err := strconv.ParseUint(strings.TrimSpace(raw), 10, 64)
		if err != nil {
			return fmt.Errorf("expected a positive integer, got %q", raw)
		}
		value.SetUint(parsed)
	case reflect.Float32, reflect.Float64:
		parsed, err := strconv.ParseFloat(strings.TrimSpace(raw), 64)
		if err != nil {
			return fmt.Errorf("expected a number, got %q", raw)
		}
		value.SetFloat(parsed)
	case reflect.Slice:
		return setEnvSlice(value, raw)
	default:
		return fmt.Errorf("type %s cannot be set from the environment", value.Type())
	}
	return nil
}

func setEnvSlice(value reflect.Value, raw string) error {
	parts := strings.Split(raw, ",")
	items := make([]string, 0, len(parts))
	for _, part := range parts {
		trimmed := strings.TrimSpace(part)
		if trimmed != "" {
			items = append(items, trimmed)
		}
	}

	switch value.Type().Elem().Kind() {
	case reflect.String:
		value.Set(reflect.ValueOf(items))
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		parsed := make([]int64, 0, len(items))
		for _, item := range items {
			number, err := strconv.ParseInt(item, 10, 64)
			if err != nil {
				return fmt.Errorf("expected a list of integers, got %q", item)
			}
			parsed = append(parsed, number)
		}
		slice := reflect.MakeSlice(value.Type(), len(parsed), len(parsed))
		for i, number := range parsed {
			slice.Index(i).SetInt(number)
		}
		value.Set(slice)
	default:
		return fmt.Errorf("type %s cannot be set from the environment", value.Type())
	}
	return nil
}

func parseBool(raw string) (bool, error) {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "1", "t", "true", "yes", "y", "on", "enable", "enabled":
		return true, nil
	case "0", "f", "false", "no", "n", "off", "disable", "disabled", "":
		return false, nil
	}
	return false, fmt.Errorf("expected a boolean, got %q", raw)
}

func hasEnvBelow(structType reflect.Type, prefix string, lookup Lookup) bool {
	for i := 0; i < structType.NumField(); i++ {
		field := structType.Field(i)
		if !field.IsExported() {
			continue
		}

		name := prefix + "_" + snakeUpper(yamlName(field))
		if field.Type.Kind() == reflect.Struct {
			if hasEnvBelow(field.Type, name, lookup) {
				return true
			}
			continue
		}
		if field.Type.Kind() == reflect.Pointer {
			if field.Type.Elem().Kind() == reflect.Struct {
				if hasEnvBelow(field.Type.Elem(), name, lookup) {
					return true
				}
				continue
			}
		}
		for _, candidate := range envNames(field, prefix) {
			if _, ok := lookup(candidate); ok {
				return true
			}
		}
	}
	return false
}

func collectEnvNames(structType reflect.Type, prefix string, names *[]string) {
	for i := 0; i < structType.NumField(); i++ {
		field := structType.Field(i)
		if !field.IsExported() {
			continue
		}

		name := prefix + "_" + snakeUpper(yamlName(field))
		switch field.Type.Kind() {
		case reflect.Struct:
			collectEnvNames(field.Type, name, names)
		case reflect.Pointer:
			if field.Type.Elem().Kind() == reflect.Struct {
				collectEnvNames(field.Type.Elem(), name, names)
				continue
			}
			*names = append(*names, envNames(field, prefix)...)
		case reflect.Array, reflect.Map:
			continue
		default:
			*names = append(*names, envNames(field, prefix)...)
		}
	}
}

func yamlName(field reflect.StructField) string {
	tag := field.Tag.Get("yaml")
	if tag == "" {
		return strings.ToUpper(field.Name)
	}
	name := strings.SplitN(tag, ",", 2)[0]
	if name == "" || name == "-" {
		return strings.ToUpper(field.Name)
	}
	return name
}

func snakeUpper(name string) string {
	if strings.Contains(name, "_") || name == strings.ToUpper(name) {
		return strings.ToUpper(name)
	}

	var builder strings.Builder
	for i, r := range name {
		if r >= 'A' && r <= 'Z' && i > 0 {
			builder.WriteByte('_')
		}
		builder.WriteRune(r)
	}
	return strings.ToUpper(builder.String())
}
