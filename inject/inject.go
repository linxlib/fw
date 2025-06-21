// Package inject provides utilities for mapping and injecting dependencies in various ways.
package inject

import (
	"fmt"
	"reflect"
)

// Injector represents an interface for mapping and injecting dependencies into structs
// and function arguments.
type Injector interface {
	Applicator
	Invoker
	TypeMapper
	Provider
	// SetParent sets the parent of the injector. If the injector cannot find a
	// dependency in its Type map it will check its parent before returning an
	// error.
	SetParent(Injector)
	GetParent() Injector
}
type Provider interface {
	Provide(interface{}) error
}

// Applicator represents an interface for mapping dependencies to a struct.
type Applicator interface {
	// Apply Maps dependencies in the Type map to each field in the struct
	// that is tagged with 'inject'. Returns an error if the injection
	// fails.
	Apply(interface{}) error
}

// Invoker represents an interface for calling functions via reflection.
type Invoker interface {
	// Invoke attempts to call the interface{} provided as a function,
	// providing dependencies for function arguments based on Type. Returns
	// a slice of reflect.Value representing the returned values of the function.
	// Returns an error if the injection fails.
	Invoke(interface{}) ([]reflect.Value, error)
}

// TypeMapper represents an interface for mapping interface{} values based on type.
type TypeMapper interface {
	// Map Maps the interface{} value based on its immediate type from reflect.TypeOf.
	Map(...interface{}) TypeMapper
	// MapTo Maps the interface{} value based on the pointer of an Interface provided.
	// This is really only useful for mapping a value as an interface, as interfaces
	// cannot at this time be referenced directly without a pointer.
	MapTo(interface{}, interface{}) TypeMapper
	// Set Provides a possibility to directly insert a mapping based on type and value.
	// This makes it possible to directly map type arguments not possible to instantiate
	// with reflect like unidirectional channels.
	Set(reflect.Type, reflect.Value) TypeMapper
	// Returns the Value that is mapped to the current type. Returns a zeroed Value if
	// the Type has not been mapped.
	Get(reflect.Type) reflect.Value
	Value(reflect.Type) reflect.Value
}

type injector struct {
	values map[reflect.Type]reflect.Value
	parent Injector
}

// InterfaceOf dereferences a pointer to an Interface type.
// It panics if value is not an pointer to an interface.
func InterfaceOf(value interface{}) reflect.Type {
	t := reflect.TypeOf(value)

	for t.Kind() == reflect.Ptr {
		t = t.Elem()
	}

	if t.Kind() != reflect.Interface {
		panic("Called inject.InterfaceOf with a value that is not a pointer to an interface. (*MyInterface)(nil)")
	}

	return t
}

// New returns a new Injector.
func New() Injector {
	return &injector{
		values: make(map[reflect.Type]reflect.Value),
	}
}
func (inj *injector) GetParent() Injector {
	return inj.parent
}

// Invoke attempts to call the interface{} provided as a function,
// providing dependencies for function arguments based on Type.
// Returns a slice of reflect.Value representing the returned values of the function.
// Returns an error if the injection fails.
// It panics if f is not a function
func (inj *injector) Invoke(f interface{}) ([]reflect.Value, error) {
	t := reflect.TypeOf(f)

	var in = make([]reflect.Value, t.NumIn()) //Panic if t is not kind of Func
	for i := 0; i < t.NumIn(); i++ {
		argType := t.In(i)
		val := inj.Get(argType)
		if !val.IsValid() {
			return nil, fmt.Errorf("Value not found for type %v", argType)
		}

		in[i] = val
	}

	return reflect.ValueOf(f).Call(in), nil
}

// Maps dependencies in the Type map to each field in the struct
// that is tagged with 'inject'.
// Returns an error if the injection fails.
func (inj *injector) Apply(val interface{}) error {
	v := reflect.ValueOf(val)

	for v.Kind() == reflect.Ptr {
		v = v.Elem()
	}

	if v.Kind() != reflect.Struct {
		return nil // Should not panic here ?
	}

	t := v.Type()

	for i := 0; i < v.NumField(); i++ {
		f := v.Field(i)
		structField := t.Field(i)
		if f.CanSet() && (structField.Tag == "inject" || structField.Tag.Get("inject") != "") {
			ft := f.Type()
			v := inj.Get(ft)
			if !v.IsValid() {
				return fmt.Errorf("Value not found for type %v", ft)
			}

			f.Set(v)
		}

	}

	return nil
}
func (inj *injector) Provide(val any) error {
	if val == nil {
		return fmt.Errorf("val cannot be nil")
	}
	v := reflect.ValueOf(val)
	t0 := v.Type()
	for v.Kind() == reflect.Ptr {
		v = v.Elem()
	}
	t := v.Type()
	//if v.Kind() != reflect.Struct {
	//	return fmt.Errorf("value should be Pointer: %s", v.String()) // Should not panic here ?
	//}

	v1 := inj.Value(t)
	if !v1.IsValid() {
		v2 := inj.Value(t0)
		if !v2.IsValid() {
			return fmt.Errorf("value not found for type %v", t)
		}
		v.Set(v2.Elem())
	} else {
		v.Set(v1)
	}
	return nil
}
func (inj *injector) Value(t reflect.Type) reflect.Value {
	val := inj.values[t]

	if val.IsValid() {
		return val
	}

	// No concrete types found, try to find implementors if t is an interface.
	if t.Kind() == reflect.Interface {
		for k, v := range inj.values {
			if k.Implements(t) {
				val = v
				break
			}
		}
	}

	// Still no type found, try to look it up on the parent
	if !val.IsValid() && inj.parent != nil {
		val = inj.parent.Value(t)
	}

	return val
}

// Maps the concrete value of val to its dynamic type using reflect.TypeOf,
// It returns the TypeMapper registered in.
func (inj *injector) Map(val ...interface{}) TypeMapper {
	for _, val := range val {
		inj.values[reflect.TypeOf(val)] = reflect.ValueOf(val)
	}
	return inj
}

func (inj *injector) MapTo(val interface{}, ifacePtr interface{}) TypeMapper {
	inj.values[InterfaceOf(ifacePtr)] = reflect.ValueOf(val)
	return inj
}
func MapTo[T any](this Injector, val any) TypeMapper {
	return this.MapTo(val, (*T)(nil))
}

func Provide[T any](this Injector) T {
	var a = new(T)
	this.Provide(a)
	return *a
}

// Maps the given reflect.Type to the given reflect.Value and returns
// the Typemapper the mapping has been registered in.
func (inj *injector) Set(typ reflect.Type, val reflect.Value) TypeMapper {
	inj.values[typ] = val
	return inj
}

func (inj *injector) Get(t reflect.Type) reflect.Value {
	val := inj.values[t]

	if val.IsValid() {
		return val
	}

	// no concrete types found, try to find implementors
	// if t is an interface
	if t.Kind() == reflect.Interface {
		for k, v := range inj.values {
			if k.Implements(t) {
				val = v
				break
			}
		}
	}

	// Still no type found, try to look it up on the parent
	if !val.IsValid() && inj.parent != nil {
		val = inj.parent.Get(t)
	}

	return val

}

func (inj *injector) SetParent(parent Injector) {
	inj.parent = parent
}
