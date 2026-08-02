// Command fakeextractor is a minimal gofi-graph extractor, used by the tests to
// exercise the external path end to end. It ignores --root and invents a tiny
// graph, so the test proves the plumbing rather than any parsing.
package main

import (
	"flag"
	"fmt"
)

func main() {
	root := flag.String("root", "", "repository root")
	mode := flag.String("mode", "fast", "fast|deep")
	flag.Parse()

	fmt.Printf("{\"rec\":\"header\",\"schema\":\"gofi-graph/v1\",\"language\":\"fake\",\"module\":\"com.acme.app\",\"tool\":\"fakeextractor 0.1.0\",\"mode\":%q}\n", *mode)
	fmt.Println(`{"rec":"node","id":"fake:com.acme.api","kind":"package","name":"com.acme.api","file":"src/api","lines":40}`)
	fmt.Println(`{"rec":"node","id":"fake:com.acme.api.Server","kind":"class","name":"Server","unit":"fake:com.acme.api","file":"src/api/Server.java","line":12,"vis":"public"}`)
	fmt.Println(`{"rec":"node","id":"fake:com.acme.api.Server#start","kind":"method","name":"start","unit":"fake:com.acme.api","owner":"Server","file":"src/api/Server.java","line":31,"vis":"public"}`)
	fmt.Println(`{"rec":"edge","from":"fake:com.acme.api","to":"fake:com.acme.api.Server","rel":"contains","file":"src/api/Server.java","line":12,"conf":1}`)
	fmt.Println(`{"rec":"edge","from":"fake:com.acme.api","to":"fake:com.acme.api.Server#start","rel":"contains","file":"src/api/Server.java","line":31,"conf":1}`)
	fmt.Println(`{"rec":"edge","from":"fake:com.acme.api.Server#start","to":"fake:com.acme.api.Server","rel":"uses","conf":0.7}`)
	fmt.Printf("{\"rec\":\"diag\",\"severity\":\"warn\",\"msg\":\"root was %s\"}\n", *root)
	fmt.Println(`{"rec":"summary","files":2,"loc":40,"unresolved":1,"ambiguous":0}`)
}
