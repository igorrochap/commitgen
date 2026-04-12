package loading

import (
	"fmt"
	"time"
)

func Start(done <-chan struct{}) {
	go func() {
		frames := []string{"/", "|", "-", "\\"}
		for {
			select {
			case <-done:
				return
			default:
				for _, f := range frames {
					fmt.Printf("\rGenerating... %s", f)
					time.Sleep(100 * time.Millisecond)
				}
			}
		}
	}()
}
