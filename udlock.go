package main

import (
	"fmt"
	"net"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

var _ = filepath.Abs

// UDLock uses a unix-domain socket to lock
// a file to prevent two cooperating processing from
// using the same file.
type UDLock struct {
	Path     string
	lsn      net.Listener
	Finished chan struct{}

	mut    sync.Mutex
	isDone bool
}

// NewUDLock obtains a lock based on path. If the
// lock was obtained, a nil err will be returned.
// path will have ".lock" appended if it does not
// already have it. Call lock.Close() to release
// the lock.
func NewUDLock(path string) (lock *UDLock, err error) {

	if strings.HasSuffix(path, ".lock") {
		// okay as is.
	} else {
		path = path + ".lock"
	}

	// maybe path getting too long? try not extending it, shortening instead.
	//path2, err0 := filepath.Abs(path)
	//panicOn(err0)
	//path = path2
	//
	// yes, without this path shortening, we can get spurious errors with
	// a long path, like
	// udlock.go:76 2023-09-11T10:41:41.364-05:00 net.Listen("unix", path='/home/jaten/models/dtn_calculated_indi/slow_small_careful_2023july02/simple_thresh/out.q10.dir/SPY.simtrade/my.rbook.rog.lock') give err2='listen unix /home/jaten/models/dtn_calculated_indi/slow_small_careful_2023july02/simple_thresh/out.q10.dir/SPY.simtrade/my.rbook.rog.lock: bind: invalid argument'
	//
	path = filepath.Base(path)

	staleLock := false
	if FileExists(path) {
		// see if process at this unix domain socket is
		// still alive and responding to lock queries, or
		// if the .lock file is just stale and we can
		// grab it.
		conn, err1 := net.Dial("unix", path)
		//vv("dial err1 = '%v' on path '%v'", err1, path)
		if err1 == nil {
			var buf [4096]byte
			err2 := conn.SetReadDeadline(time.Now().Add(3 * time.Second))
			panicOn(err2)
			n, err3 := conn.Read(buf[:])
			if err3 != nil {
				if netErr, ok := err3.(net.Error); ok && netErr.Timeout() {
					staleLock = true
				}
			}
			if staleLock {
				os.Remove(path)
			} else {
				err = fmt.Errorf("path '%v' has a live lock already: '%v'", path, string(buf[:n]))
				return
			}
		} else {
			//vv("removing stale socket file b/c could not contact the process holding it")
			os.Remove(path)
		}
	}
	lsn, err2 := net.Listen("unix", path)
	if err2 != nil {
		vv(`net.Listen("unix", path='%v') give err2='%v'`, path, err2)
		return nil, err2
	}

	lock = &UDLock{
		Path:     path,
		lsn:      lsn,
		Finished: make(chan struct{}),
	}
	lock.start()
	return lock, nil
}

// Close releases the lock
func (lock *UDLock) Close() {
	lock.mut.Lock()
	done := lock.isDone
	lock.mut.Unlock()
	if done {
		os.Remove(lock.Path)
		return
	}
	conn, err1 := net.DialTimeout("unix", lock.Path, 5*time.Second)

	if err1 == nil {
		conn.SetReadDeadline(time.Now().Add(time.Second))
		var buf [4096]byte
		conn.Read(buf[:])

		conn.Write([]byte("shutdown"))
		<-lock.Finished
	}
	os.Remove(lock.Path)
}

func (lock *UDLock) start() {
	go func() {
		defer func() {
			lock.mut.Lock()
			lock.isDone = true
			close(lock.Finished)
			lock.mut.Unlock()
		}()
		for {
			//vv("top of UDLock.start() loop")

			// Accept new connections, tell them the lock is held,
			// and check if we are shutting down
			conn, err := lock.lsn.Accept()
			if err != nil {
				// ex: 'accept unix my.rbook.rog.lock: use of closed network connection'
				vv("lsn.Accept error (probably closed due to shutdown): '%v'", err)
				return
			}

			go func(conn net.Conn) {
				//vv("Client connected [%s]", conn.RemoteAddr().Network())

				err = conn.SetWriteDeadline(time.Now().Add(5 * time.Second))
				panicOn(err)
				fmt.Fprintf(conn, "'%v' locked! by pid:%v", lock.Path, os.Getpid())

				// check if is request to shutdown
				err = conn.SetReadDeadline(time.Now().Add(5 * time.Second))
				panicOn(err)
				var buf [4096]byte
				n, err := conn.Read(buf[:len("shutdown")])
				if err == nil && n >= len("shutdown") {
					if string(buf[:len("shutdown")]) == "shutdown" {
						conn.Close()
						//vv("exit lock listening loop by closing lsn.")
						lock.lsn.Close()
						return
					}
				}
				conn.Close()
			}(conn)
		}
	}()
}
