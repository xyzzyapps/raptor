package gcre

import "sync"

// HostFunc is a host-language matcher. It is the organic escape hatch:
// a .raku grammar names <HOST_foo> and Go registers foo.
// The form is still a Raku angle-call (a subrule name); gcre intercepts
// the HOST_ prefix instead of requiring a rule body.
type HostFunc func(g *Grammar, ctx *Context, cap *Match) bool

var (
	hostMu sync.RWMutex
	hosts  = map[string]HostFunc{}
)

// RegisterHost binds a HOST_* name used in .raku files.
func RegisterHost(name string, fn HostFunc) {
	hostMu.Lock()
	defer hostMu.Unlock()
	hosts[name] = fn
}

func lookupHost(name string) HostFunc {
	hostMu.RLock()
	defer hostMu.RUnlock()
	return hosts[name]
}

type rxHost struct {
	name    string
	capture bool
	quant   byte
}

func (r *rxHost) match(g *Grammar, ctx *Context, cap *Match, token bool) bool {
	once := func() bool {
		if !token {
			ctx.SkipWS()
		}
		fn := lookupHost(r.name)
		if fn == nil {
			return false
		}
		start := ctx.Pos
		m := NewMatch("", start, start, true)
		if !fn(g, ctx, m) {
			ctx.Pos = start
			return false
		}
		m.To = ctx.Pos
		m.Ok = true
		if m.Str == "" {
			m.Str = string(ctx.Src[start:ctx.Pos])
		}
		if r.capture && cap != nil {
			cap.AddNamed("HOST_"+r.name, m)
			if m.Made != nil {
				cap.Make(m.Made)
			}
		}
		return true
	}
	return applyQuant(r.quant, ctx, once)
}
