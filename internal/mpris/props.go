package mpris

import (
	"sync"

	"github.com/godbus/dbus/v5"
	"github.com/godbus/dbus/v5/introspect"
	"github.com/godbus/dbus/v5/prop"
)

// propDef describe una propiedad exportada; set != nil la hace escribible.
type propDef struct {
	value any
	emit  bool                  // emitir PropertiesChanged al cambiar
	set   func(any) *dbus.Error // escritura de un cliente externo
}

// properties es un reemplazo mínimo de godbus/prop, que tiene dos bugs con
// propiedades tipo mapa (nuestro Metadata): su Set escribe DENTRO del mapa
// ya publicado —una respuesta a GetAll en vuelo lo codifica fuera del lock:
// data race, y puede tumbar el proceso—, y nunca borra claves, así que el
// álbum/género de la pista anterior quedaba pegado al pasar a una sin tags.
// Aquí los valores se REEMPLAZAN, jamás se mutan: el lector en vuelo ve el
// valor viejo intacto y el mapa nuevo llega entero.
type properties struct {
	conn *dbus.Conn
	path dbus.ObjectPath

	mu sync.Mutex
	m  map[string]map[string]*propDef
}

// exportProps publica org.freedesktop.DBus.Properties en path. En el bus
// solo son visibles Get, GetAll y Set: el resto de métodos no tiene la
// firma que dbus exporta (último retorno *dbus.Error).
func exportProps(conn *dbus.Conn, path dbus.ObjectPath, spec map[string]map[string]*propDef) (*properties, error) {
	p := &properties{conn: conn, path: path, m: spec}
	return p, conn.Export(p, path, "org.freedesktop.DBus.Properties")
}

// find requiere p.mu tomado.
func (p *properties) find(iface, name string) (*propDef, *dbus.Error) {
	props, ok := p.m[iface]
	if !ok {
		return nil, prop.ErrIfaceNotFound
	}
	e, ok := props[name]
	if !ok {
		return nil, prop.ErrPropNotFound
	}
	return e, nil
}

// Get implementa org.freedesktop.DBus.Properties.Get.
func (p *properties) Get(iface, name string) (dbus.Variant, *dbus.Error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	e, derr := p.find(iface, name)
	if derr != nil {
		return dbus.Variant{}, derr
	}
	return dbus.MakeVariant(e.value), nil
}

// GetAll implementa org.freedesktop.DBus.Properties.GetAll.
func (p *properties) GetAll(iface string) (map[string]dbus.Variant, *dbus.Error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	props, ok := p.m[iface]
	if !ok {
		return nil, prop.ErrIfaceNotFound
	}
	out := make(map[string]dbus.Variant, len(props))
	for name, e := range props {
		out[name] = dbus.MakeVariant(e.value)
	}
	return out, nil
}

// Set implementa org.freedesktop.DBus.Properties.Set. El setter corre SIN
// p.mu: puede disparar comandos del demonio que terminen en SetMust sin
// interbloquearse. Si acepta, el valor se publica ya; el Update posterior
// del demonio lo confirma (o corrige).
func (p *properties) Set(iface, name string, v dbus.Variant) *dbus.Error {
	p.mu.Lock()
	e, derr := p.find(iface, name)
	p.mu.Unlock()
	if derr != nil {
		return derr
	}
	if e.set == nil {
		return prop.ErrReadOnly
	}
	if derr := e.set(v.Value()); derr != nil {
		return derr
	}
	p.SetMust(iface, name, v.Value())
	return nil
}

// SetMust publica un valor nuevo (reemplazo, nunca mutación) y emite
// PropertiesChanged si la propiedad lo pide. Una propiedad inexistente es
// un error de programación: panic, como el SetMust de godbus/prop.
func (p *properties) SetMust(iface, name string, v any) {
	p.mu.Lock()
	e, derr := p.find(iface, name)
	if derr != nil {
		p.mu.Unlock()
		panic(derr)
	}
	e.value = v
	emit := e.emit
	p.mu.Unlock()
	if emit {
		p.conn.Emit(p.path, "org.freedesktop.DBus.Properties.PropertiesChanged",
			iface, map[string]dbus.Variant{name: dbus.MakeVariant(v)}, []string{})
	}
}

// Introspection describe las propiedades de iface para node().
func (p *properties) Introspection(iface string) []introspect.Property {
	p.mu.Lock()
	defer p.mu.Unlock()
	out := make([]introspect.Property, 0, len(p.m[iface]))
	for name, e := range p.m[iface] {
		access := "read"
		if e.set != nil {
			access = "readwrite"
		}
		out = append(out, introspect.Property{
			Name: name, Type: dbus.SignatureOf(e.value).String(), Access: access,
		})
	}
	return out
}
