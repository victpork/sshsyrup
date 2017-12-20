package main

import (
	"fmt"
	"log"
	"net"
	"strings"
	"time"

	"github.com/mkishere/binutils"
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
	config        Config
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
	Modes   []uint8
}
type winChgRequest struct {
	Width  uint32
	Height uint32
}

// NewSSHSession create new SSH connection based on existing socket connection
func NewSSHSession(nConn net.Conn, sshConfig *ssh.ServerConfig, localConfig Config) (*SSHSession, error) {
	conn, chans, reqs, err := ssh.NewServerConn(nConn, sshConfig)
	if err != nil {
		return nil, err
	}

	log.Printf("New SSH connection from %s (%s)", conn.RemoteAddr(), conn.ClientVersion())

	activity := make(chan bool)
	go func(activity chan bool) {
		defer nConn.Close()
		for range activity {
			// When receive from activity channel, reset deadline
			nConn.SetReadDeadline(time.Now().Add(localConfig.SvrTimeout))
		}
	}(activity)

	go ssh.DiscardRequests(reqs)
	return &SSHSession{
		user:          conn.User(),
		src:           conn.RemoteAddr(),
		clientVersion: string(conn.ClientVersion()),
		activity:      activity,
		sshChan:       chans,
		config:        localConfig,
	}, nil
}

func (s *SSHSession) handleNewSession(newChan ssh.NewChannel) {

	channel, requests, err := newChan.Accept()
	if err != nil {
		log.Printf("Could not accept channel: %v", err)
		return
	}

	/* defer func() {
		channel.SendRequest("exit-status", false, []byte{0, 0, 0, 0})
		channel.Close()
	}() */

	go func(in <-chan *ssh.Request) {
		for req := range in {
			log.Println("Request: " + req.Type)
			switch req.Type {
			case "winadj@putty.projects.tartarus.org", "simple@putty.projects.tartarus.org":
				//Do nothing here
				req.Reply(true, nil)
			case "pty-req":
				// Of coz we are not going to create a PTY here as we are honeypot.
				// We are creating a pseudo-PTY
				var ptyreq ptyRequest
				if err := ssh.Unmarshal(req.Payload, &ptyreq); err != nil {
					req.Reply(false, nil)
				}
				log.Printf("User [%v] requesting pty(%v %vx%v)", s.user, ptyreq.Term, ptyreq.Width, ptyreq.Height)
				s.ptyReq = &ptyreq
				req.Reply(true, nil)
			case "env":
				var envReq envRequest
				if err := ssh.Unmarshal(req.Payload, &envReq); err != nil {
					req.Reply(false, nil)
				} else {
					log.Printf("User [%v] sends envvar:%v=%v", s.user, envReq.Name, envReq.Value)
					req.Reply(true, nil)
				}
			case "shell":
				log.Printf("User [%v] requesting shell access", s.user)
				if s.ptyReq == nil {
					s.ptyReq = &ptyRequest{
						Width:  80,
						Height: 24,
						Term:   "vt100",
						Modes:  []byte{},
					}
				}
				// The need of a goroutine here is that PuTTY will wait for reply before acknowledge it enters shell mode
				go s.NewShell(channel)
				req.Reply(true, nil)
			case "subsystem":
				var subsys string
				binutils.Unmarshal(req.Payload, &s)
				log.Printf("User [%v] requested subsystem %v", s.user, subsys)
				req.Reply(true, nil)
			case "window-change":
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
			default:
				log.Printf("Unknown channel request type %v", req.Type)
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
		log.Printf("User [%v] created new channel(%v)", s.user, newChannel.ChannelType())
		if newChannel.ChannelType() != "session" {
			newChannel.Reject(ssh.UnknownChannelType, "unknown channel type")
			log.Printf("Unknown channel type %v", newChannel.ChannelType())
			continue
		} else {
			go s.handleNewSession(newChannel)
		}
	}
}

// NewShell creates new shell
func (s *SSHSession) NewShell(channel ssh.Channel) {
	tLog := termlogger.NewACastLogger(int(s.ptyReq.Width), int(s.ptyReq.Height),
		s.ptyReq.Term, "", "honey", s.config.AcinemaAPIEndPt, s.config.AcinemaAPIKey, channel)
	s.term = terminal.NewTerminal(tLog, "$ ")

	defer channel.Close()
	defer tLog.Close()
cmdLoop:
	for {
		cmd, err := s.term.ReadLine()
		log.Printf("[%v] typed command %v", s.user, cmd)
		switch {
		case err != nil:
			log.Printf("Err:%v", err)
			break cmdLoop
		case strings.TrimSpace(cmd) == "":
			//Do nothing
		case cmd == "logout", cmd == "quit":
			log.Printf("User [%v] logged out", s.user)
			return
		case strings.HasPrefix(cmd, "ls"):

		default:
			args := strings.SplitN(cmd, " ", 2)
			s.term.Write([]byte(fmt.Sprintf("%v: command not found\n", args[0])))
		}
	}
}
