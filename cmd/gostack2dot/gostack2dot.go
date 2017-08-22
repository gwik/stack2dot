package main

import (
	"bufio"
	"fmt"
	"io/ioutil"
	"log"
	"math"
	"os"
	"strconv"
	"strings"

	"github.com/maruel/panicparse/stack"
)

type attribute struct {
	Name  string
	Value string
}

type attributes []attribute

func (attrs attributes) String() string {
	buf := ""
	for i, a := range attrs {
		if i > 0 {
			buf += "," + a.Name + "=" + dotQuote(a.Value)
		} else {
			buf += a.Name + "=" + dotQuote(a.Value)
		}
	}
	return buf
}

type edge struct {
	Source int
	Target int
}

type node struct {
	ID     int
	Label  string
	Weight int
	Call   *stack.Call
}

func dotQuote(s string) string {
	return strings.Replace(s, `"`, `\"`, -1)
}

func edgeDot(out *output, source, target string, attrs ...attribute) {
	if len(attrs) > 0 {
		out.String(source + " -> " + target + "[" + attributes(attrs).String() + "];\n")
	} else {
		out.String(source + " -> " + target + ";\n")
	}
}

func nodeDot(out *output, id string, attrs ...attribute) {
	out.String(id)
	out.Byte(' ')
	if len(attrs) > 0 {
		out.Byte('[')
	}
	for i, a := range attrs {
		if i > 0 {
			out.Byte(',')
		}
		out.String(a.Name)
		out.String(`="`)
		out.String(dotQuote(a.Value))
		out.Byte('"')
	}
	if len(attrs) > 0 {
		out.Byte(']')
	}
	out.String(";\n")
}

type output bufio.Writer

func (w *output) String(s string) {
	if _, err := (*bufio.Writer)(w).WriteString(s); err != nil {
		log.Fatalf("failed to write to ouput: %v", err)
	}
}

func (w *output) Byte(b byte) {
	if err := (*bufio.Writer)(w).WriteByte(b); err != nil {
		log.Fatalf("failed to write to ouput: %v", err)
	}
}

func main() {
	goroutines, err := stack.ParseDump(os.Stdin, ioutil.Discard)
	if err != nil {
		log.Fatal(err)
	}

	buf := bufio.NewWriter(os.Stdout)
	defer buf.Flush()

	out := (*output)(buf)

	out.String("digraph goroutines {\n")
	out.String("node [style=filled fillcolor=\"#f8f8f8\"];\n")

	var nodes []node
	sourceMap := make(map[string]int)
	edges := make(map[edge]int)
	max := 1

	for _, g := range goroutines {
		var lastNode *node
		for _, c := range g.Stack.Calls {
			var (
				id int
				ok bool
			)
			if id, ok = sourceMap[c.FullSourceLine()]; !ok {
				id = len(nodes)
				sourceMap[c.FullSourceLine()] = id
				nodes = append(nodes, node{
					ID:     id,
					Label:  c.Func.PkgDotName(),
					Weight: 1,
					Call:   &c,
				})
			} else {
				nodes[id].Weight++
				if nodes[id].Weight > max {
					max = nodes[id].Weight
				}
			}

			if lastNode != nil {
				edges[edge{Source: id, Target: lastNode.ID}]++
			}

			lastNode = &nodes[id]
		}
	}

	total := len(goroutines)

	for _, n := range nodes {
		label := fmt.Sprintf("%s\n%d of %d (%0.2f%%)", n.Label, n.Weight, total, float64(n.Weight)/float64(total)*100)
		// Scale font sizes from 8 to 24 based on percentage of flat frequency.
		// Use non linear growth to emphasize the size difference.
		baseFontSize, maxFontGrowth := 8, 16.0
		fontSize := baseFontSize
		if max > 0 && n.Weight > 0 && float64(n.Weight) <= float64(max) {
			fontSize += int(math.Ceil(maxFontGrowth * math.Sqrt(float64(n.Weight)/float64(max))))
		}
		nodeDot(
			out,
			fmt.Sprintf("N%d", n.ID),
			attribute{Name: "fontsize", Value: strconv.Itoa(fontSize)},
			attribute{Name: "label", Value: label},
			attribute{Name: "shape", Value: "box"},
		)
	}

	var scratch [3]attribute
	for e, w := range edges {
		attrs := scratch[:1]
		attrs[0].Name = "label"
		attrs[0].Value = strconv.Itoa(w)

		if weight := 1 + w*100/total; weight > 1 {
			attrs = append(attrs, attribute{Name: "weight", Value: strconv.Itoa(weight)})
		}
		if width := 1 + w*10/total; width > 1 {
			attrs = append(attrs, attribute{Name: "penwidth", Value: strconv.Itoa(width)})
		}
		edgeDot(out, fmt.Sprintf("N%d", e.Source), fmt.Sprintf("N%d", e.Target), attrs...)
	}

	out.String("}\n") // closes digraph
}
