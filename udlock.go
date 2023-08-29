package main

import (
	"fmt"
	"net"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// UDLock uses a unix-domain socket to lock
// a file to prevent two cooperating processing from
// using the same file.
type UDLock struct {
	Path     string
	lsn      net.Listener
	Finished chan struct{}
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

	path2, err0 := filepath.Abs(path)
	panicOn(err0)
	path = path2

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
		defer close(lock.Finished)
		for {
			//vv("top of UDLock.start() loop")

			// Accept new connections, tell them the lock is held,
			// and check if we are shutting down
			conn, err := lock.lsn.Accept()
			if err != nil {
				//vv("lsn.Accept error (probably closed due to shutdown): '%v'", err)
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
				n, err := conn.Read(buf[:])
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
