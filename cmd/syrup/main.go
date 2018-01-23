package main

import (
	"bufio"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"net"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/imdario/mergo"
	colorable "github.com/mattn/go-colorable"
	honeyos "github.com/mkishere/sshsyrup/os"
	_ "github.com/mkishere/sshsyrup/os/command"
	"github.com/mkishere/sshsyrup/virtualfs"
	"github.com/rifflock/lfshook"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/afero"
	"golang.org/x/crypto/ssh"
)

// Config type is a map for storing config values
type Config struct {
	SvrAddr         string            `json:"server.addr"`
	SvrPort         int               `json:"server.port"`
	SvrAllowRndUser bool              `json:"server.allowRandomUser"`
	SvrVer          string            `json:"server.ident"`
	SvrMaxTries     int               `json:"server.maxTries"`
	SvrMaxConn      int               `json:"server.maxConnections"`
	SvrUserList     map[string]string `json:"server.userList"`
	SvrTimeout      time.Duration     `json:"server.Timeout"`
	SessionLogFmt   string            `json:"server.sessionLogFmt"`
	VFSImgFile      string            `json:"virtualfs.imageFile"`
	VFSUIDMapFile   string            `json:"virtualfs.uidMappingFile"`
	VFSGIDMapFile   string            `json:"virtualfs.gidMappingFile"`
	VFSReadOnly     bool              `json:"virtualfs.readOnly"`
	VFSTempDir      string            `json:"virtualfs.SavedFileDir"`
	VFSWriteToImage bool              `json:"virtualfs.writeToImage"`
	AcinemaAPIEndPt string            `json:"asciinema.apiEndpoint"`
	AcinemaAPIKey   string            `json:"asciinema.apiKey"`
}

const (
	logTimeFormat string = "20060102"
)

var (
	config     = loadConfiguration("config.json")
	defaultCfg = Config{
		SvrAddr:         "0.0.0.0",
		SvrPort:         2222,
		SvrAllowRndUser: true,
		SvrVer:          "SSH-2.0-OpenSSH_6.8p1 Ubuntu-2ubuntu2.8",
		SvrMaxTries:     3,
		SvrMaxConn:      10,
		SvrUserList: map[string]string{
			"testuser": "tiger",
		},
		SvrTimeout:      time.Duration(time.Minute * 10),
		SessionLogFmt:   "asciinema",
		VFSImgFile:      "filesystem.zip",
		VFSUIDMapFile:   "passwd",
		VFSGIDMapFile:   "group",
		VFSTempDir:      "tempdir",
		AcinemaAPIEndPt: "https://asciinema.org",
	}
	vfs afero.Fs
)

func init() {
	// Merge
	mergo.Merge(&config, defaultCfg)

	log.SetFormatter(&log.TextFormatter{ForceColors: true})
	log.SetOutput(colorable.NewColorableStdout())
	pathMap := lfshook.PathMap{
		log.InfoLevel: fmt.Sprintf("logs/%v.log", time.Now().Format(logTimeFormat)),
	}

	log.AddHook(lfshook.NewHook(
		pathMap,
		&log.JSONFormatter{},
	))

	// Initalize VFS
	var err error
	// ID Mapping
	//uidMap, gidMap := loadIDMapping(config.VFSUIDMapFile), loadIDMapping(config.VFSGIDMapFile)
	backupFS := afero.NewBasePathFs(afero.NewOsFs(), config.VFSTempDir)
	zipfs, err := virtualfs.NewVirtualFS(config.VFSImgFile)
	if err != nil {
		log.Error("Cannot create virtual filesystem")
	}
	vfs = afero.NewCopyOnWriteFs(zipfs, backupFS)
	err = honeyos.LoadUsers(config.VFSUIDMapFile)
	if err != nil {
		log.Errorf("Cannot load user mapping file %v", config.VFSUIDMapFile)
	}
	err = honeyos.LoadGroups(config.VFSGIDMapFile)
	if err != nil {
		log.Errorf("Cannot load group mapping file %v", config.VFSGIDMapFile)
	}
}

func main() {

	// Read banner
	bannerFile, err := ioutil.ReadFile("banner.txt")
	if err != nil {
		bannerFile = []byte{}
	}
	sshConfig := &ssh.ServerConfig{
		AuthLogCallback: func(c ssh.ConnMetadata, method string, err error) {
			if method != "none" {
				log.WithFields(log.Fields{
					"user":       c.User(),
					"srcIP":      c.RemoteAddr().String(),
					"authMethod": method,
				}).Infof("User trying to login with %v", method)
			}
		},

		PasswordCallback: func(c ssh.ConnMetadata, pass []byte) (*ssh.Permissions, error) {
			log.WithFields(log.Fields{
				"user":     c.User(),
				"srcIP":    c.RemoteAddr().String(),
				"password": string(pass),
			}).Info("User trying to login with password")
			if stpass, exists := config.SvrUserList[c.User()]; exists && (stpass == string(pass) || stpass == "*") || config.SvrAllowRndUser {
				return &ssh.Permissions{
					Extensions: map[string]string{
						"permit-agent-forwarding": "yes",
					},
				}, nil
			}
			return nil, fmt.Errorf("password rejected for %q", c.User())
		},

		PublicKeyCallback: func(c ssh.ConnMetadata, key ssh.PublicKey) (*ssh.Permissions, error) {
			log.WithFields(log.Fields{
				"user":              c.User(),
				"srcIP":             c.RemoteAddr().String(),
				"pubKeyType":        key.Type(),
				"pubKeyFingerprint": base64.StdEncoding.EncodeToString(key.Marshal()),
			}).Info("User trying to login with key")
			return nil, errors.New("Key rejected, revert to password login")
		},

		ServerVersion: config.SvrVer,

		BannerCallback: func(c ssh.ConnMetadata) string {
			return string(bannerFile)
		},
	}

	privateBytes, err := ioutil.ReadFile("id_rsa")
	if err != nil {
		log.WithError(err).Fatal("Failed to load private key")
	}

	private, err := ssh.ParsePrivateKey(privateBytes)
	if err != nil {
		log.WithError(err).Fatal("Failed to parse private key")
	}

	sshConfig.AddHostKey(private)

	connChan := make(chan net.Conn)
	// Create pool of workers to handle connections
	for i := 0; i < config.SvrMaxConn; i++ {
		go createSessionHandler(connChan, sshConfig)
	}

	listener, err := net.Listen("tcp", fmt.Sprintf("%v:%v", config.SvrAddr, config.SvrPort))
	if err != nil {
		log.WithError(err).Fatal("Could not create listening socket")
	}
	defer listener.Close()

	for {
		nConn, err := listener.Accept()

		log.WithField("srcIP", nConn.RemoteAddr().String()).Info("Connection established")
		if err != nil {
			log.WithError(err).Error("Failed to accept incoming connection")
			continue
		}
		connChan <- nConn
	}

}

func loadConfiguration(file string) Config {
	var config Config
	configFile, err := os.Open(file)
	defer configFile.Close()
	if err != nil {
		log.WithField("file", file).WithError(err).Errorf("Cannot open configuration file")
	}

	jsonParser := json.NewDecoder(configFile)
	err = jsonParser.Decode(&config)
	if err != nil {
		log.WithField("file", file).WithError(err).Errorf("Failed to parse configuration file")
	}
	return config
}

func loadIDMapping(file string) (m map[int]string) {
	m = map[int]string{0: "root"}
	f, err := os.OpenFile(file, os.O_RDONLY, 0666)
	defer f.Close()
	if err != nil {
		return
	}
	buf := bufio.NewScanner(f)
	linenum := 1
	for buf.Scan() {
		fields := strings.Split(buf.Text(), ":")
		id, err := strconv.ParseInt(fields[2], 10, 32)
		if err != nil {
			log.Error("Cannot parse mapping file %v line %v", file, linenum)
			continue
		}
		m[int(id)] = fields[0]
		linenum++
	}
	return
}
