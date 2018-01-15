package main

import (
	"encoding/base64"
	"fmt"
	"net"
	"strings"
	"time"

	log "github.com/sirupsen/logrus"

	"github.com/mkishere/sshsyrup/util/termlogger"
	"golang.org/x/crypto/ssh"
	"golang.org/x/crypto/ssh/terminal"
)

// SSHSession stores SSH session info
type SSHSession struct {
	user          string
	src           net.Addr
	clientVersion string
	activity      chan bool
	sshChan       <-chan ssh.NewChannel
	ptyReq        *ptyRequest
	term          *terminal.Terminal
	log           *log.Entry
}

type envRequest struct {
	Name  string
	Value string
}

type ptyRequest struct {
	Term    string
	Width   uint32
	Height  uint32
	PWidth  uint32
	PHeight uint32
	Modes   string
}
type winChgRequest struct {
	Width  uint32
	Height uint32
}

// NewSSHSession create new SSH connection based on existing socket connection
func NewSSHSession(nConn net.Conn, sshConfig *ssh.ServerConfig) (*SSHSession, error) {
	conn, chans, reqs, err := ssh.NewServerConn(nConn, sshConfig)
	if err != nil {
		return nil, err
	}

	logger := log.WithFields(log.Fields{
		"user":      conn.User(),
		"srcIP":     conn.RemoteAddr().String(),
		"clientStr": string(conn.ClientVersion()),
		"sessionId": base64.StdEncoding.EncodeToString(conn.SessionID()),
	})
	logger.Infof("New SSH connection with client")

	activity := make(chan bool)
	go func(activity chan bool) {
		defer nConn.Close()
		for range activity {
			// When receive from activity channel, reset deadline
			nConn.SetReadDeadline(time.Now().Add(config.SvrTimeout))
		}
	}(activity)

	go ssh.DiscardRequests(reqs)
	return &SSHSession{
		user:          conn.User(),
		src:           conn.RemoteAddr(),
		clientVersion: string(conn.ClientVersion()),
		activity:      activity,
		sshChan:       chans,
		log:           logger,
	}, nil
}

func (s *SSHSession) handleNewSession(newChan ssh.NewChannel) {

	channel, requests, err := newChan.Accept()
	if err != nil {
		s.log.WithError(err).Error("Could not accept channel")
		return
	}

	go func(in <-chan *ssh.Request) {
		for req := range in {

			switch req.Type {
			case "winadj@putty.projects.tartarus.org", "simple@putty.projects.tartarus.org":
				//Do nothing here
			case "pty-req":
				// Of coz we are not going to create a PTY here as we are honeypot.
				// We are creating a pseudo-PTY
				var ptyreq ptyRequest
				if err := ssh.Unmarshal(req.Payload, &ptyreq); err != nil {
					s.log.WithField("reqType", req.Type).WithError(err).Errorln("Cannot parse user request payload")
					req.Reply(false, nil)
				} else {
					s.log.WithField("reqType", req.Type).Infof("User requesting pty(%v %vx%v)", ptyreq.Term, ptyreq.Width, ptyreq.Height)
					s.ptyReq = &ptyreq
					req.Reply(true, nil)
				}
			case "env":
				var envReq envRequest
				if err := ssh.Unmarshal(req.Payload, &envReq); err != nil {
					req.Reply(false, nil)
				} else {
					s.log.WithFields(log.Fields{
						"reqType":     req.Type,
						"envVarName":  envReq.Name,
						"envVarValue": envReq.Value,
					}).Infof("User sends envvar:%v=%v", envReq.Name, envReq.Value)
					req.Reply(true, nil)
				}
			case "shell":
				s.log.WithField("reqType", req.Type).Info("User requesting shell access")
				if s.ptyReq == nil {
					s.ptyReq = &ptyRequest{
						Width:  80,
						Height: 24,
						Term:   "vt100",
					}
				}
				// The need of a goroutine here is that PuTTY will wait for reply before acknowledge it enters shell mode
				go s.NewShell(channel)
				req.Reply(true, nil)
			case "subsystem":
				subsys := string(req.Payload[4:])
				s.log.WithFields(log.Fields{
					"reqType":   req.Type,
					"subSystem": subsys,
				}).Infof("User requested subsystem %v", subsys)
				req.Reply(true, nil)
			case "window-change":
				s.log.WithField("reqType", req.Type).Info("User shell window size changed")
				if s.term == nil {
					req.Reply(false, nil)
				} else {
					var winChg *winChgRequest
					if err := ssh.Unmarshal(req.Payload, winChg); err != nil {
						req.Reply(false, nil)
					}
					s.term.SetSize(int(winChg.Width), int(winChg.Height))
					req.Reply(true, nil)
				}
			case "exec":
				cmd := string(req.Payload[4:])
				s.log.WithFields(log.Fields{
					"reqType": req.Type,
					"cmd":     cmd,
				}).Info("User request remote exec")
				channel.Write([]byte(fmt.Sprintf("%v: command not found\n", cmd)))
				req.Reply(true, nil)
				closeChannel(channel)
			default:
				s.log.WithField("reqType", req.Type).Infof("Unknown channel request type %v", req.Type)
			}
		}
	}(requests)
}

func (s *SSHSession) handleNewConn() {
	// Service the incoming Channel channel.
	for newChannel := range s.sshChan {
		// Channels have a type, depending on the application level
		// protocol intended. In the case of a shell, the type is
		// "session" and ServerShell may be used to present a simple
		// terminal interface.
		s.log.WithField("chanType", newChannel.ChannelType()).Info("User created new session channel")
		if newChannel.ChannelType() != "session" {
			newChannel.Reject(ssh.UnknownChannelType, "unknown channel type")
			s.log.WithField("chanType", newChannel.ChannelType()).Infof("Unknown channel type %v", newChannel.ChannelType())
			continue
		} else {
			go s.handleNewSession(newChannel)
		}
	}
}

// NewShell creates new shell
func (s *SSHSession) NewShell(channel ssh.Channel) {
	asciiLogParams := map[string]string{
		"TERM": s.ptyReq.Term,
		"USER": s.user,
		"SRC":  s.src.String(),
	}

	tLog := termlogger.NewACastLogger(int(s.ptyReq.Width), int(s.ptyReq.Height),
		config.AcinemaAPIEndPt, config.AcinemaAPIKey, channel, asciiLogParams)
	s.term = terminal.NewTerminal(tLog, "$ ")
	defer tLog.Close()
	defer closeChannel(channel)
cmdLoop:
	for {
		cmd, err := s.term.ReadLine()
		s.log.WithField("cmd", cmd).Infof("User input command %v", cmd)
		switch {
		case err != nil:
			if err.Error() == "EOF" {
				s.log.Info("EOF received from client")
			} else {
				s.log.WithError(err).Error("Error when reading terminal")
			}
			break cmdLoop
		case strings.TrimSpace(cmd) == "":
			//Do nothing
		case cmd == "logout", cmd == "quit":
			s.log.Infof("User logged out")
			return
		case strings.HasPrefix(cmd, "ls"):

		default:
			args := strings.SplitN(cmd, " ", 2)
			s.term.Write([]byte(fmt.Sprintf("%v: command not found\n", args[0])))
		}
	}
}

func createSessionHandler(c <-chan net.Conn, sshConfig *ssh.ServerConfig) {
	for conn := range c {
		sshSession, err := NewSSHSession(conn, sshConfig)
		if err != nil {
			log.WithField("srcIP", conn.RemoteAddr().String()).WithError(err).Error("Error establishing SSH connection")
		}
		sshSession.handleNewConn()
		conn.Close()
	}
}

func closeChannel(ch ssh.Channel) {
	ch.SendRequest("exit-status", false, []byte{0, 0, 0, 0})
	ch.Close()
}
