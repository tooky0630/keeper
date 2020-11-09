package keeper

import (
    "errors"
    "fmt"
    "reflect"
    "strings"
    "unsafe"
)

const (
    _nameTag = "name"
    _optionalTag = "optional"
)

// A batch of initial action to be invoked after bean`s reference injected.
type Initializer interface {
    AfterPropertySet()
}

// Option configures a Container. It's included for future functionality;
// currently, there are no concrete implementations.
type Option interface {
    applyOption(*Container)
}

type optionFunc func(*Container)

func (f optionFunc) applyOption(c *Container) { f(c) }

var noopRegisterOption registerOptions

// options for bean register
type registerOptions struct {
    Name  string
}

func (opt registerOptions) Validate() error {
    if opt.Name == "" {
        return errors.New("cannot use empty name")
    }
    if strings.ContainsRune(opt.Name, '`') {
        return fmt.Errorf("invalid Name(%q): names cannot contain backquotes", opt.Name)
    }
    return nil
}

// A RegisterOption modifies the default behavior of Register.
type RegisterOption interface {
    applyRegisterOption(opts *registerOptions)
}

type registerOptionFunc func(*registerOptions)

func (f registerOptionFunc) applyRegisterOption(opts *registerOptions) { f(opts) }

// Name is a RegisterOption that specifies that all values produced by a
// bean should have the given name.
//
// Given,
//
//   type Connection struct {}
//
// The following will provide two connections to the container: one under the
// name "ro" and the other under the name "rw".
//
//   c.Register(new(Connection), keeper.Name("ro"))
//   c.Register(new(Connection), keeper.Name("rw"))
//
// This option cannot be provided for constructors which produce result
// objects.
func Name(name string) RegisterOption {
    return registerOptionFunc(func(options *registerOptions) {
        options.Name = name
    })
}

type Keeper interface {
    // find the bean of the name
    Find(name string) interface{}
    // get copy of all bean map
    All() map[string]interface{}
    // inject of node`s dependence, but not register
    Provider(ptr interface{}) error
    // reject the dependence and register it
    Register(ptr interface{}, opts ...RegisterOption) error
}

func New(opts ...Option) Keeper {
    c := &Container{
        nodes: make(map[string]interface{}),
    }
    for _, opt := range opts {
        opt.applyOption(c)
    }
    return c
}

// Container defines the behavior of the manager for members and their dependencies.
// Container is an application level global context, in most cases, only one take effect in the app.
type Container struct {
    nodes map[string]interface{}
}

func (c *Container) Find(name string) interface{} {
    return c.nodes[name]
}

func (c *Container) All() map[string]interface{} {
    cm := make(map[string]interface{}, len(c.nodes))
    for name, bean := range c.nodes {
        cm[name] = bean
    }
    return cm
}

func (c *Container) Provider(ptr interface{}) error {
    return c.load(ptr, noopRegisterOption)
}

func (c *Container) Register(node interface{}, opts ...RegisterOption) error {
    var options registerOptions
    for _, o := range opts {
        o.applyRegisterOption(&options)
    }
    if err := options.Validate(); err != nil {
        return err
    }
    _, exist := c.nodes[options.Name]
    if exist {
        return fmt.Errorf("register duplicate! %s already register by %s", options.Name, reflect.TypeOf(node).Name())
    }
    if reflect.TypeOf(node).Kind() == reflect.Ptr { // ptr needs to inject dependence
        err := c.load(node, options)
        if err != nil {
            return err
        }
    }
    c.nodes[options.Name] = node // normal node
    return nil
}

// not thread safe
func (c *Container) load(ptr interface{}, _ registerOptions) error {
    typ := reflect.TypeOf(ptr)
    if typ == nil {
        return errors.New("can't provide an untyped nil")
    }
    if typ.Kind() != reflect.Ptr {
        return fmt.Errorf("must provide pointer of bean, got %v (type %v)", ptr, typ)
    }
    typ = typ.Elem()
    val := reflect.ValueOf(ptr).Elem()
    for i := 0; i < typ.NumField();i++ {
        fv := val.Field(i)
        tv := typ.Field(i)
        tag, ok := tv.Tag.Lookup(_nameTag)
        if !ok {
            continue
        }
        depOpts := strings.Split(tag, ",")
        name := depOpts[0]
        var optional bool
        if len(depOpts) > 1 && depOpts[1] == _optionalTag {
            optional = true
        }
        elem := c.Find(name)
        if elem == nil {
            if optional {
                continue
            }
            return fmt.Errorf("failed to load %s", name)
        }
        fv = reflect.NewAt(fv.Type(), unsafe.Pointer(fv.UnsafeAddr())).Elem()
        nv := reflect.ValueOf(elem).Elem()
        if nv.Kind() != fv.Kind() {
            return fmt.Errorf("reference %s@%s kind not suit to %s.%s@%s",
                name, reflect.TypeOf(elem).Name(), typ.Name(), tv.Name, fv.Type().Name())
        }
        fv.Set(nv)
    }
    if initializer, ok := ptr.(Initializer); ok {
        initializer.AfterPropertySet()
    }
    return nil
}
