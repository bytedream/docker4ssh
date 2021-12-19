package api

import (
	"fmt"
	"reflect"
	"strings"
)

type APIError struct {
	Message string `json:"message"`
}

func structJsonLookup(v interface{}) (map[string]reflect.Kind, error) {
	rt := reflect.TypeOf(v)
	if rt.Kind() != reflect.Struct {
		return nil, fmt.Errorf("given interface is not a struct")
	}

	lookup := make(map[string]reflect.Kind)

	for i := 0; i < rt.NumField(); i++ {
		field := rt.Field(i)

		name := strings.Split(field.Tag.Get("json"), ",")[0]
		value := field.Type.Kind()

		if field.Type.Kind() == reflect.Struct {
			value = reflect.Map
		}

		lookup[name] = value
	}

	return lookup, nil
}
