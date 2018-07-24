package sshw

import (
	"github.com/go-yaml/yaml"
	"io/ioutil"
	"os/user"
	"path"
)

type Node struct {
	Name     string  `json:"name"`
	Host     string  `json:"host"`
	User     string  `json:"user"`
	Port     int     `json:"port"`
	KeyPath  string  `json:"keypath"`
	Password string  `json:"password"`
	Children []*Node `json:"children"`
}

func (node *Node) String() string {
	return node.Name
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
