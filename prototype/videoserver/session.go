// rtER Project - SRL, McGill University, 2013
//
// Author: echa@cim.mcgill.ca

package main

import (
//	"io/ioutil"
	"net/http"
	"syscall"
	"runtime"
	"strings"
	"strconv"
	"time"
	"log"
	"os"
	"io"
)

// Transcoder states
const (
	TC_INIT		int = 0
	TC_RUNNING	int = 1
	TC_EOS		int = 2
	TC_FAILED	int = 3
)

type TranscodeSession struct {
	Server		*ServerState	// link to server used for signalling session state
	UID			uint64			// video UID
	Type		int				// ingest type id TRANSCODE_TYPE_XX
	state		int				// our state (not the state of the external process)
	Args		string			// command line arguments for transcoder
	Proc		*os.Process		// process management
	Pipe		*os.File		// IO channel to transcoder
	Pstate		*os.ProcessState	// set when transcoder finished
	LogFile		*os.File		// transcoder logfile
	Timer		*time.Timer		// session inactivity timer

	// Statistics
	BytesIn		int64			// total number of bytes received in requests
	BytesOut	int64			// total number of bytes forwarded to transcoder
	CallsIn		int64			// total number of times the ingest handler was called
	CpuUser		time.Duration	// user-space CPU time used
	CpuSystem	time.Duration	// system CPU time used
}

func NewTranscodeSession(srv *ServerState, id uint64) *TranscodeSession {
	log.Printf("Session constructor")

	s := TranscodeSession{
		Server: srv,
		UID: id,
		state: TC_INIT,
	}

	// register with server
	srv.SessionUpdate(id, TC_INIT)

	// make sure Close is properly called
	runtime.SetFinalizer(&s, (*TranscodeSession).Close)
	return &s
}

func (s *TranscodeSession) setState(state int) {
	// EOS and FAILED are final
	if s.state == TC_EOS || s.state == TC_FAILED { return }

	// set state and inform server
	s.state = state
	s.Server.SessionUpdate(s.UID, s.state)
}

func (s *TranscodeSession) IsOpen() bool {
	return s.state == TC_RUNNING
}

func (s *TranscodeSession) Open(t int) *ServerError {

	if s.IsOpen() { return nil }

	s.Type = t
	s.Args = BuildTranscodeCommand(s)
	log.Printf("Opening transcoder session: %s", s.Args)

	// create pipe
	pr, pw, err := os.Pipe()
	if err != nil {
		s.setState(TC_FAILED)
		return ServerErrorTranscodeFailed
	}
	s.Pipe = pw

	// create logfile
	idstr := strconv.FormatUint(s.UID, 10)
	logname := c.Transcode.Log_file_path + "/" + idstr + ".log"
	s.LogFile, _ = os.OpenFile(logname, os.O_WRONLY|os.O_CREATE|os.O_APPEND, 0644)

	// start transcode process
	var attr os.ProcAttr
	attr.Dir = "."
	attr.Files = []*os.File{pr, s.LogFile, s.LogFile}
	s.Proc, err = os.StartProcess(c.Transcode.Command, strings.Fields(s.Args), &attr)

	if err != nil {
		log.Printf("Error starting process: %s", err)
		s.setState(TC_FAILED)
		pr.Close()
		pw.Close()
		s.LogFile.Close()
		s.Pipe = nil
		s.Type = 0
		s.Args = ""
		return ServerErrorTranscodeFailed
	}

	// close read-end of pipe and logfile after successful start
	pr.Close()
	s.LogFile.Close()

	// set timeout for session cleanup
	s.Timer = time.AfterFunc(time.Duration(c.Server.Session_timeout) * time.Second,
	 						 func() { s.HandleTimeout() })


	// set state
	s.setState(TC_RUNNING)
	return nil
}

func (s *TranscodeSession) Close() *ServerError {

	// cancel close timeout (todo: potential race condition?)
	s.Timer.Stop()

	if !s.IsOpen() { return nil }
	log.Printf("Closing session %d", s.UID)

	// set state
	s.setState(TC_EOS)

	// close pipe
	s.Pipe.Close()

	// gracefully shut down transcode process (SIGINT, 2)
	log.Printf("Sending signal")
	var err error
	if err = s.Proc.Signal(syscall.SIGINT); err != nil {
		log.Printf("Sending signal to transcoder failed: %s", err)
		// assuming the transcoder process has finished
	}

	log.Printf("Waiting on process")
	if s.Pstate, err = s.Proc.Wait(); err != nil {
		log.Printf("Transcoder exited with error: %s and state %s", err, s.Pstate.String())
		return nil
	} else {
		log.Printf("Transcoder exit state: %s", s.Pstate.String())
	}

	log.Printf("Transcoder exited state %s", s.Pstate.String())

	// get final process statistics
	s.CpuSystem = s.Pstate.SystemTime()
	s.CpuUser = s.Pstate.UserTime()

	return nil
}

func (s *TranscodeSession) ValidateRequest(r *http.Request) *ServerError {

	// check for proper mime type
	if !IsMimeTypeValid(s.Type, r.Header.Get("Content-Type")) {
		return ServerErrorWrongMimetype
	}

	// check content

	return nil
}

func (s *TranscodeSession) Write(r *http.Request) *ServerError {

	if !s.IsOpen() { return ServerErrorTranscodeFailed }

	// reset session close timeout (race condition)
	s.Timer.Stop()

	log.Printf("Writing data to session %d", s.UID)

	// check request compatibility (mime type, content)
	if err := s.ValidateRequest(r); err != nil { return err }

	// push data into pipe until body us empty or EOF (broken pipe)
	written, err := io.Copy(s.Pipe, r.Body)
	log.Printf("Written %d bytes to session %d", written, s.UID)

	// statitsics
	s.CallsIn++
	s.BytesIn += written
	s.BytesOut += written

	// error handling
	if err == nil && written == 0 {
		// normal session close on source request (empty body)
		log.Printf("Closing session %d down for good", s.UID)
		// close the http session
		r.Close = true
		s.Close()

	} else if (err != nil) {
		// session close due to broken pipe (transcoder)
		log.Printf("Closing session %d on broken pipe.", s.UID)
		s.Close()
		return ServerErrorTranscodeFailed
	}

	r.Body.Close()

	// restart timer on success
	s.Timer = time.AfterFunc(time.Duration(c.Server.Session_timeout) * time.Second,
	 						 func() { s.HandleTimeout() })

	return nil
}

func (s *TranscodeSession) HandleTimeout() {
	log.Printf("Session timeout")
	s.Close()
}

