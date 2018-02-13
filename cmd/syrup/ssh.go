package main

import (
	"encoding/base64"
	"encoding/binary"
	"fmt"
	"net"
	"strings"
	"time"

	"github.com/spf13/viper"

	os "github.com/mkishere/sshsyrup/os"
	"github.com/mkishere/sshsyrup/os/command"
	"github.com/mkishere/sshsyrup/sftp"
	"github.com/mkishere/sshsyrup/util/termlogger"
	log "github.com/sirupsen/logrus"
	"golang.org/x/crypto/ssh"
)

// SSHSession stores SSH session info
type SSHSession struct {
	user          string
	src           net.Addr
	clientVersion string
	sshChan       <-chan ssh.NewChannel
	log           *log.Entry
	sys           *os.System
	term          string
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

type tunnelRequest struct {
	RemoteHost string
	RemotePort uint32
	LocalHost  string
	LocalPort  uint32
}

// NewSSHSession create new SSH connection based on existing socket connection
func NewSSHSession(nConn net.Conn, sshConfig *ssh.ServerConfig) (*SSHSession, error) {
	conn, chans, reqs, err := ssh.NewServerConn(nConn, sshConfig)
	if err != nil {
		return nil, err
	}
	clientIP, port, _ := net.SplitHostPort(conn.RemoteAddr().String())
	logger := log.WithFields(log.Fields{
		"user":      conn.User(),
		"srcIP":     clientIP,
		"port":      port,
		"clientStr": string(conn.ClientVersion()),
		"sessionId": base64.StdEncoding.EncodeToString(conn.SessionID()),
	})
	logger.Infof("New SSH connection with client")

	go ssh.DiscardRequests(reqs)
	return &SSHSession{
		user:          conn.User(),
		src:           conn.RemoteAddr(),
		clientVersion: string(conn.ClientVersion()),
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
	var sh *os.Shell
	go func(in <-chan *ssh.Request, channel ssh.Channel) {
		quitSignal := make(chan int, 1)
		for {
			select {
			case req := <-in:
				if req == nil {
					return
				}
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

						s.sys = os.NewSystem(s.user, viper.GetString("server.hostname"), vfs, channel, int(ptyreq.Width), int(ptyreq.Height), s.log)
						s.term = ptyreq.Term
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
					if s.sys == nil {
						s.sys = os.NewSystem(s.user, viper.GetString("server.hostname"), vfs, channel, 80, 24, s.log)
					}

					sh = os.NewShell(s.sys, s.src.String(), s.log.WithField("module", "shell"), quitSignal)

					// Create delay function if exists
					if viper.GetInt("server.delay") > 0 {
						sh.DelayFunc = createDelayFunc(viper.GetInt("server.delay"), 500)
					}
					// Create hook for session logger (For recording session to UML/asciinema)
					var hook termlogger.LogHook
					if viper.GetString("server.sessionLogFmt") == "asciinema" {
						asciiLogParams := map[string]string{
							"TERM": s.term,
							"USER": s.user,
							"SRC":  s.src.String(),
						}
						hook, err = termlogger.NewAsciinemaHook(s.sys.Width(), s.sys.Height(),
							viper.GetString("asciinema.apiEndpoint"), viper.GetString("asciinema.apiKey"), asciiLogParams,
							fmt.Sprintf("logs/sessions/%v-%v.cast", s.user, termlogger.LogTimeFormat))

					} else if viper.GetString("server.sessionLogFmt") == "uml" {
						hook, err = termlogger.NewUMLHook(0, fmt.Sprintf("logs/sessions/%v-%v.ulm.log", s.user, time.Now().Format(logTimeFormat)))
					} else {
						log.Errorf("Session Log option %v not recognized", viper.GetString("server.sessionLogFmt"))
					}
					if err != nil {
						log.Errorf("Cannot create %v log file", viper.GetString("server.sessionLogFmt"))
					}
					// The need of a goroutine here is that PuTTY will wait for reply before acknowledge it enters shell mode
					go sh.HandleRequest(hook)
					req.Reply(true, nil)
				case "subsystem":
					subsys := string(req.Payload[4:])
					s.log.WithFields(log.Fields{
						"reqType":   req.Type,
						"subSystem": subsys,
					}).Infof("User requested subsystem %v", subsys)
					if subsys == "sftp" {
						sftpSrv := sftp.NewSftp(channel, vfs,
							s.user, s.log.WithField("module", "sftp"), quitSignal)
						go sftpSrv.HandleRequest()
						req.Reply(true, nil)
					} else {
						req.Reply(false, nil)
					}
				case "window-change":
					s.log.WithField("reqType", req.Type).Info("User shell window size changed")
					if sh != nil {
						winChg := &winChgRequest{}
						if err := ssh.Unmarshal(req.Payload, winChg); err != nil {
							req.Reply(false, nil)
						}
						sh.SetSize(int(winChg.Width), int(winChg.Height))
					}
				case "exec":
					cmd := string(req.Payload[4:])
					s.log.WithFields(log.Fields{
						"reqType": req.Type,
						"cmd":     cmd,
					}).Info("User request remote exec")
					args := strings.Split(cmd, " ")
					var sys *os.System
					if s.sys == nil {
						sys = os.NewSystem(s.user, viper.GetString("server.hostname"), vfs, channel, 80, 24, s.log)
					} else {
						sys = s.sys
					}
					if strings.HasPrefix(args[0], "scp") {
						scp := command.NewSCP(channel, vfs, s.log.WithField("module", "scp"))
						go scp.Main(args[1:], quitSignal)
						req.Reply(true, nil)
						continue
					}
					n, err := sys.Exec(args[0], args[1:])
					if err != nil {
						channel.Write([]byte(fmt.Sprintf("%v: command not found\r\n", cmd)))
					}
					quitSignal <- n
					req.Reply(true, nil)
				default:
					s.log.WithField("reqType", req.Type).Infof("Unknown channel request type %v", req.Type)
				}
			case ret := <-quitSignal:
				s.log.Info("User closing channel")
				defer closeChannel(channel, ret)
				return
			}
		}
	}(requests, channel)
}

func (s *SSHSession) handleNewConn() {
	// Service the incoming Channel channel.
	for newChannel := range s.sshChan {
		s.log.WithField("chanType", newChannel.ChannelType()).Info("User created new session channel")
		switch newChannel.ChannelType() {
		case "direct-tcpip", "forwarded-tcpip":
			var treq tunnelRequest
			err := ssh.Unmarshal(newChannel.ExtraData(), &treq)
			if err != nil {
				s.log.WithError(err).Error("Cannot unmarshal port forwarding data")
				newChannel.Reject(ssh.UnknownChannelType, "Corrupt payload")
			}
			s.log.WithFields(log.Fields{
				"remoteHost": fmt.Sprintf("%v:%v", treq.RemoteHost, treq.RemotePort),
				"localHost":  fmt.Sprintf("%v:%v", treq.LocalHost, treq.LocalPort),
				"chanType":   newChannel.ChannelType(),
			}).Info("Trying to establish connection with port forwarding")
			newChannel.Reject(ssh.Prohibited, "Port forwarding disabled")
		case "session":
			go s.handleNewSession(newChannel)
		default:
			newChannel.Reject(ssh.UnknownChannelType, "unknown channel type")
			s.log.WithField("chanType", newChannel.ChannelType()).Infof("Unknown channel type %v", newChannel.ChannelType())
			continue
		}
	}
}

func createSessionHandler(c <-chan net.Conn, sshConfig *ssh.ServerConfig) {
	for conn := range c {
		sshSession, err := NewSSHSession(conn, sshConfig)
		if err != nil {
			clientIP, port, _ := net.SplitHostPort(conn.RemoteAddr().String())
			log.WithFields(log.Fields{
				"srcIP": clientIP,
				"port":  port,
			}).WithError(err).Error("Error establishing SSH connection")
		} else {
			sshSession.handleNewConn()
		}
		conn.Close()
	}
}

func closeChannel(ch ssh.Channel, signal int) {
	b := make([]byte, 4)
	binary.BigEndian.PutUint32(b, uint32(signal))
	ch.SendRequest("exit-status", false, b)
	ch.Close()
}
