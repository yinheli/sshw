package sshw

import (
	"github.com/go-yaml/yaml"
	"golang.org/x/crypto/ssh"
	"io/ioutil"
	"os/user"
	"path"
)

type Node struct {
	Name       string  `json:"name"`
	Host       string  `json:"host"`
	User       string  `json:"user"`
	Port       int     `json:"port"`
	KeyPath    string  `json:"keypath"`
	Passphrase string  `json:"passphrase"`
	Password   string  `json:"password"`
	Children   []*Node `json:"children"`
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

var (
	config []*Node
)

func GetConfig() []*Node {
	return config
}

func LoadConfig() error {
	u, err := user.Current()
	if err != nil {
		return err
	}
	b, err := ioutil.ReadFile(path.Join(u.HomeDir, ".sshw"))
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
