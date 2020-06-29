package frr

import (
	"fmt"
	"io"
)

func indent(w io.Writer, depth int) {
	for i := 0; i < depth; i++ {
		fmt.Fprint(w, " ")
	}
}

func writeWithIndent(w io.Writer, depth int, s string) {
	indent(w, depth)
	fmt.Fprintln(w, s)
}

func (r RouteRedistribution) Write(w io.Writer, indent int) {
	if r.Connected {
		writeWithIndent(w, indent, "redistribute connected")
	}
	if r.Static {
		writeWithIndent(w, indent, "redistribute static")
	}
	if r.OSPF {
		writeWithIndent(w, indent, "redistribute ospf")
	}
	if r.ISIS {
		writeWithIndent(w, indent, "redistribute isis")
	}
}

func (c *FRRConfig) internalIfs() map[string]IfConfig {
	res := make(map[string]IfConfig, len(c.Interfaces))
	for n, i := range c.Interfaces {
		if !i.External {
			res[n] = i
		}
	}
	return res
}

func (c *FRRConfig) externalIfs() map[string]IfConfig {
	res := make(map[string]IfConfig, len(c.Interfaces))
	for n, i := range c.Interfaces {
		if i.External {
			res[n] = i
		}
	}
	return res
}