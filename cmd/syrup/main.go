package main

import (
	"bufio"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"math/rand"
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
	SvrAddr         string `json:"server.addr"`
	SvrPort         int    `json:"server.port"`
	SvrAllowRndUser bool   `json:"server.allowRandomUser"`
	SvrVer          string `json:"server.ident"`
	SvrMaxTries     int    `json:"server.maxTries"`
	SvrMaxConn      int    `json:"server.maxConnections"`
	SvrTimeout      int    `json:"server.timeout"`
	SvrSpeed        int    `json:"server.speed"`
	SvrDelay        int    `json:"server.processDelay"`
	SvrHostname     string `json:"server.Hostname"`
	SvrCmdList      string `json:"server.commandList"`
	SessionLogFmt   string `json:"server.sessionLogFmt"`
	VFSImgFile      string `json:"virtualfs.imageFile"`
	VFSUIDMapFile   string `json:"virtualfs.uidMappingFile"`
	VFSGIDMapFile   string `json:"virtualfs.gidMappingFile"`
	VFSReadOnly     bool   `json:"virtualfs.readOnly"`
	VFSTempDir      string `json:"virtualfs.SavedFileDir"`
	AcinemaAPIEndPt string `json:"asciinema.apiEndpoint"`
	AcinemaAPIKey   string `json:"asciinema.apiKey"`
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
		SvrVer:          "SSH-2.0-OpenSSH_6.8p1",
		SvrMaxTries:     3,
		SvrMaxConn:      10,
		SvrTimeout:      600,
		SvrSpeed:        -1,
		SvrDelay:        -1,
		SvrHostname:     "spr1139",
		SvrCmdList:      "commands.txt",
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
		log.InfoLevel: "logs/activity.log",
	}
	if _, err := os.Stat("logs"); os.IsNotExist(err) {
		os.MkdirAll("logs/sessions", 0755)
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
	// Load command list
	honeyos.RegisterFakeCommand(readFiletoArray(config.SvrCmdList))
	// Randomize seed
	rand.Seed(time.Now().Unix())
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

			if stpass, exists := honeyos.IsUserExist(c.User()); exists && (stpass == string(pass) || stpass == "*") || config.SvrAllowRndUser {
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
		tConn := NewThrottledConnection(nConn, int64(config.SvrSpeed), time.Duration(time.Second*time.Duration(config.SvrTimeout)))
		log.WithField("srcIP", tConn.RemoteAddr().String()).Info("Connection established")
		if err != nil {
			log.WithError(err).Error("Failed to accept incoming connection")
			continue
		}
		connChan <- tConn
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

func readFiletoArray(path string) []string {
	f, err := os.Open(path)
	if err != nil {
		return []string{}
	}
	defer f.Close()
	sc := bufio.NewScanner(f)
	var lines []string
	for sc.Scan() {
		lines = append(lines, sc.Text())
	}
	return lines
}
	}
	return lines
}

func createDelayFunc(base, r int) func() {
	return func() {
		sleepTime := base - r + rand.Intn(2*r)
		time.Sleep(time.Millisecond * time.Duration(sleepTime))
	}
}
