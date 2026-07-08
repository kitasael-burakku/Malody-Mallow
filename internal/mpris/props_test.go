package mpris

import (
	"sync"
	"testing"

	"github.com/godbus/dbus/v5"
	"github.com/godbus/dbus/v5/prop"
)

// newTestProps arma un properties sin bus (conn nil): las entradas van sin
// emit para que SetMust no intente emitir señales.
func newTestProps(set func(any) *dbus.Error) *properties {
	return &properties{
		path: objPath,
		m: map[string]map[string]*propDef{
			playerIface: {
				"Metadata": {value: map[string]dbus.Variant{
					"xesam:title": dbus.MakeVariant("vieja"),
					"xesam:album": dbus.MakeVariant("Demos"),
				}},
				"Volume":         {value: 0.5, set: set},
				"PlaybackStatus": {value: "Stopped"},
			},
		},
	}
}

// TestPropsReplaceDropsStaleKeys cubre el bug de godbus/prop que motivó esta
// implementación: su Set escribía dentro del mapa publicado sin borrar
// claves, y el álbum de la pista anterior sobrevivía al cambio de Metadata.
func TestPropsReplaceDropsStaleKeys(t *testing.T) {
	p := newTestProps(nil)
	old, derr := p.Get(playerIface, "Metadata")
	if derr != nil {
		t.Fatal(derr)
	}

	// pista nueva sin álbum: el mapa se reemplaza entero
	p.SetMust(playerIface, "Metadata", map[string]dbus.Variant{
		"xesam:title": dbus.MakeVariant("nueva"),
	})
	got, derr := p.Get(playerIface, "Metadata")
	if derr != nil {
		t.Fatal(derr)
	}
	m := got.Value().(map[string]dbus.Variant)
	if _, ok := m["xesam:album"]; ok {
		t.Error("xesam:album sobrevivió al reemplazo de Metadata")
	}
	if m["xesam:title"].Value() != "nueva" {
		t.Errorf("title = %v", m["xesam:title"].Value())
	}
	// y el mapa viejo (que un GetAll en vuelo podría estar codificando)
	// quedó intacto
	if om := old.Value().(map[string]dbus.Variant); om["xesam:title"].Value() != "vieja" {
		t.Errorf("el mapa viejo fue mutado: %v", om)
	}
}

func TestPropsGetAllAndErrors(t *testing.T) {
	p := newTestProps(nil)
	all, derr := p.GetAll(playerIface)
	if derr != nil || len(all) != 3 {
		t.Fatalf("GetAll: %v / %d entradas", derr, len(all))
	}
	if _, derr := p.Get("no.existe", "Volume"); derr == nil {
		t.Error("Get con interfaz desconocida no devolvió error")
	}
	if _, derr := p.Get(playerIface, "NoExiste"); derr == nil {
		t.Error("Get con propiedad desconocida no devolvió error")
	}
	if _, derr := p.GetAll("no.existe"); derr == nil {
		t.Error("GetAll con interfaz desconocida no devolvió error")
	}
}

func TestPropsSet(t *testing.T) {
	var got any
	p := newTestProps(func(v any) *dbus.Error {
		if _, ok := v.(float64); !ok {
			return prop.ErrInvalidArg
		}
		got = v
		return nil
	})

	// escritura válida: pasa por el setter y publica el valor
	if derr := p.Set(playerIface, "Volume", dbus.MakeVariant(0.8)); derr != nil {
		t.Fatalf("Set: %v", derr)
	}
	if got != 0.8 {
		t.Errorf("el setter recibió %v", got)
	}
	if v, _ := p.Get(playerIface, "Volume"); v.Value() != 0.8 {
		t.Errorf("Volume publicado = %v", v.Value())
	}

	// el setter rechaza: el valor publicado no cambia
	if derr := p.Set(playerIface, "Volume", dbus.MakeVariant("alto")); derr == nil {
		t.Error("Set con tipo inválido no devolvió error")
	}
	if v, _ := p.Get(playerIface, "Volume"); v.Value() != 0.8 {
		t.Errorf("Volume tras Set inválido = %v", v.Value())
	}

	// propiedad sin setter: solo lectura
	if derr := p.Set(playerIface, "PlaybackStatus", dbus.MakeVariant("Playing")); derr == nil {
		t.Error("Set sobre propiedad de solo lectura no devolvió error")
	}
}

func TestPropsIntrospection(t *testing.T) {
	p := newTestProps(func(any) *dbus.Error { return nil })
	access := map[string]string{}
	for _, ip := range p.Introspection(playerIface) {
		access[ip.Name] = ip.Access
	}
	if access["Volume"] != "readwrite" || access["Metadata"] != "read" {
		t.Errorf("accesos: %v", access)
	}
}

// TestPropsConcurrent martillea SetMust contra Get/GetAll: con -race habría
// cazado la carrera de godbus/prop (mutación del mapa publicado mientras
// una respuesta en vuelo lo codifica).
func TestPropsConcurrent(t *testing.T) {
	p := newTestProps(nil)
	var wg sync.WaitGroup
	for w := 0; w < 4; w++ {
		wg.Add(2)
		go func() {
			defer wg.Done()
			for i := 0; i < 500; i++ {
				p.SetMust(playerIface, "Metadata", map[string]dbus.Variant{
					"xesam:title": dbus.MakeVariant("t"),
				})
			}
		}()
		go func() {
			defer wg.Done()
			for i := 0; i < 500; i++ {
				v, _ := p.Get(playerIface, "Metadata")
				// recorrer el mapa simula la codificación de la respuesta
				for k := range v.Value().(map[string]dbus.Variant) {
					_ = k
				}
				p.GetAll(playerIface)
			}
		}()
	}
	wg.Wait()
}
