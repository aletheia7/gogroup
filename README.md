[![Go Reference](https://pkg.go.dev/badge/github.com/aletheia7/gogroup.svg)](https://pkg.go.dev/github.com/aletheia7/gogroup)

#### Documentation

gogroup allows running a group of goroutines. The gogroup.Group waits for
all goroutines to end. All goroutines in the group are signaled through a
context to end gracefully when one goroutine ends.

Use Group.Cancel() to cancel all gorountines gracefully.

Use <ctrl-c> to cancel all goroutines gracefully.

Group.Interrupted indicates if an os.Signal was received.

Group.Get_err() indicates if an error was set by a goroutine.

#### Example

```go
package main

import (
	"fmt"
	"log"
	"time"

	"github.com/aletheia7/gogroup"
)

var gg = gogroup.New()

func main() {
	log.SetFlags(log.Lshortfile)
	go do_1()
	go do_2()
	defer log.Println("main done")
	// gg.Wait() will wait for all goroutines
	// <ctrl-c> will cancel all goroutines
	gg.Wait()
	log.Printf("main error: %v, interrupted: %v\n", gg.Get_err(), gg.Interrupted)
}

func do_1() {
	defer log.Println("do_1: done")
	key := gg.Register()
	defer gg.Unregister(key)
	gg.Set_err(fmt.Errorf("do_1 error"))
	child_gg := gogroup.New(gogroup.With_cancel(gg))
	// parent gg.Cancel() will call child_gg.Cancel() to cleanup hierarchy
	<-child_gg.Done()
	log.Println("do_1 child_gg.Cancel() called by parent")
	<-gg.Done()
}

func do_2() {
	defer log.Println("do_2: done")
	key := gg.Register()
	defer gg.Unregister(key)
	for ct, total := 1, 2; ct <= total; ct++ {
		select {
		case <-time.After(time.Second):
			log.Println("do_2: tick", ct, "of", total)
		case <-gg.Done():
			return
		}
	}
	// Set_err() was called earlier in do_1().
	// It will be ignored.
	gg.Set_err(fmt.Errorf("do_2 error"))
}
```
#### Output
```
t.go:43: do_2: tick 1 of 2
t.go:43: do_2: tick 2 of 2
t.go:51: do_2: done
t.go:32: do_1 child_gg.Cancel() called by parent
t.go:34: do_1: done
t.go:21: main error: do_1 error, interrupted: false
t.go:22: main done
```

#### License 

Use of this source code is governed by a BSD-2-Clause license that can be
found in the LICENSE file.

[![BSD-2-Clause License](img/osi_logo_100X133_90ppi_0.png)](https://opensource.org/)
