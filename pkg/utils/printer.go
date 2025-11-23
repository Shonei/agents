package utils

import (
	"encoding/json"
	"fmt"
	"os"
	"reflect"
	"strings"
	"text/tabwriter"

	"gopkg.in/yaml.v3"
)

var resources = map[string][]string{}

func RegisterResource(resource any, headers []string) {
	rt := reflect.TypeOf(resource)

	_, ok := resources[rt.Name()]
	if ok {
		panic(fmt.Sprintf("resource %s already registered", rt.Name()))
	}

	resources[rt.Name()] = headers
}

// Print prints the resource to the console
// If the resource is a slice, it prints all the elements and headers
func Print(resource any, format string) {
	if resource == nil {
		fmt.Println("<nil>")
		return
	}

	v := reflect.ValueOf(resource)
	t := v.Type()

	// If slice
	if t.Kind() == reflect.Slice {
		ln := v.Len()
		if ln == 0 {
			fmt.Println("(no items)")
			return
		}

		// Determine element type (follow pointer)
		et := t.Elem()
		if et.Kind() == reflect.Ptr {
			et = et.Elem()
		}

		switch format {
		case "table":
			printTableSlice(et, v)
		case "yaml":
			printYamlSlice(et, v)
		case "json":
			printJsonSlice(et, v)
		default:
			fmt.Fprintf(os.Stderr, "unsupported output format '%s'\n", format)
			os.Exit(1)
		}

		return
	}

	// Non-slice printing
	val := v
	if val.Kind() == reflect.Ptr {
		val = valueDeref(val)
	}

	if val.Kind() == reflect.Struct {
		switch format {
		case "table":
			printTableSlice(t, reflect.ValueOf([]any{resource}))
		case "yaml":
			data, err := yaml.Marshal(val.Interface())
			if err != nil {
				fmt.Printf("(failed to marshal to YAML: %v)\n", err)
				return
			}

			fmt.Print(string(data))
		case "json":
			data, err := json.Marshal(val.Interface())
			if err != nil {
				fmt.Printf("(failed to marshal to JSON: %v)\n", err)
				return
			}

			fmt.Print(string(data))
		default:
			fmt.Fprintf(os.Stderr, "unsupported output format '%s'\n", format)
			os.Exit(1)
		}

		// Marshal struct to YAML and print

		return
	}
}

func printYamlSlice(et reflect.Type, v reflect.Value) {
	yamlArray := struct {
		Type  string `yaml:"type"`
		Items []any  `yaml:"items"`
	}{
		Type:  et.Name(),
		Items: make([]any, 0, v.Len()),
	}

	for i := 0; i < v.Len(); i++ {
		yamlArray.Items = append(yamlArray.Items, v.Index(i).Interface())
	}

	data, err := yaml.Marshal(yamlArray)
	if err != nil {
		fmt.Fprintf(os.Stderr, "(failed to marshal to YAML: %v)\n", err)
		return
	}

	fmt.Print(string(data))
}

func printJsonSlice(et reflect.Type, v reflect.Value) {
	yamlArray := struct {
		Type  string `json:"type"`
		Items []any  `json:"items"`
	}{
		Type:  et.Name(),
		Items: make([]any, 0, v.Len()),
	}

	for i := 0; i < v.Len(); i++ {
		yamlArray.Items = append(yamlArray.Items, v.Index(i).Interface())
	}

	data, err := json.Marshal(yamlArray)
	if err != nil {
		fmt.Fprintf(os.Stderr, "(failed to marshal to JSON: %v)\n", err)
		return
	}

	fmt.Print(string(data))
}

func printTableSlice(et reflect.Type, v reflect.Value) {
	headers := getHeadersForType(et)
	ln := v.Len()

	w := tabwriter.NewWriter(os.Stdout, 0, 0, 1, ' ', 0)

	w.Write([]byte(fmt.Sprintf("%s\n", strings.Join(headers, "\t"))))

	// prints a --- line after the headers
	breaker := make([]string, 0, len(headers))
	for i := range headers {
		breaker = append(breaker, strings.Repeat("-", len(headers[i])))
	}
	w.Write([]byte(fmt.Sprintf("%s\n", strings.Join(breaker, "\t"))))

	for i := 0; i < ln; i++ {
		row := valueDeref(v.Index(i))
		for _, h := range headers {
			w.Write([]byte(fmt.Sprintf("%s\t", fieldAsString(row, h))))
		}
		w.Write([]byte("\n"))
	}

	w.Flush()
}

// getHeadersForType returns registered headers for a struct type name or derives them from exported fields
func getHeadersForType(t reflect.Type) []string {
	if t == nil {
		panic("type is nil")
	}
	if t.Kind() != reflect.Struct {
		panic("type is not struct")
	}
	if hs, ok := resources[t.Name()]; ok {
		return hs
	}

	panic("no headers registered for type " + t.Name())
}

// valueDeref returns the value with one level of pointer dereference if needed
func valueDeref(v reflect.Value) reflect.Value {
	if v.IsValid() && v.Kind() == reflect.Ptr && !v.IsNil() {
		return v.Elem()
	}
	return v
}

// fieldAsString returns the string representation for a named exported field on a struct value
func fieldAsString(v reflect.Value, fieldName string) string {
	if !v.IsValid() {
		return ""
	}

	// Unwrap interfaces and pointers on the input value until we hit the concrete value
	for {
		switch v.Kind() {
		case reflect.Interface, reflect.Ptr:
			if v.IsNil() {
				return ""
			}
			v = v.Elem()
		default:
			goto DONE_UNWRAP_IN
		}
	}
DONE_UNWRAP_IN:

	if v.Kind() == reflect.Slice {
		values := make([]string, 0, v.Len())
		for i := 0; i < v.Len(); i++ {
			values = append(values, fieldAsString(v.Index(i), fieldName))
		}

		return strings.Join(values, ", ")
	}

	if v.Kind() != reflect.Struct {
		return fmt.Sprint(v.Interface())
	}

	names := strings.Split(fieldName, ".")
	if len(names) > 1 {
		f := v.FieldByName(names[0])
		if !f.IsValid() {
			return ""
		}

		return fieldAsString(f, strings.Join(names[1:], "."))
	}

	f := v.FieldByName(fieldName)
	if !f.IsValid() {
		return ""
	}

	return fmt.Sprint(f.Interface())
}
