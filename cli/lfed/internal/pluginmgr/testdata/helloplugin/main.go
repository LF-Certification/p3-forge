// Command helloplugin is a minimal lfed plugin used by pluginmgr tests.
//
// `__metadata` prints a manifest. `fail` exits 3. `kill` kills itself.
// `wait` prints "ready" and blocks until a signal ends it. Any other
// invocation prints the LFED_* environment and the arguments.
package main

import (
	"fmt"
	"os"
	"sort"
	"strings"
	"time"
)

const manifest = `{
  "schema_version": "1",
  "name": "hello",
  "version": "0.0.1",
  "summary": "Test plugin for pluginmgr",
  "requires_lfed": ">=0.1.0 <1",
  "commands": [{"path": ["greet"], "summary": "Print the environment"}]
}
`

func main() {
	if len(os.Args) > 1 {
		switch os.Args[1] {
		case "__metadata":
			fmt.Print(manifest)
			return
		case "fail":
			os.Exit(3)
		case "kill":
			p, _ := os.FindProcess(os.Getpid())
			_ = p.Kill()
			time.Sleep(time.Hour)
		case "wait":
			fmt.Println("ready")
			time.Sleep(time.Hour)
		}
	}
	var env []string
	for _, kv := range os.Environ() {
		if strings.HasPrefix(kv, "LFED_") {
			env = append(env, kv)
		}
	}
	sort.Strings(env)
	for _, kv := range env {
		fmt.Println(kv)
	}
	fmt.Println("args:", strings.Join(os.Args[1:], " "))
}
