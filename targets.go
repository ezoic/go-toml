package toml

import (
	"fmt"
	"math"
	"reflect"
	"strings"
	"sync"
)

type target interface {
	// Dereferences the target.
	get() reflect.Value

	// Store a string at the target.
	setString(v string) error

	// Store a boolean at the target
	setBool(v bool) error

	// Store an int64 at the target
	setInt64(v int64) error

	// Store a float64 at the target
	setFloat64(v float64) error

	// Stores any value at the target
	set(v reflect.Value) error
}

// valueTarget just contains a reflect.Value that can be set.
// It is used for struct fields.
type valueTarget reflect.Value

func (t valueTarget) get() reflect.Value {
	return reflect.Value(t)
}

func (t valueTarget) set(v reflect.Value) error {
	reflect.Value(t).Set(v)
	return nil
}

func (t valueTarget) setString(v string) error {
	t.get().SetString(v)
	return nil
}

func (t valueTarget) setBool(v bool) error {
	t.get().SetBool(v)
	return nil
}

func (t valueTarget) setInt64(v int64) error {
	t.get().SetInt(v)
	return nil
}

func (t valueTarget) setFloat64(v float64) error {
	t.get().SetFloat(v)
	return nil
}

// interfaceTarget wraps an other target to dereference on get.
type interfaceTarget struct {
	x target
}

func (t interfaceTarget) get() reflect.Value {
	return t.x.get().Elem()
}

func (t interfaceTarget) set(v reflect.Value) error {
	return t.x.set(v)
}

func (t interfaceTarget) setString(v string) error {
	return t.x.setString(v)
}

func (t interfaceTarget) setBool(v bool) error {
	return t.x.setBool(v)
}

func (t interfaceTarget) setInt64(v int64) error {
	return t.x.setInt64(v)
}

func (t interfaceTarget) setFloat64(v float64) error {
	return t.x.setFloat64(v)
}

// mapTarget targets a specific key of a map.
type mapTarget struct {
	v reflect.Value
	k reflect.Value
}

func (t mapTarget) get() reflect.Value {
	return t.v.MapIndex(t.k)
}

func (t mapTarget) set(v reflect.Value) error {
	t.v.SetMapIndex(t.k, v)
	return nil
}

func (t mapTarget) setString(v string) error {
	return t.set(reflect.ValueOf(v))
}

func (t mapTarget) setBool(v bool) error {
	return t.set(reflect.ValueOf(v))
}

func (t mapTarget) setInt64(v int64) error {
	return t.set(reflect.ValueOf(v))
}

func (t mapTarget) setFloat64(v float64) error {
	return t.set(reflect.ValueOf(v))
}

// makes sure that the value pointed at by t is indexable (Slice, Array), or
// dereferences to an indexable (Ptr, Interface).
func ensureValueIndexable(t target) error {
	f := t.get()

	switch f.Type().Kind() {
	case reflect.Slice:
		if f.IsNil() {
			return t.set(reflect.MakeSlice(f.Type(), 0, 0))
		}
	case reflect.Interface:
		if f.IsNil() || f.Elem().Type() != sliceInterfaceType {
			return t.set(reflect.MakeSlice(sliceInterfaceType, 0, 0))
		}
		if f.Elem().Type().Kind() != reflect.Slice {
			return fmt.Errorf("interface is pointing to a %s, not a slice", f.Kind())
		}
	case reflect.Ptr:
		if f.IsNil() {
			ptr := reflect.New(f.Type().Elem())
			err := t.set(ptr)
			if err != nil {
				return err
			}
			f = t.get()
		}
		return ensureValueIndexable(valueTarget(f.Elem()))
	case reflect.Array:
		// arrays are always initialized.
	default:
		return fmt.Errorf("cannot initialize a slice in %s", f.Kind())
	}
	return nil
}

var sliceInterfaceType = reflect.TypeOf([]interface{}{})
var mapStringInterfaceType = reflect.TypeOf(map[string]interface{}{})

func ensureMapIfInterface(x target) {
	v := x.get()
	if v.Kind() == reflect.Interface && v.IsNil() {
		newElement := reflect.MakeMap(mapStringInterfaceType)
		x.set(newElement)
	}
}

func setString(t target, v string) error {
	f := t.get()

	switch f.Kind() {
	case reflect.String:
		return t.setString(v)
	case reflect.Interface:
		return t.set(reflect.ValueOf(v))
	default:
		return fmt.Errorf("cannot assign string to a %s", f.Kind())
	}
}

func setBool(t target, v bool) error {
	f := t.get()

	switch f.Kind() {
	case reflect.Bool:
		return t.setBool(v)
	case reflect.Interface:
		return t.set(reflect.ValueOf(v))
	default:
		return fmt.Errorf("cannot assign bool to a %s", f.String())
	}
}

const maxInt = int64(^uint(0) >> 1)
const minInt = -maxInt - 1

func setInt64(t target, v int64) error {
	f := t.get()

	switch f.Kind() {
	case reflect.Int64:
		return t.setInt64(v)
	case reflect.Int32:
		if v < math.MinInt32 || v > math.MaxInt32 {
			return fmt.Errorf("integer %d does not fit in an int32", v)
		}
		return t.set(reflect.ValueOf(int32(v)))
	case reflect.Int16:
		if v < math.MinInt16 || v > math.MaxInt16 {
			return fmt.Errorf("integer %d does not fit in an int16", v)
		}
		return t.set(reflect.ValueOf(int16(v)))
	case reflect.Int8:
		if v < math.MinInt8 || v > math.MaxInt8 {
			return fmt.Errorf("integer %d does not fit in an int8", v)
		}
		return t.set(reflect.ValueOf(int8(v)))
	case reflect.Int:
		if v < minInt || v > maxInt {
			return fmt.Errorf("integer %d does not fit in an int", v)
		}
		return t.set(reflect.ValueOf(int(v)))

	case reflect.Uint64:
		if v < 0 {
			return fmt.Errorf("negative integer %d cannot be stored in an uint64", v)
		}
		return t.set(reflect.ValueOf(uint64(v)))
	case reflect.Uint32:
		if v < 0 {
			return fmt.Errorf("negative integer %d cannot be stored in an uint32", v)
		}
		if v > math.MaxUint32 {
			return fmt.Errorf("integer %d cannot be stored in an uint32", v)
		}
		return t.set(reflect.ValueOf(uint32(v)))
	case reflect.Uint16:
		if v < 0 {
			return fmt.Errorf("negative integer %d cannot be stored in an uint16", v)
		}
		if v > math.MaxUint16 {
			return fmt.Errorf("integer %d cannot be stored in an uint16", v)
		}
		return t.set(reflect.ValueOf(uint16(v)))
	case reflect.Uint8:
		if v < 0 {
			return fmt.Errorf("negative integer %d cannot be stored in an uint8", v)
		}
		if v > math.MaxUint8 {
			return fmt.Errorf("integer %d cannot be stored in an uint8", v)
		}
		return t.set(reflect.ValueOf(uint8(v)))
	case reflect.Uint:
		if v < 0 {
			return fmt.Errorf("negative integer %d cannot be stored in an uint", v)
		}
		return t.set(reflect.ValueOf(uint(v)))
	case reflect.Interface:
		return t.set(reflect.ValueOf(v))
	default:
		return fmt.Errorf("cannot assign int64 to a %s", f.String())
	}
}

func setFloat64(t target, v float64) error {
	f := t.get()

	switch f.Kind() {
	case reflect.Float64:
		return t.setFloat64(v)
	case reflect.Float32:
		if v > math.MaxFloat32 {
			return fmt.Errorf("float %f cannot be stored in a float32", v)
		}
		return t.set(reflect.ValueOf(float32(v)))
	case reflect.Interface:
		return t.set(reflect.ValueOf(v))
	default:
		return fmt.Errorf("cannot assign float64 to a %s", f.String())
	}
}

// Returns the element at idx of the value pointed at by target, or an error if
// t does not point to an indexable.
// If the target points to an Array and idx is out of bounds, it returns
// (nil, nil) as this is not a fatal error (the unmarshaler will skip).
func elementAt(t target, idx int) (target, error) {
	f := t.get()

	switch f.Kind() {
	case reflect.Slice:
		// TODO: use the idx function argument and avoid alloc if possible.
		idx := f.Len()
		err := t.set(reflect.Append(f, reflect.New(f.Type().Elem()).Elem()))
		if err != nil {
			return nil, err
		}
		return valueTarget(t.get().Index(idx)), nil
	case reflect.Array:
		if idx >= f.Len() {
			return nil, nil
		}
		return valueTarget(f.Index(idx)), nil
	case reflect.Interface:
		if f.IsNil() {
			panic("interface should have been initialized")
		}
		ifaceElem := f.Elem()
		if ifaceElem.Kind() != reflect.Slice {
			return nil, fmt.Errorf("cannot elementAt on a %s", f.Kind())
		}
		idx := ifaceElem.Len()
		newElem := reflect.New(ifaceElem.Type().Elem()).Elem()
		newSlice := reflect.Append(ifaceElem, newElem)
		err := t.set(newSlice)
		if err != nil {
			return nil, err
		}
		return valueTarget(t.get().Elem().Index(idx)), nil
	case reflect.Ptr:
		return elementAt(valueTarget(f.Elem()), idx)
	default:
		return nil, fmt.Errorf("cannot elementAt on a %s", f.Kind())
	}
}

func (d *decoder) scopeTableTarget(append bool, t target, name string) (target, bool, error) {
	x := t.get()

	switch x.Kind() {
	// Kinds that need to recurse

	case reflect.Interface:
		t, err := scopeInterface(append, t)
		if err != nil {
			return t, false, err
		}
		return d.scopeTableTarget(append, t, name)
	case reflect.Ptr:
		t, err := scopePtr(t)
		if err != nil {
			return t, false, err
		}
		return d.scopeTableTarget(append, t, name)
	case reflect.Slice:
		t, err := scopeSlice(append, t)
		if err != nil {
			return t, false, err
		}
		append = false
		return d.scopeTableTarget(append, t, name)
	case reflect.Array:
		t, err := d.scopeArray(append, t)
		if err != nil {
			return t, false, err
		}
		append = false
		return d.scopeTableTarget(append, t, name)

	// Terminal kinds

	case reflect.Struct:
		return scopeStruct(x, name)
	case reflect.Map:
		if x.IsNil() {
			t.set(reflect.MakeMap(x.Type()))
			x = t.get()
		}

		return scopeMap(x, name)
	default:
		panic(fmt.Errorf("can't scope on a %s", x.Kind()))
	}
}

func scopeInterface(append bool, t target) (target, error) {
	err := initInterface(append, t)
	if err != nil {
		return t, err
	}
	return interfaceTarget{t}, nil
}

func scopePtr(t target) (target, error) {
	err := initPtr(t)
	if err != nil {
		return t, err
	}
	return valueTarget(t.get().Elem()), nil
}

func initPtr(t target) error {
	x := t.get()
	if !x.IsNil() {
		return nil
	}
	return t.set(reflect.New(x.Type().Elem()))
}

// initInterface makes sure that the interface pointed at by the target is not
// nil.
// Returns the target to the initialized value of the target.
func initInterface(append bool, t target) error {
	x := t.get()

	if x.Kind() != reflect.Interface {
		panic("this should only be called on interfaces")
	}

	if !x.IsNil() && (x.Elem().Type() == sliceInterfaceType || x.Elem().Type() == mapStringInterfaceType) {
		return nil
	}

	var newElement reflect.Value
	if append {
		newElement = reflect.MakeSlice(sliceInterfaceType, 0, 0)
	} else {
		newElement = reflect.MakeMap(mapStringInterfaceType)
	}
	err := t.set(newElement)
	if err != nil {
		return err
	}

	return nil
}

func scopeSlice(append bool, t target) (target, error) {
	v := t.get()

	if append {
		newElem := reflect.New(v.Type().Elem())
		newSlice := reflect.Append(v, newElem.Elem())
		err := t.set(newSlice)
		if err != nil {
			return t, err
		}
		v = t.get()
	}
	return valueTarget(v.Index(v.Len() - 1)), nil
}

func (d *decoder) scopeArray(append bool, t target) (target, error) {
	v := t.get()

	idx := d.arrayIndex(append, v)

	if idx >= v.Len() {
		return nil, fmt.Errorf("not enough space in the array")
	}

	return valueTarget(v.Index(idx)), nil
}

func scopeMap(v reflect.Value, name string) (target, bool, error) {
	k := reflect.ValueOf(name)

	keyType := v.Type().Key()
	if !k.Type().AssignableTo(keyType) {
		if !k.Type().ConvertibleTo(keyType) {
			return nil, false, fmt.Errorf("cannot convert string into map key type %s", keyType)
		}
		k = k.Convert(keyType)
	}

	if !v.MapIndex(k).IsValid() {
		newElem := reflect.New(v.Type().Elem())
		v.SetMapIndex(k, newElem.Elem())
	}

	return mapTarget{
		v: v,
		k: k,
	}, true, nil
}

type fieldPathsMap = map[string][]int

type fieldPathsCache struct {
	m map[reflect.Type]fieldPathsMap
	l sync.RWMutex
}

func (c *fieldPathsCache) get(t reflect.Type) (fieldPathsMap, bool) {
	c.l.RLock()
	paths, ok := c.m[t]
	c.l.RUnlock()
	return paths, ok
}

func (c *fieldPathsCache) set(t reflect.Type, m fieldPathsMap) {
	c.l.Lock()
	c.m[t] = m
	c.l.Unlock()
}

var globalFieldPathsCache = fieldPathsCache{
	m: map[reflect.Type]fieldPathsMap{},
	l: sync.RWMutex{},
}

func scopeStruct(v reflect.Value, name string) (target, bool, error) {
	// TODO: cache this, and reduce allocations

	fieldPaths, ok := globalFieldPathsCache.get(v.Type())
	if !ok {
		fieldPaths = map[string][]int{}

		path := make([]int, 0, 16)
		var walk func(reflect.Value)
		walk = func(v reflect.Value) {
			t := v.Type()
			for i := 0; i < t.NumField(); i++ {
				l := len(path)
				path = append(path, i)
				f := t.Field(i)
				if f.Anonymous {
					walk(v.Field(i))
				} else if f.PkgPath == "" {
					// only consider exported fields
					fieldName, ok := f.Tag.Lookup("toml")
					if !ok {
						fieldName = f.Name
					}

					pathCopy := make([]int, len(path))
					copy(pathCopy, path)

					fieldPaths[fieldName] = pathCopy
					// extra copy for the case-insensitive match
					fieldPaths[strings.ToLower(fieldName)] = pathCopy
				}
				path = path[:l]
			}
		}

		walk(v)

		globalFieldPathsCache.set(v.Type(), fieldPaths)
	}

	path, ok := fieldPaths[name]
	if !ok {
		path, ok = fieldPaths[strings.ToLower(name)]
	}
	if !ok {
		return nil, false, nil
	}

	return valueTarget(v.FieldByIndex(path)), true, nil
}