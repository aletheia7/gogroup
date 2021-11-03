[![Go Reference](https://pkg.go.dev/badge/github.com/aletheia7/gogroup.svg)](https://pkg.go.dev/github.com/aletheia7/gogroup)

#### Documentation

gogroup allows running a group of goroutines. The gogroup.Group  waits for
all goroutines to end. All goroutines in the group are signaled through a
context to end gracefully when one goroutine ends.

Use Group.Cancel() to cancel all gorountines gracefully.

Group.Interrupted indicates if an os.Signal was received.

Group.Err() indicates if an error was set by a goroutine.

#### Example

```go
package main

import (
	"fmt"
	"github.com/aletheia7/gogroup"
	"log"
	"time"
)

func main() {
	log.SetFlags(log.Lshortfile)
	// ctrl-c to end gracefully or
	// let Object.Run count to 8 for a graceful error
	g := gogroup.New(gogroup.Add_signals(gogroup.Unix))
	// Register()/Unregister() example
	go do(g)
	// Grouper Interface
	g.Go(&Object{})
	defer log.Println("wait done:", g.Wait())
	<-g.Context.Done()
}

func do(g *gogroup.Group) {
	defer log.Println("do done")
	key := g.Register()
	defer g.Unregister(key)
	for {
		select {
		case <-g.Done():
			return
		case <-time.After(time.Second):
			log.Println("do")
		}
	}
}

type Object struct{}

func (o *Object) Run(g *gogroup.Group) {
	defer log.Println("Object.Run done")
	ct := 0
	total := 8
	for {
		select {
		case <-g.Done():
			return
		case t := <-time.After(time.Second):
			ct++
			log.Println("run:", ct, "of", total, t.Format("3:04:05 pm"))
			if ct == total {
				g.Set_err(fmt.Errorf("some err"))
				return
			}
		}
	}
}
```

#### License 

Use of this source code is governed by a BSD-2-Clause license that can be
found in the LICENSE file.

[![BSD-2-Clause License](img/osi_logo_100X133_90ppi_0.png)](https://opensource.org/)
