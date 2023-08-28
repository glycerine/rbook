package main

import (
	"fmt"
	"net"
	"os"
	"testing"
	"time"

	cv "github.com/glycerine/goconvey/convey"
)

func TestUDLock(t *testing.T) {

	cv.Convey("UDLock should provide cooperating rbook processes with file locking on the my.rbook files so we do not have over-write/accidental truncation issues if two processes try to start in the same directory", t, func() {

		path := "testlock.lock"
		os.Remove(path)
		time.Sleep(10 * time.Millisecond)

		// check that we handle cleanup of a stale lock from a crashed process.
		lsn, err := net.Listen("unix", path)
		panicOn(err)
		if !FileExists(path) {
			panic(fmt.Sprintf("path '%v' lockfile should have been left in the filesystem!", path))
		}
		vv("good, '%v', our expected unix domain socket was found.", path)
		lsn.Close() // removes the file.

		lock, err := NewUDLock(path)
		panicOn(err)
		_ = lock
		vv("started new UDLock")

		// try to get the lock 5 times.
		for i := 0; i < 5; i++ {
			lock2, err := NewUDLock(path)
			if err == nil {
				panic("expected to have error here due to path already being locked")
			}
			vv("good, lock2 clould not be obtained; i = %v", i)
			_ = lock2
		}
		lock.Close()
		vv("good: back from lock.Close()")

		// now we should be able to get the lock again.
		lock, err = NewUDLock(path)
		panicOn(err)
		vv("good: able to get the lock again now that it is not held")
		lock.Close()
		if FileExists(path) {
			panic("should have cleaned up the lock file!")
		}
		vv("good: all done")
		cv.So(true, cv.ShouldBeTrue)
	})
}
