package sshw

import (
	"fmt"
	"io/ioutil"
	"os"
	"os/user"
	"path"
	"strconv"
	"time"

	"github.com/atrox/homedir"
	"github.com/kevinburke/ssh_config"
	"golang.org/x/crypto/ssh"
	"gopkg.in/yaml.v2"
)

type Node struct {
	Name           string           `yaml:"name"`
	Alias          string           `yaml:"alias"`
	Host           string           `yaml:"host"`
	User           string           `yaml:"user"`
	Port           int              `yaml:"port"`
	KeyPath        string           `yaml:"keypath"`
	Passphrase     string           `yaml:"passphrase"`
	Password       string           `yaml:"password"`
	CallbackShells []*CallbackShell `yaml:"callback-shells"`
	Children       []*Node          `yaml:"children"`
	Jump           []*Node          `yaml:"jump"`
}

type CallbackShell struct {
	Cmd   string        `yaml:"cmd"`
	Delay time.Duration `yaml:"delay"`
}

func (n *Node) String() string {
	return n.Name
}

func (n *Node) user() string {
	if n.User == "" {
		return "root"
	}
	return n.User
}

func (n *Node) port() int {
	if n.Port <= 0 {
		return 22
	}
	return n.Port
}

func (n *Node) password() ssh.AuthMethod {
	if n.Password == "" {
		return nil
	}
	return ssh.Password(n.Password)
}

func (n *Node) alias() string {
	return n.Alias
}

var (
	config []*Node
)

func GetConfig() []*Node {
	return config
}

func LoadConfig() error {
	b, err := LoadConfigBytes(".sshw", ".sshw.yml", ".sshw.yaml")
	if err != nil {
		return err
	}
	var c []*Node
	err = yaml.Unmarshal(b, &c)
	if err != nil {
		return err
	}

	config = c

	return nil
}

func LoadSshConfig() error {
	u, err := user.Current()
	if err != nil {
		l.Error(err)
		return nil
	}
	f, _ := os.Open(path.Join(u.HomeDir, ".ssh/config"))
	cfg, _ := ssh_config.Decode(f)
	var nc []*Node
	for _, host := range cfg.Hosts {
		alias := fmt.Sprintf("%s", host.Patterns[0])
		hostName, err := cfg.Get(alias, "HostName")
		if err != nil {
			return err
		}
		if hostName != "" {
			port, _ := cfg.Get(alias, "Port")
			if port == "" {
				port = "22"
			}
			var c = new(Node)
			c.Name = alias
			c.Alias = alias
			c.Host = hostName
			c.User, _ = cfg.Get(alias, "User")
			c.Port, _ = strconv.Atoi(port)
			keyPath, _ := cfg.Get(alias, "IdentityFile")
			c.KeyPath, _ = homedir.Expand(keyPath)
			nc = append(nc, c)
			// fmt.Println(c.Alias, c.Host, c.User, c.Port, c.KeyPath)
		}
	}
	config = nc
	return nil
}

func LoadConfigBytes(names ...string) ([]byte, error) {
	u, err := user.Current()
	if err != nil {
		return nil, err
	}
	// homedir
	for i := range names {
		sshw, err := ioutil.ReadFile(path.Join(u.HomeDir, names[i]))
		if err == nil {
			return sshw, nil
		}
	}
	// relative
	for i := range names {
		sshw, err := ioutil.ReadFile(names[i])
		if err == nil {
			return sshw, nil
		}
	}
	return nil, err
}
